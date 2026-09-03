package weebcentral

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "weebcentral"
	Name           = "Weeb Central"
	DefaultBaseURL = "https://weebcentral.com"
)

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

func (p *Provider) ID() string { return ID }

func (p *Provider) Name() string { return Name }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		CanSearch:      true,
		CanBrowse:      true,
		SupportedSorts: []domain.SortOrder{domain.SortPopular, domain.SortRecent},
		HasTags:        false,
	}
}

func (p *Provider) MatchesURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Host), "weebcentral.com")
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	reqURL := fmt.Sprintf("%s/search/data?text=%s&display_mode=Full+Display", p.baseURL, url.QueryEscape(query))
	return p.fetchSeries(ctx, reqURL)
}

func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	sortParam := "Popularity"
	if opts.Sort == domain.SortRecent {
		sortParam = "Latest+Updates"
	}
	reqURL := fmt.Sprintf("%s/search/data?sort=%s&display_mode=Full+Display", p.baseURL, sortParam)
	return p.fetchSeries(ctx, reqURL)
}

func (p *Provider) fetchSeries(ctx context.Context, reqURL string) ([]domain.Series, error) {
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

	var results []domain.Series
	doc.Find("article.bg-base-300, article").Each(func(_ int, s *goquery.Selection) {
		link := s.Find("a.line-clamp-1, h2 a, a").First()
		href, exists := link.Attr("href")
		if !exists || !strings.Contains(href, "/series/") {
			return
		}
		title := cleanSeriesTitle(link)
		if title == "" {
			return
		}

		seriesURL := p.resolveURL(href)
		parts := strings.Split(strings.TrimRight(seriesURL, "/"), "/")
		seriesID := parts[len(parts)-1]

		results = append(results, domain.Series{
			ID:    seriesID,
			Title: title,
			URL:   seriesURL,
		})
	})

	return results, nil
}

func cleanSeriesTitle(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find("span, .badge, [class*='badge'], svg, i").Remove()
	text := clone.Text()
	if strings.TrimSpace(text) == "" {
		text = s.Text()
	}

	fields := strings.Fields(text)
	var filtered []string
	for _, f := range fields {
		if len(filtered) == 0 && (strings.EqualFold(f, "official") || strings.EqualFold(f, "scanlation")) {
			continue
		}
		filtered = append(filtered, f)
	}

	if len(filtered) > 0 {
		return strings.Join(filtered, " ")
	}
	return strings.Join(fields, " ")
}

func (p *Provider) GetTags(_ context.Context) ([]domain.Tag, error) {
	return nil, nil
}

func (p *Provider) GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error) {
	// WeebCentral series pages link to chapters directly or via /full-chapter-list
	reqURL := series.URL
	if !strings.HasSuffix(reqURL, "/full-chapter-list") {
		reqURL = strings.TrimRight(reqURL, "/") + "/full-chapter-list"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		// Fallback to series URL directly if full-chapter-list endpoint 404s
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, series.URL, nil)
		resp, err = p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	var chapters []domain.Chapter
	seen := make(map[string]bool)
	index := 0

	doc.Find("a[href*='/chapters/']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		chapterURL := p.resolveURL(href)
		if seen[chapterURL] {
			return
		}
		seen[chapterURL] = true

		title := strings.Join(strings.Fields(s.Text()), " ")
		if title == "" {
			title = fmt.Sprintf("Chapter %d", index+1)
		}
		chapterID := filepath.Base(chapterURL)

		chapters = append(chapters, domain.Chapter{
			ID:            chapterID,
			SeriesID:      series.ID,
			Title:         title,
			URL:           chapterURL,
			OriginalLabel: title,
			Index:         index,
		})
		index++
	})

	// Reverse to ascending reading order
	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
		chapters[i].Index = i
		chapters[j].Index = j
	}

	return chapters, nil
}

func (p *Provider) GetPages(ctx context.Context, chapter domain.Chapter) ([]domain.Page, error) {
	reqURL := chapter.URL
	if !strings.HasSuffix(reqURL, "/images") {
		reqURL = strings.TrimRight(reqURL, "/") + "/images?is_reading_mode=true"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		// Fallback to chapter URL directly
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, chapter.URL, nil)
		resp, err = p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	var pages []domain.Page
	index := 0
	doc.Find("section img, img.cursor-pointer, img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			src, exists = s.Attr("data-src")
		}
		if !exists || src == "" || !strings.Contains(src, "http") {
			return
		}

		pages = append(pages, domain.Page{
			URL:   src,
			Index: index,
		})
		index++
	})

	return pages, nil
}

func (p *Provider) resolveURL(relPath string) string {
	if strings.HasPrefix(relPath, "http://") || strings.HasPrefix(relPath, "https://") {
		return relPath
	}
	if !strings.HasPrefix(relPath, "/") {
		relPath = "/" + relPath
	}
	return p.baseURL + relPath
}
