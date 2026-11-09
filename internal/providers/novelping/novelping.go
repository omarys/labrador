// Package novelping implements a text (light novel) provider for novelping.com.
//
// Unlike the manga providers, chapters here are prose: chapters are enumerated
// through the site's JSON chapter-option endpoint and the body is scraped from
// div#chr-content and converted to Markdown, one paragraph per <p>. The
// provider declares domain.MediaText so the downloader writes .md chapters
// instead of .cbz.
//
// Note: novelping.com/robots.txt disallows /book/ for crawlers. This provider
// is intended for local, personal use.
package novelping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/htmlmd"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "novelping"
	Name           = "NovelPing"
	DefaultBaseURL = "https://novelping.com"
)

// ErrTextOnly is returned by GetPages, which does not apply to prose chapters.
var ErrTextOnly = errors.New("novelping: chapters are text, use GetChapterText")

// reVolumePrefix matches a leading volume marker such as "Volume 1" or
// "volume 12". NovelPing chapter names are volume-scoped ("Volume 1  2 — …"),
// so the first number in the name is the volume, not the chapter.
var reVolumePrefix = regexp.MustCompile(`(?i)^\s*volume\s+([0-9]+)\b`)

// volumeChapterScale spaces chapters inside a volume so the composite number
// stays distinct and ordered: 1.0, 1.001, 1.002, 2.0, …
const volumeChapterScale = 1000

type Provider struct {
	baseURL string
	client  *http.Client
}

func New(client *http.Client) *Provider {
	return NewWithBaseURL(DefaultBaseURL, client)
}

func NewWithBaseURL(baseURL string, client *http.Client) *Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

func (p *Provider) ID() string   { return ID }
func (p *Provider) Name() string { return Name }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		CanSearch:      true,
		CanBrowse:      true,
		SupportedSorts: []domain.SortOrder{domain.SortPopular, domain.SortRecent},
		HasTags:        true,
		Media:          domain.MediaText,
	}
}

func (p *Provider) MatchesURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Host), "novelping.com")
}

// Search queries the site's search page (page 1).
func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	searchURL := fmt.Sprintf("%s/search?keyword=%s", p.baseURL, url.QueryEscape(query))
	return p.fetchListing(ctx, searchURL)
}

// Browse lists series from the sort feeds, or from a genre when a tag is given.
// Genre pages accept their own ?sort= vocabulary, so a tag and a sort combine
// into one request rather than one overriding the other.
func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	var browseURL string
	switch {
	case opts.Tag != nil && opts.Tag.ID != "":
		browseURL = fmt.Sprintf("%s/novelping-genres/%s?page=%d&sort=%s",
			p.baseURL, url.PathEscape(opts.Tag.ID), page, genreSort(opts.Sort))
	case opts.Sort == domain.SortRecent:
		browseURL = fmt.Sprintf("%s/sort/updates?page=%d", p.baseURL, page)
	default:
		browseURL = fmt.Sprintf("%s/sort/popular?page=%d", p.baseURL, page)
	}

	return p.fetchListing(ctx, browseURL)
}

// genreSort maps a sort order onto the ?sort= values genre pages accept.
// Anything unrecognised — including an empty sort — falls back to ALL_TIME,
// which is the order the site serves by default.
func genreSort(sort domain.SortOrder) string {
	if sort == domain.SortRecent {
		return "LASTEST" // sic: the site's own spelling
	}
	return "ALL_TIME"
}

// GetTags collects the genre links rendered on the home page.
func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	doc, err := p.fetchDoc(ctx, p.baseURL+"/")
	if err != nil {
		return nil, err
	}

	var tags []domain.Tag
	seen := make(map[string]bool)
	doc.Find("a[href*='novelping-genres/']").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		id := genreID(href)
		if id == "" || seen[id] {
			return
		}
		label := strings.TrimSpace(s.Text())
		if label == "" {
			label = strings.TrimSpace(s.AttrOr("title", ""))
		}
		if label == "" {
			return
		}
		seen[id] = true
		tags = append(tags, domain.Tag{ID: id, Label: label})
	})

	return tags, nil
}

