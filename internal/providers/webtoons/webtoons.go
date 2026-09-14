package webtoons

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "webtoons"
	Name           = "Webtoons"
	DefaultBaseURL = "https://www.webtoons.com"
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
		SupportedSorts: []domain.SortOrder{domain.SortPopular, domain.SortRecent, domain.SortRating},
		HasTags:        true,
	}
}

func (p *Provider) MatchesURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "webtoons.com") || strings.Contains(host, "linewebtoon.com")
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	reqURL := fmt.Sprintf("%s/en/search?keyword=%s", p.baseURL, url.QueryEscape(query))
	return p.fetchSeries(ctx, reqURL)
}

func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	var reqURL string
	if opts.Tag != nil && opts.Tag.ID != "" {
		reqURL = fmt.Sprintf("%s/en/genres/%s", p.baseURL, strings.ToLower(opts.Tag.ID))
	} else {
		reqURL = fmt.Sprintf("%s/en/genres", p.baseURL)
	}

	sortParam := "MANA"
	switch opts.Sort {
	case domain.SortRecent:
		sortParam = "UPDATE"
	case domain.SortRating:
		sortParam = "LIKEIT"
	}

	reqURL = fmt.Sprintf("%s?sortOrder=%s", reqURL, sortParam)
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
	seen := make(map[string]bool)

	doc.Find("a._card_item, a._genre_title_a, ul.webtoon_list li a, ul.card_lst li a, a[href*='list?title_no=']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}
		if !strings.Contains(href, "title_no=") && !strings.Contains(href, "/list?") {
			return
		}

		seriesURL := p.resolveURL(href)
		seriesID := extractSeriesID(seriesURL, s)
		if seriesID == "" {
			return
		}

		if seen[seriesID] {
			return
		}

		title := cleanSeriesTitle(s)
		if title == "" {
			return
		}

		seen[seriesID] = true
		results = append(results, domain.Series{
			ID:    seriesID,
			Title: title,
			URL:   seriesURL,
		})
	})

	return results, nil
}

func extractSeriesID(rawURL string, s *goquery.Selection) string {
	if s != nil {
		if id, exists := s.Attr("data-title-no"); exists && id != "" {
			return id
		}
	}
	u, err := url.Parse(rawURL)
	if err == nil {
		if id := u.Query().Get("title_no"); id != "" {
			return id
		}
	}
	parts := strings.Split(strings.TrimRight(rawURL, "/"), "/")
	for i, part := range parts {
		if part == "list" && i > 0 {
			return parts[i-1]
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func cleanSeriesTitle(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find("span.badge, [class*='badge'], svg, i, em.ico").Remove()

	var title string
	if t := clone.Find(".info_text strong.title, .info_text .title, strong.title, p.title, .subj").First(); t.Length() > 0 {
		title = strings.TrimSpace(t.Text())
	}
	if title == "" {
		if t := clone.Find(".title").First(); t.Length() > 0 {
			title = strings.TrimSpace(t.Text())
		}
	}
	if title == "" {
		title = strings.TrimSpace(clone.Text())
	}
	return strings.Join(strings.Fields(title), " ")
}

func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	reqURL := fmt.Sprintf("%s/en/genres", p.baseURL)
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

	var tags []domain.Tag
	seen := make(map[string]bool)

	doc.Find("div.snb_wrap a.snb_tab, ul.snb a.snb_tab, a.snb_tab").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Text())
		if label == "" {
			return
		}

		var id string
		if genre, exists := s.Attr("data-genre"); exists && genre != "" {
			id = strings.ToLower(genre)
		}
		if id == "" {
			href, _ := s.Attr("href")
			if href != "" {
				u, err := url.Parse(href)
				if err == nil {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					for i, part := range parts {
						if part == "genres" && i+1 < len(parts) {
							id = strings.ToLower(parts[i+1])
							break
						}
					}
				}
			}
		}
		if id == "" {
			id = strings.ToLower(strings.ReplaceAll(label, " ", "_"))
		}

		if seen[id] {
			return
		}
		seen[id] = true
		tags = append(tags, domain.Tag{
			ID:    id,
			Label: label,
		})
	})

	return tags, nil
}

