// Package novelarchive implements a text (light novel) provider for
// novelarchive.st.
//
// novelarchive.st is a client-rendered single-page app: every HTML route serves
// the same shell, and all catalogue and chapter data comes from a JSON API. The
// provider therefore talks to the API directly and parses no HTML:
//
//	GET /api/novels?page=&limit=&search=&sort=&genres_include=&genre_match=
//	GET /api/novels/<novel-id>
//	GET /api/novels/<novel-id>/chapters/<ordinal>
//
// Chapter identity: the API has no chapter ids. A chapter is addressed by its
// 1-based ordinal position in the series' chapter_names array, which ships
// whole with the series payload in reading order, so no pagination is needed.
// Requesting an ordinal past the end of the series answers "Chapter does not
// exist", and the ordinal is not portable across series.
//
// The API body is plain text, either with paragraphs separated by a blank line
// or with one paragraph per line and no blank lines at all, so GetChapterText
// normalizes it to one paragraph per line and the provider declares
// domain.MediaText: the downloader writes .md chapters, not .cbz.
//
// Note: novelarchive.st/robots.txt allows every agent ("User-agent: *",
// "Allow: /") but disallows AI-training crawlers and reserves text-and-data
// mining rights, so this provider is intended for local, personal use. The API
// sits behind Cloudflare, which answers 403 to requests without an
// Accept-Language header, so every request sets one explicitly.
package novelarchive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "novelarchive"
	Name           = "NovelArchive"
	DefaultBaseURL = "https://novelarchive.st"

	// pageSize is the catalogue page size requested when the caller does not
	// pick one.
	pageSize = 50

	// maxAttempts is how many times one API request is tried. The origin
	// intermittently stalls — Cloudflare answers 504 when the backend takes too
	// long, and the very next request usually succeeds.
	maxAttempts = 2
)

// retryDelay is the pause between attempts. It is a variable so tests can
// collapse it.
var retryDelay = 2 * time.Second

// ErrTextOnly is returned by GetPages, which does not apply to prose chapters.
var ErrTextOnly = errors.New("novelarchive: chapters are text, use GetChapterText")

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
		CanSearch: true,
		CanBrowse: true,
		// The API also accepts "recent", "popular" and "rating"; an unknown
		// value silently degrades to "recent" server-side.
		SupportedSorts: []domain.SortOrder{domain.SortRecent, domain.SortPopular, domain.SortRating},
		HasTags:        true,
		Media:          domain.MediaText,
	}
}

func (p *Provider) MatchesURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Host), "novelarchive.st")
}

// Search queries the catalogue with the server's own search, which falls back
// to fuzzy matching when there is no exact hit.
func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	params := url.Values{}
	params.Set("search", query)
	return p.list(ctx, params)
}

// Browse lists series from the catalogue, optionally narrowed to one genre.
// Genres are matched by their filter value, which is not the display label.
func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("sort", sortParam(opts.Sort))

	if opts.Tag != nil {
		genre := strings.TrimSpace(opts.Tag.ID)
		if genre == "" {
			genre = strings.TrimSpace(opts.Tag.Label)
		}
		if genre != "" {
			params.Set("genres_include", genre)
			params.Set("genre_match", "any")
		}
	}

	return p.list(ctx, params)
}

// sortParam maps a sort order onto the ?sort= values the API accepts.
func sortParam(sort domain.SortOrder) string {
	switch sort {
	case domain.SortPopular:
		return "popular"
	case domain.SortRating:
		return "rating"
	default:
		// Includes domain.SortRecent and domain.SortAlphabetical, which the
		// catalogue does not offer: "recent" is the site's own default.
		return "recent"
	}
}

// GetTags returns the site's whole genre vocabulary from /api/novels/genres.
// The endpoint is authoritative and cheap — one request for 659 entries — so the
// list is complete rather than sampled, and a caller may replace its stored
// vocabulary with it.
//
// Tag.ID is the lower-case filter value and Tag.Label the display name. The two
// disagree in ways that matter: the API filters case-sensitively, and
// "Completed" matches nothing while "completed" matches two novels.
func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	var payload genresResponse
	if err := p.getJSON(ctx, "/api/novels/genres", &payload); err != nil {
		return nil, fmt.Errorf("listing genres: %w", err)
	}

	seen := make(map[string]struct{}, len(payload.Genres))
	tags := make([]domain.Tag, 0, len(payload.Genres))
	for _, g := range payload.Genres {
		id := strings.ToLower(strings.TrimSpace(g.Value))
		label := strings.TrimSpace(g.Label)
		if id == "" || label == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		tags = append(tags, domain.Tag{ID: id, Label: label})
	}

	sort.Slice(tags, func(i, j int) bool { return tags[i].Label < tags[j].Label })
	return tags, nil
}

// GetChapters reads the series payload and turns its chapter_names into
// chapters, numbered by the name and addressed by position.
func (p *Provider) GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error) {
	novelID := p.novelID(series)
	if novelID == "" {
		return nil, errors.New("novelarchive: cannot determine novel id")
	}

	var payload novelResponse
	if err := p.getJSON(ctx, "/api/novels/"+url.PathEscape(novelID), &payload); err != nil {
		return nil, err
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("novelarchive: novel %s: %s", novelID, payload.Error)
	}

	names := payload.Novel.ChapterNames
	chapters := make([]domain.Chapter, 0, len(names))
	for i, name := range names {
		ordinal := strconv.Itoa(i + 1)
		chapters = append(chapters, domain.Chapter{
			ID:            ordinal,
			SeriesID:      novelID,
			Title:         strings.TrimSpace(name),
			URL:           fmt.Sprintf("%s/reader?novel=%s&chapter=%s", p.baseURL, url.QueryEscape(novelID), ordinal),
			OriginalLabel: name,
			Number:        domain.ParseChapterNumber(name),
			Index:         i,
		})
	}

	return chapters, nil
}