// GetChapters enumerates every chapter through the site's JSON chapter-option
// endpoint. The series HTML only embeds the ~30 most recent chapters, so the
// endpoint is the only complete source.
func (p *Provider) GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error) {
	slug := bookSlug(series.URL)
	if slug == "" {
		slug = strings.TrimPrefix(series.ID, "/")
	}
	if slug == "" {
		return nil, errors.New("novelping: cannot determine novel slug")
	}

	reqURL := fmt.Sprintf("%s/ajax/chapter-option?novelId=%s&format=json", p.baseURL, url.QueryEscape(slug))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload chapterOptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding chapter list: %w", err)
	}
	if !payload.Success {
		return nil, fmt.Errorf("chapter list unavailable for novel %s", slug)
	}

	chapters := make([]domain.Chapter, 0, len(payload.Chapters))
	volume, seqInVolume := 0, 0
	for _, c := range payload.Chapters {
		if c.ChapterID == "" {
			continue
		}
		chapterURL := c.URL
		if chapterURL == "" {
			chapterURL = fmt.Sprintf("%s/book/%s/%s", p.baseURL, slug, c.ChapterID)
		}
		chapters = append(chapters, domain.Chapter{
			ID:            c.ChapterID,
			SeriesID:      series.ID,
			Title:         c.ChapterName,
			URL:           chapterURL,
			OriginalLabel: c.ChapterName,
			Number:        chapterNumber(c.ChapterName, &volume, &seqInVolume),
			Index:         len(chapters),
		})
	}

	return chapters, nil
}

// chapterNumber derives a number that is unique per series and ordered by
// position in the payload. Volume-scoped names are numbered by volume and
// position within it (1.0, 1.001, …) because the generic parser returns the
// leading volume number for every chapter of a volume, collapsing them onto one
// key. Names without a volume marker keep the generic parser's result, which
// may be nil.
//
// volume and seqInVolume carry state across one GetChapters loop: the sequence
// resets whenever the volume number changes, and payload order is reading order.
func chapterNumber(name string, volume, seqInVolume *int) *float64 {
	v, ok := volumePrefix(name)
	if !ok {
		return domain.ParseChapterNumber(name)
	}
	if v != *volume {
		*volume, *seqInVolume = v, 0
	} else {
		(*seqInVolume)++
	}
	n := math.Round((float64(*volume)+float64(*seqInVolume)/volumeChapterScale)*volumeChapterScale) / volumeChapterScale
	return &n
}

// volumePrefix reports the volume number of a leading "Volume <N>" marker.
func volumePrefix(name string) (int, bool) {
	m := reVolumePrefix.FindStringSubmatch(name)
	if len(m) < 2 {
		return 0, false
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return v, true
}

// GetPages does not apply to prose chapters.
func (p *Provider) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	return nil, ErrTextOnly
}

// GetChapterText scrapes the chapter body and returns it as a Markdown
// fragment: one paragraph per <p>, inline markup converted, literal prose
// escaped. The chapter heading is not included — the archive writer adds it.
func (p *Provider) GetChapterText(ctx context.Context, chapter domain.Chapter) (string, error) {
	doc, err := p.fetchDoc(ctx, chapter.URL)
	if err != nil {
		return "", err
	}

	container := doc.Find("div#chr-content").First()
	if container.Length() == 0 {
		return "", fmt.Errorf("novelping: chapter content not found at %s", chapter.URL)
	}

	body := strings.TrimSpace(htmlmd.Fragment(container))
	if body == "" {
		return "", fmt.Errorf("novelping: chapter body is empty at %s", chapter.URL)
	}
	return body, nil
}

// fetchListing scrapes novel cards from a listing, genre, tag or search page.
func (p *Provider) fetchListing(ctx context.Context, reqURL string) ([]domain.Series, error) {
	doc, err := p.fetchDoc(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var results []domain.Series
	seen := make(map[string]bool)
	doc.Find("h3.novel-title a[href*='/book/'], div.index-novel div.item a[href*='/book/']").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}

		seriesURL := p.resolveURL(href)
		id := bookSlug(seriesURL)
		if id == "" || seen[id] {
			return
		}

		title := strings.TrimSpace(s.AttrOr("title", ""))
		if title == "" {
			title = strings.TrimSpace(s.Find("h3").Text())
		}
		if title == "" {
			title = strings.TrimSpace(s.Text())
		}
		if title == "" {
			return
		}

		seen[id] = true
		results = append(results, domain.Series{ID: id, Title: title, URL: seriesURL})
	})

	return results, nil
}

func (p *Provider) fetchDoc(ctx context.Context, reqURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}
	return doc, nil
}

func (p *Provider) resolveURL(href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}
	return p.baseURL + href
}

// bookSlug extracts the novel slug from a /book/<slug> URL.
func bookSlug(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if part == "book" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// genreID extracts the genre slug from a /novelping-genres/<genre> link.
func genreID(href string) string {
	path := href
	if u, err := url.Parse(href); err == nil {
		path = u.Path
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "novelping-genres" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

type chapterOption struct {
	ChapterID   string `json:"chapter_id"`
	ChapterName string `json:"chapter_name"`
	URL         string `json:"url"`
	Premium     bool   `json:"premium"`
	Platinum    bool   `json:"platinum"`
}

type chapterOptionResponse struct {
	Success  bool            `json:"success"`
	NovelID  string          `json:"novel_id"`
	Chapters []chapterOption `json:"chapters"`
}
