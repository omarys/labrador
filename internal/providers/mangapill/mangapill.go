package mangapill

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
	ID             = "mangapill"
	Name           = "MangaPill"
	DefaultBaseURL = "https://mangapill.com"
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

func (p *Provider) ID() string {
	return ID
}

func (p *Provider) Name() string {
	return Name
}

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
	return strings.Contains(strings.ToLower(u.Host), "mangapill.com")
}

// Search finds series matching the query string
func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(query))
	return p.fetchSeriesList(ctx, searchURL)
}

// fetchSeriesList is an internal helper that scrapes series cards from a page.
func (p *Provider) fetchSeriesList(ctx context.Context, reqURL string) ([]domain.Series, error) {
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
	doc.Find("div.grid div.border-border, div.container div.grid > div, div.my-3.grid > div").Each(func(_ int, s *goquery.Selection) {
		a := s.Find("a[href^='/manga/'], a").First()
		relHref, exists := a.Attr("href")
		if !exists || !strings.HasPrefix(relHref, "/manga/") {
			return
		}

		title := strings.TrimSpace(s.Find(".font-bold").Text())
		if title == "" {
			title = strings.TrimSpace(a.Text())
		}
		if title == "" {
			return
		}

		seriesURL := p.resolveURL(relHref)
		seriesID := strings.TrimPrefix(relHref, "/manga/")

		results = append(results, domain.Series{
			ID:    seriesID,
			Title: title,
			URL:   seriesURL,
		})
	})
	return results, nil
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

// GetTags extracts available genres from the genre index.
func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/mangas", nil)
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
	doc.Find("div[data-menu='genres'] a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		id := strings.TrimPrefix(href, "/genres/")
		label := strings.TrimSpace(s.Text())
		if id != "" && label != "" {
			tags = append(tags, domain.Tag{
				ID:    id,
				Label: label,
			})
		}
	})

	return tags, nil
}

// GetPages extracts the image URLs for a single chapter.
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
	doc.Find("picture img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("data-src")
		if !exists || src == "" {
			src, exists = s.Attr("src")
		}
		if !exists || src == "" {
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

// Browse lists series with optional genre filtering and sorting.
func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	var browseURL string
	if opts.Tag != nil && opts.Tag.ID != "" {
		browseURL = fmt.Sprintf("%s/genres/%s?page=%d", p.baseURL, opts.Tag.ID, page)
	} else if opts.Sort == domain.SortRecent {
		browseURL = fmt.Sprintf("%s/chapters?page=%d", p.baseURL, page)
	} else {
		browseURL = fmt.Sprintf("%s/search?q=&page=%d", p.baseURL, page)
	}

	return p.fetchSeriesList(ctx, browseURL)
}

// GetChapters extracts all chapter links from a series page.
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
	doc.Find("#chapters a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(s.Text())
		chapterURL := p.resolveURL(href)
		chapterID := strings.TrimPrefix(href, "/ck/")

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
	return chapters, nil
}
