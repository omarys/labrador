package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const (
	ID             = "mangadex"
	Name           = "MangaDex"
	DefaultBaseURL = "https://api.mangadex.org"
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
	return strings.Contains(strings.ToLower(u.Host), "mangadex.org")
}

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Series, error) {
	reqURL := fmt.Sprintf("%s/manga?title=%s&limit=20&includes[]=cover_art", p.baseURL, url.QueryEscape(query))
	return p.fetchManga(ctx, reqURL)
}

func (p *Provider) Browse(ctx context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	limit := 20
	offset := 0
	if opts.Page > 1 {
		offset = (opts.Page - 1) * limit
	}

	orderParam := "order[followedCount]=desc"
	if opts.Sort == domain.SortRecent {
		orderParam = "order[latestUploadedChapter]=desc"
	}

	reqURL := fmt.Sprintf("%s/manga?limit=%d&offset=%d&%s", p.baseURL, limit, offset, orderParam)
	if opts.Tag != nil && opts.Tag.ID != "" {
		reqURL += fmt.Sprintf("&includedTags[]=%s", opts.Tag.ID)
	}

	return p.fetchManga(ctx, reqURL)
}

type mangaListResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title map[string]string `json:"title"`
		} `json:"attributes"`
	} `json:"data"`
}

func (p *Provider) fetchManga(ctx context.Context, reqURL string) ([]domain.Series, error) {
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

	var res mangaListResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding mangadex response: %w", err)
	}

	var list []domain.Series
	for _, item := range res.Data {
		title := item.Attributes.Title["en"]
		if title == "" {
			for _, v := range item.Attributes.Title {
				title = v
				break
			}
		}

		list = append(list, domain.Series{
			ID:    item.ID,
			Title: title,
			URL:   fmt.Sprintf("https://mangadex.org/title/%s", item.ID),
		})
	}

	return list, nil
}

type tagsResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Name map[string]string `json:"name"`
		} `json:"attributes"`
	} `json:"data"`
}

func (p *Provider) GetTags(ctx context.Context) ([]domain.Tag, error) {
	reqURL := fmt.Sprintf("%s/manga/tag", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var res tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding tags: %w", err)
	}

	var tags []domain.Tag
	for _, item := range res.Data {
		name := item.Attributes.Name["en"]
		if name != "" {
			tags = append(tags, domain.Tag{
				ID:    item.ID,
				Label: name,
			})
		}
	}

	return tags, nil
}

type chapterFeedResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title   string `json:"title"`
			Chapter string `json:"chapter"`
		} `json:"attributes"`
	} `json:"data"`
}

func (p *Provider) GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error) {
	mangaID := series.ID
	if mangaID == "" {
		// Extract UUID from URL
		parts := strings.Split(strings.TrimRight(series.URL, "/"), "/")
		mangaID = parts[len(parts)-1]
	}

	reqURL := fmt.Sprintf("%s/manga/%s/feed?translatedLanguage[]=en&order[chapter]=asc&limit=100", p.baseURL, mangaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var res chapterFeedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding chapters: %w", err)
	}

	var chapters []domain.Chapter
	for i, item := range res.Data {
		title := item.Attributes.Title
		if title == "" && item.Attributes.Chapter != "" {
			title = fmt.Sprintf("Chapter %s", item.Attributes.Chapter)
		} else if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}

		var numPtr *float64
		if num, err := strconv.ParseFloat(item.Attributes.Chapter, 64); err == nil {
			numPtr = &num
		}

		chapters = append(chapters, domain.Chapter{
			ID:            item.ID,
			SeriesID:      mangaID,
			Title:         title,
			URL:           fmt.Sprintf("https://mangadex.org/chapter/%s", item.ID),
			Number:        numPtr,
			OriginalLabel: title,
			Index:         i,
		})
	}

	return chapters, nil
}

type atHomeResponse struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash string   `json:"hash"`
		Data []string `json:"data"`
	} `json:"chapter"`
}

func (p *Provider) GetPages(ctx context.Context, chapter domain.Chapter) ([]domain.Page, error) {
	chapterID := chapter.ID
	if chapterID == "" {
		parts := strings.Split(strings.TrimRight(chapter.URL, "/"), "/")
		chapterID = parts[len(parts)-1]
	}

	reqURL := fmt.Sprintf("%s/at-home/server/%s", p.baseURL, chapterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var res atHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding at-home response: %w", err)
	}

	var pages []domain.Page
	for i, filename := range res.Chapter.Data {
		pageURL := fmt.Sprintf("%s/data/%s/%s", res.BaseURL, res.Chapter.Hash, filename)
		pages = append(pages, domain.Page{
			URL:   pageURL,
			Index: i,
		})
	}

	return pages, nil
}