func (p *Provider) GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error) {
	seriesID := series.ID
	if seriesID == "" {
		seriesID = extractSeriesID(series.URL, nil)
	}

	startURL := series.URL
	if startURL == "" {
		startURL = fmt.Sprintf("%s/episodeList?titleNo=%s", p.baseURL, seriesID)
	}
	startURL = p.resolveURL(startURL)

	var allChapters []domain.Chapter
	seen := make(map[string]bool)
	baseURL := startURL

	for page := 1; page <= 500; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		reqURL, err := setURLPage(baseURL, page)
		if err != nil {
			return nil, fmt.Errorf("building page url: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			if page == 1 {
				return nil, fmt.Errorf("unexpected status: %s", resp.Status)
			}
			break
		}

		if page == 1 && resp.Request != nil && resp.Request.URL != nil {
			baseURL = resp.Request.URL.String()
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("parsing html: %w", err)
		}

		var pageChapters []domain.Chapter
		doc.Find("li._episodeItem, li.detail_list_item, #_episodeList li").Each(func(_ int, s *goquery.Selection) {
			link := s.Find("a.detail_list_link, a[href*='/viewer']").First()
			href, exists := link.Attr("href")
			if !exists || href == "" {
				return
			}
			chapterURL := p.resolveURL(href)

			epNoStr, _ := s.Attr("data-episode-no")
			if epNoStr == "" {
				if u, err := url.Parse(chapterURL); err == nil {
					epNoStr = u.Query().Get("episode_no")
				}
			}

			chapterID := epNoStr
			if chapterID == "" {
				parts := strings.Split(strings.TrimRight(chapterURL, "/"), "/")
				chapterID = parts[len(parts)-1]
			}

			if seen[chapterURL] || (chapterID != "" && seen[chapterID]) {
				return
			}

			title := strings.TrimSpace(s.Find(".subj span, .subj, .title").First().Text())
			if title == "" {
				title = strings.TrimSpace(link.Text())
			}
			if title == "" {
				if epNoStr != "" {
					title = fmt.Sprintf("Episode %s", epNoStr)
				} else {
					title = fmt.Sprintf("Chapter %d", len(allChapters)+len(pageChapters)+1)
				}
			}

			var numPtr *float64
			if epNoStr != "" {
				if n, err := strconv.ParseFloat(epNoStr, 64); err == nil {
					numPtr = &n
				}
			}
			if numPtr == nil {
				numPtr = domain.ParseChapterNumber(title)
			}

			pageChapters = append(pageChapters, domain.Chapter{
				ID:            chapterID,
				SeriesID:      seriesID,
				Title:         title,
				URL:           chapterURL,
				OriginalLabel: title,
				Number:        numPtr,
			})
		})

		if len(pageChapters) == 0 {
			break
		}

		for _, ch := range pageChapters {
			seen[ch.URL] = true
			if ch.ID != "" {
				seen[ch.ID] = true
			}
			allChapters = append(allChapters, ch)
		}

		if shouldStopPagination(doc, page) {
			break
		}
	}

	// Reverse to ascending reading order
	for i, j := 0, len(allChapters)-1; i < j; i, j = i+1, j-1 {
		allChapters[i], allChapters[j] = allChapters[j], allChapters[i]
	}
	for i := range allChapters {
		allChapters[i].Index = i
	}

	return allChapters, nil
}

func shouldStopPagination(doc *goquery.Document, currentPage int) bool {
	paginate := doc.Find(".paginate, div.paginate")
	if paginate.Length() == 0 {
		return true
	}

	// If there is an active next button, continue
	if paginate.Find("a.pg_next").Length() > 0 {
		return false
	}

	// No next button; check highest page link
	maxPage := currentPage
	paginate.Find("a.pg_page").Each(func(_ int, s *goquery.Selection) {
		pageNum, err := strconv.Atoi(strings.TrimSpace(s.Text()))
		if err == nil && pageNum > maxPage {
			maxPage = pageNum
		}
	})

	return currentPage >= maxPage
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	var pages []domain.Page
	seen := make(map[string]bool)
	index := 0

	doc.Find("#_imageList img, .viewer_img img, img._images").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("data-url")
		if !exists || src == "" {
			src, exists = s.Attr("src")
		}
		if !exists || src == "" {
			return
		}
		if strings.Contains(src, "bg_transparency.png") {
			return
		}
		if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
			return
		}
		if seen[src] {
			return
		}
		seen[src] = true

		pages = append(pages, domain.Page{
			URL:   src,
			Index: index,
			Headers: map[string][]string{
				"Referer": {"https://www.webtoons.com/"},
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

func setURLPage(rawURL string, page int) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
