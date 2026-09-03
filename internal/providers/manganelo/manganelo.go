package manganelo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "manganelo"
	Name           = "Manganelo"
	DefaultBaseURL = "https://manganelo.com"
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
		HasTags:        true,
	}
}

func (p *Provider) MatchesURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "manganelo.com") || strings.Contains(host, "chapmanganelo.com")
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	formattedQuery := strings.ReplaceAll(strings.TrimSpace(query), " ", "_")
	reqURL := fmt.Sprintf("%s/search/story/%s", p.baseURL, url.PathEscape(formattedQuery))
	return p.fetchSeries(ctx, reqURL)
}

func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	sortParam := "topview"
	if opts.Sort == domain.SortRecent {
		sortParam = "latest"
	}

	var reqURL string
	if opts.Tag != nil && opts.Tag.ID != "" {
		reqURL = fmt.Sprintf("%s/genre-%s/%d?type=%s", p.baseURL, opts.Tag.ID, page, sortParam)
	} else {
		reqURL = fmt.Sprintf("%s/genre-all/%d?type=%s", p.baseURL, page, sortParam)
	}

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
	doc.Find("div.search-story-item, div.content-genres-item").Each(func(_ int, s *goquery.Selection) {
		link := s.Find("a.item-title").First()
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}

		parts := strings.Split(strings.TrimRight(href, "/"), "/")
		seriesID := parts[len(parts)-1]

		results = append(results, domain.Series{
			ID:    seriesID,
			Title: title,
			URL:   p.resolveURL(href),
		})
	})

	return results, nil
}

func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/genre-all", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	var tags []domain.Tag
	doc.Find("div.panel-category table a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		id := strings.TrimPrefix(href, "/genre-")
		id = strings.Split(id, "/")[0]
		label := strings.TrimSpace(s.Text())
		if id != "" && label != "" && id != "all" {
			tags = append(tags, domain.Tag{
				ID:    id,
				Label: label,
			})
		}
	})

	return tags, nil
}

func (p *Provider) GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, series.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	var chapters []domain.Chapter
	index := 0
	doc.Find("ul.row-content-chapter li.a-h, div.panel-story-chapter-list li.a-h").Each(func(_ int, s *goquery.Selection) {
		link := s.Find("a.chapter-name, a").First()
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(link.Text())
		parts := strings.Split(strings.TrimRight(href, "/"), "/")
		chapterID := parts[len(parts)-1]

		chapters = append(chapters, domain.Chapter{
			ID:            chapterID,
			SeriesID:      series.ID,
			Title:         title,
			URL:           p.resolveURL(href),
			OriginalLabel: title,
			Index:         index,
		})
		index++
	})

	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
		chapters[i].Index = i
		chapters[j].Index = j
	}

	return chapters, nil
}

func (p *Provider) GetPages(ctx context.Context, chapter domain.Chapter) ([]domain.Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, chapter.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	var pages []domain.Page
	index := 0
	doc.Find("div.container-chapter-reader img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			src, exists = s.Attr("data-src")
		}
		if !exists || src == "" {
			return
		}

		pages = append(pages, domain.Page{
			URL:   src,
			Index: index,
			Headers: map[string][]string{
				"Referer": {p.baseURL},
			},
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
