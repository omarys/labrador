package readcomicsonline

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "readcomicsonline"
	Name           = "ReadComicsOnline"
	DefaultBaseURL = "https://readcomicsonline.lol"
)

var (
	webpPageRE   = regexp.MustCompile(`https://[^"\s\\]*/pages/[^"\s\\]+?\.webp`)
	pageNumRE    = regexp.MustCompile(`p(\d+)\.webp$`)
	dateSuffixRE = regexp.MustCompile(`\s*\d{4}-\d{2}-\d{2}$`)
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
		CanBrowse:      false,
		SupportedSorts: nil,
		HasTags:        false,
	}
}

func (p *Provider) MatchesURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Host), "readcomicsonline.lol")
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	reqURL := fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(query))
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
	seen := make(map[string]bool)

	doc.Find("a[href^='/comic/']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		img := s.Find("img").First()
		alt, hasAlt := img.Attr("alt")
		if !hasAlt || alt == "" {
			return
		}

		title := strings.TrimSuffix(alt, " comic cover")
		title = strings.TrimSpace(title)
		fullURL := p.resolveURL(href)

		if !seen[fullURL] && title != "" {
			seen[fullURL] = true
			parts := strings.Split(strings.TrimRight(href, "/"), "/")
			seriesID := parts[len(parts)-1]

			results = append(results, domain.Series{
				ID:    seriesID,
				Title: title,
				URL:   fullURL,
			})
		}
	})

	return results, nil
}

func (p *Provider) Browse(_ context.Context, _ provider.BrowseOptions) ([]domain.Series, error) {
	return nil, fmt.Errorf("browse not supported by readcomicsonline")
}

func (p *Provider) GetTags(_ context.Context) ([]domain.Tag, error) {
	return nil, nil
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

	u, _ := url.Parse(series.URL)
	seriesPath := u.Path

	type rawItem struct {
		title  string
		url    string
		number float64
	}

	var rawItems []rawItem
	seen := make(map[string]bool)

	selector := fmt.Sprintf("a[href^='%s/']", seriesPath)
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "Start Reading" || text == "" {
			return
		}

		href, exists := s.Attr("href")
		if !exists {
			return
		}
		fullURL := p.resolveURL(href)
		if seen[fullURL] {
			return
		}

		parts := strings.Split(strings.TrimRight(href, "/"), "/")
		lastPart := parts[len(parts)-1]
		num, err := strconv.ParseFloat(lastPart, 64)
		if err != nil {
			return
		}

		seen[fullURL] = true
		cleanedTitle := dateSuffixRE.ReplaceAllString(text, "")
		rawItems = append(rawItems, rawItem{
			title:  cleanedTitle,
			url:    fullURL,
			number: num,
		})
	})

	// Sort ascending by issue number
	sort.Slice(rawItems, func(i, j int) bool {
		return rawItems[i].number < rawItems[j].number
	})

	var chapters []domain.Chapter
	for i, item := range rawItems {
		numVal := item.number
		chapters = append(chapters, domain.Chapter{
			ID:            fmt.Sprintf("%g", numVal),
			SeriesID:      series.ID,
			Title:         item.title,
			URL:           item.url,
			Number:        &numVal,
			OriginalLabel: item.title,
			Index:         i,
		})
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
	matches := webpPageRE.FindAllString(htmlContent, -1)

	type pageItem struct {
		url string
		num int
	}

	var parsedPages []pageItem
	seen := make(map[string]bool)

	for _, m := range matches {
		if seen[m] {
			continue
		}
		seen[m] = true

		sub := pageNumRE.FindStringSubmatch(m)
		if len(sub) >= 2 {
			n, _ := strconv.Atoi(sub[1])
			parsedPages = append(parsedPages, pageItem{url: m, num: n})
		}
	}

	sort.Slice(parsedPages, func(i, j int) bool {
		return parsedPages[i].num < parsedPages[j].num
	})

	var pages []domain.Page
	for i, item := range parsedPages {
		pages = append(pages, domain.Page{
			URL:   item.url,
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
