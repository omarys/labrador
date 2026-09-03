package mangakatana

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "mangakatana"
	Name           = "MangaKatana"
	DefaultBaseURL = "https://mangakatana.com"
)

var (
	thzqRE   = regexp.MustCompile(`var thzq\s*=\s*\[([^\]]*)\]`)
	imgURLRE = regexp.MustCompile(`'([^']+)'`)
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
	return strings.Contains(strings.ToLower(u.Host), "mangakatana.com")
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	reqURL := fmt.Sprintf("%s/?search=%s&search_by=keyword", p.baseURL, url.QueryEscape(query))
	return p.fetchSeries(ctx, reqURL)
}

func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}

	var reqURL string
	if opts.Tag != nil && opts.Tag.ID != "" {
		reqURL = fmt.Sprintf("%s/genre/%s/page/%d", p.baseURL, opts.Tag.ID, page)
	} else if opts.Sort == domain.SortRecent {
		reqURL = fmt.Sprintf("%s/latest/page/%d", p.baseURL, page)
	} else {
		reqURL = fmt.Sprintf("%s/manga/page/%d", p.baseURL, page)
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
	doc.Find("#book_list .item").Each(func(_ int, s *goquery.Selection) {
		link := s.Find(".title a").First()
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}

		seriesURL := p.resolveURL(href)
		seriesID := extractKatanaID(href)

		results = append(results, domain.Series{
			ID:    seriesID,
			Title: title,
			URL:   seriesURL,
		})
	})

	return results, nil
}

func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/genres", nil)
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
	doc.Find(".genres_wrap a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		id := strings.TrimPrefix(href, "/genre/")
		id = strings.Trim(id, "/")
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
	doc.Find(".chapters table tr").Each(func(_ int, s *goquery.Selection) {
		link := s.Find(".chapter a").First()
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(link.Text())
		chapterURL := p.resolveURL(href)
		parts := strings.Split(strings.TrimRight(chapterURL, "/"), "/")
		chapterID := parts[len(parts)-1]

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

	// MangaKatana lists chapters latest first; reverse so reading order is ascending
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

	htmlContent, _ := doc.Html()
	imageURLs := extractKatanaImages(htmlContent)
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("no page images found in chapter source")
	}

	var pages []domain.Page
	for i, imgURL := range imageURLs {
		pages = append(pages, domain.Page{
			URL:   imgURL,
			Index: i,
		})
	}

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

func extractKatanaID(mangaURL string) string {
	parts := strings.Split(strings.TrimRight(mangaURL, "/"), "/")
	last := parts[len(parts)-1]
	if idx := strings.LastIndex(last, "."); idx >= 0 {
		return last[idx+1:]
	}
	return last
}

func extractKatanaImages(html string) []string {
	match := thzqRE.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil
	}

	matches := imgURLRE.FindAllStringSubmatch(match[1], -1)
	var urls []string
	for _, m := range matches {
		if len(m) >= 2 {
			urls = append(urls, m[1])
		}
	}
	return urls
}