// GetPages does not apply to prose chapters.
func (p *Provider) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	return nil, ErrTextOnly
}

// GetChapterText fetches the chapter body and returns it as normalized
// Markdown paragraphs. The chapter heading is not included — the archive writer
// adds it.
func (p *Provider) GetChapterText(ctx context.Context, chapter domain.Chapter) (string, error) {
	novelID, ordinal := p.chapterLocator(chapter)
	if novelID == "" || ordinal == "" {
		return "", fmt.Errorf("novelarchive: cannot determine chapter location for %s", chapter.URL)
	}

	path := fmt.Sprintf("/api/novels/%s/chapters/%s", url.PathEscape(novelID), url.PathEscape(ordinal))
	var payload chapterResponse
	if err := p.getJSON(ctx, path, &payload); err != nil {
		return "", err
	}
	if payload.Error != "" {
		return "", fmt.Errorf("novelarchive: chapter %s of novel %s: %s", ordinal, novelID, payload.Error)
	}

	body := normalizeProse(payload.Chapter.Content)
	if body == "" {
		return "", fmt.Errorf("novelarchive: chapter body is empty at %s", chapter.URL)
	}
	return body, nil
}

// normalizeProse turns an API body into Markdown paragraphs. Chapters arrive in
// one of two shapes — paragraphs separated by a blank line, or one paragraph per
// line with no blank lines anywhere — so every non-empty line becomes its own
// paragraph. Read as a single line, these chapters collapse into one block.
func normalizeProse(content string) string {
	paragraphs := make([]string, 0, strings.Count(content, "\n")+1)
	for _, line := range strings.Split(content, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paragraphs = append(paragraphs, line)
		}
	}
	return strings.Join(paragraphs, "\n\n")
}

// list reads one catalogue page and maps its novels onto series.
func (p *Provider) list(ctx context.Context, params url.Values) ([]domain.Series, error) {
	if params.Get("limit") == "" {
		params.Set("limit", strconv.Itoa(pageSize))
	}

	var payload novelsResponse
	if err := p.getJSON(ctx, "/api/novels?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	series := make([]domain.Series, 0, len(payload.Novels))
	for _, n := range payload.Novels {
		if n.ID == "" {
			continue
		}
		series = append(series, domain.Series{
			ID:    n.ID,
			Title: strings.TrimSpace(n.Title),
			URL:   p.seriesURL(n.ID),
		})
	}

	return series, nil
}

// getJSON issues one API request and decodes the response into dst, retrying a
// transient failure once.
func (p *Provider) getJSON(ctx context.Context, path string, dst any) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = p.doJSON(ctx, path, dst); err == nil || !isRetryable(err) {
			return err
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(retryDelay):
		}
	}
	return err
}

// doJSON performs a single request/response round trip.
func (p *Provider) doJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	// Cloudflare rejects the API with 403 when Accept-Language is absent.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("executing request: %w", err)
		}
		return retryableError{fmt.Errorf("executing request: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status: %s", resp.Status)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			return retryableError{err}
		}
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// retryableError marks a failure worth repeating: a transport error or a
// throttled/failed status.
type retryableError struct{ err error }

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	var r retryableError
	return errors.As(err, &r)
}

// seriesURL builds the canonical series page URL for a novel id.
func (p *Provider) seriesURL(novelID string) string {
	return p.baseURL + "/novel?id=" + url.QueryEscape(novelID)
}

// novelID prefers the id carried by the series and falls back to the URL. A
// series page carries ?id=, a reader page carries ?novel=, and either is pasted
// in place of a series URL often enough to matter.
func (p *Provider) novelID(series domain.Series) string {
	if series.ID != "" {
		return series.ID
	}
	u, err := url.Parse(series.URL)
	if err != nil {
		return ""
	}
	q := u.Query()
	return firstNonEmpty(q.Get("id"), q.Get("novel"))
}

// chapterLocator recovers the novel id and 1-based ordinal a chapter is
// addressed by, preferring the reader URL the chapter carries.
func (p *Provider) chapterLocator(chapter domain.Chapter) (string, string) {
	ordinal := chapter.ID
	if u, err := url.Parse(chapter.URL); err == nil {
		q := u.Query()
		if id := q.Get("novel"); id != "" {
			return id, firstNonEmpty(q.Get("chapter"), ordinal)
		}
	}
	return chapter.SeriesID, ordinal
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

type novelItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Genres       string   `json:"genres"`
	ChapterNames []string `json:"chapter_names"`
}

type chapterItem struct {
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type novelsResponse struct {
	Novels []novelItem `json:"novels"`
	Error  string      `json:"error"`
}

// genresResponse is /api/novels/genres: value is the filter key, label the
// display name.
type genresResponse struct {
	Genres []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"genres"`
	Error string `json:"error"`
}

type novelResponse struct {
	Novel novelItem `json:"novel"`
	Error string    `json:"error"`
}

type chapterResponse struct {
	Chapter chapterItem `json:"chapter"`
	Error   string      `json:"error"`
}
