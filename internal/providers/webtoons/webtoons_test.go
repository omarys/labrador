package webtoons_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/webtoons"
)

func TestProvider_Metadata(t *testing.T) {
	prov := webtoons.New(nil)
	if prov.ID() != "webtoons" {
		t.Errorf("expected ID 'webtoons', got %q", prov.ID())
	}
	if prov.Name() != "Webtoons" {
		t.Errorf("expected Name 'Webtoons', got %q", prov.Name())
	}

	caps := prov.Capabilities()
	if !caps.CanSearch {
		t.Error("expected CanSearch to be true")
	}
	if !caps.CanBrowse {
		t.Error("expected CanBrowse to be true")
	}
	if !caps.HasTags {
		t.Error("expected HasTags to be true")
	}
	if len(caps.SupportedSorts) != 3 {
		t.Errorf("expected 3 supported sorts, got %d", len(caps.SupportedSorts))
	}
}

func TestProvider_MatchesURL(t *testing.T) {
	prov := webtoons.New(nil)

	testCases := []struct {
		url      string
		expected bool
	}{
		{"https://www.webtoons.com/en/fantasy/tower-of-god/list?title_no=95", true},
		{"https://webtoons.com/en/genres/drama", true},
		{"https://m.webtoons.com/en/canvas/my-comic/list?title_no=12345", true},
		{"https://WEBTOONS.COM/viewer?title_no=95&episode_no=1", true},
		{"https://www.linewebtoon.com/en/fantasy/tower-of-god/list?title_no=95", true},
		{"https://mangadex.org/title/123", false},
		{"https://weebcentral.com/series/01JJ", false},
		{"not-a-valid-url", false},
	}

	for _, tc := range testCases {
		actual := prov.MatchesURL(tc.url)
		if actual != tc.expected {
			t.Errorf("MatchesURL(%q) = %v, expected %v", tc.url, actual, tc.expected)
		}
	}
}

func TestProvider_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/en/search" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("keyword") != "tower of god" {
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}

		html := `
		<ul class="card_lst">
			<li>
				<a href="/en/fantasy/tower-of-god/list?title_no=95" class="link _card_item" data-title-no="95">
					<div class="info_text">
						<strong class="title">Tower of God</strong>
						<div class="author">SIU</div>
					</div>
				</a>
			</li>
			<li>
				<a href="https://www.webtoons.com/en/canvas/tower-of-god-fan/list?title_no=726081" class="link _card_item" data-title-no="726081">
					<div class="info_text">
						<strong class="title">Tower of God Fan Comic</strong>
						<div class="author">FanAuthor</div>
					</div>
				</a>
			</li>
			<!-- Duplicate entry to test deduplication -->
			<li>
				<a href="/en/fantasy/tower-of-god/list?title_no=95" class="link _card_item" data-title-no="95">
					<div class="info_text">
						<strong class="title">Tower of God</strong>
					</div>
				</a>
			</li>
		</ul>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	results, err := prov.Search(context.Background(), "tower of god")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 unique series, got %d", len(results))
	}

	if results[0].ID != "95" || results[0].Title != "Tower of God" {
		t.Errorf("unexpected series[0]: %+v", results[0])
	}
	if results[0].URL != ts.URL+"/en/fantasy/tower-of-god/list?title_no=95" {
		t.Errorf("unexpected series[0] URL: %s", results[0].URL)
	}

	if results[1].ID != "726081" || results[1].Title != "Tower of God Fan Comic" {
		t.Errorf("unexpected series[1]: %+v", results[1])
	}
}

func TestProvider_Search_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	_, err := prov.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error on 500 status")
	}
}

func TestProvider_Browse(t *testing.T) {
	var requestedPath string
	var requestedQuery string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery

		html := `
		<ul class="webtoon_list">
			<li>
				<a href="/en/action/eleceed/list?title_no=1571" class="link _genre_title_a" data-title-no="1571">
					<div class="info_text">
						<strong class="title">Eleceed</strong>
					</div>
				</a>
			</li>
		</ul>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())

	// 1. Default browse (no tag, default popular sort)
	results, err := prov.Browse(context.Background(), provider.BrowseOptions{})
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}
	if requestedPath != "/en/genres" || requestedQuery != "sortOrder=MANA" {
		t.Errorf("unexpected request: path=%s, query=%s", requestedPath, requestedQuery)
	}
	if len(results) != 1 || results[0].Title != "Eleceed" || results[0].ID != "1571" {
		t.Errorf("unexpected results: %+v", results)
	}

	// 2. Browse with Tag and Recent sort
	_, err = prov.Browse(context.Background(), provider.BrowseOptions{
		Tag:  &domain.Tag{ID: "action"},
		Sort: domain.SortRecent,
	})
	if err != nil {
		t.Fatalf("Browse with tag failed: %v", err)
	}
	if requestedPath != "/en/genres/action" || requestedQuery != "sortOrder=UPDATE" {
		t.Errorf("unexpected request: path=%s, query=%s", requestedPath, requestedQuery)
	}

	// 3. Browse with Rating sort
	_, err = prov.Browse(context.Background(), provider.BrowseOptions{
		Tag:  &domain.Tag{ID: "fantasy"},
		Sort: domain.SortRating,
	})
	if err != nil {
		t.Fatalf("Browse with rating sort failed: %v", err)
	}
	if requestedPath != "/en/genres/fantasy" || requestedQuery != "sortOrder=LIKEIT" {
		t.Errorf("unexpected request: path=%s, query=%s", requestedPath, requestedQuery)
	}
}

func TestProvider_GetTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/en/genres" {
			http.NotFound(w, r)
			return
		}

		html := `
		<div class="snb_wrap">
			<ul class="snb">
				<li class="snb_item"><a href="/en/genres/drama?sortOrder=MANA" class="snb_tab" data-genre="DRAMA">DRAMA</a></li>
				<li class="snb_item"><a href="/en/genres/action?sortOrder=MANA" class="snb_tab" data-genre="ACTION">ACTION</a></li>
				<li class="snb_item"><a href="/en/genres/slice_of_life?sortOrder=MANA" class="snb_tab" data-genre="SLICE_OF_LIFE">SLICE OF LIFE</a></li>
				<!-- Duplicate tag -->
				<li class="snb_item"><a href="/en/genres/drama?sortOrder=MANA" class="snb_tab" data-genre="DRAMA">DRAMA</a></li>
			</ul>
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	tags, err := prov.GetTags(context.Background())
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}

	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	if tags[0].ID != "drama" || tags[0].Label != "DRAMA" {
		t.Errorf("unexpected tag[0]: %+v", tags[0])
	}
	if tags[1].ID != "action" || tags[1].Label != "ACTION" {
		t.Errorf("unexpected tag[1]: %+v", tags[1])
	}
	if tags[2].ID != "slice_of_life" || tags[2].Label != "SLICE OF LIFE" {
		t.Errorf("unexpected tag[2]: %+v", tags[2])
	}
}

func TestProvider_GetChapters_Pagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			html := `
			<ul id="_episodeList">
				<li class="_episodeItem" data-episode-no="4">
					<a href="/en/fantasy/tower/ep-4/viewer?title_no=95&episode_no=4" class="detail_list_link">
						<span class="subj"><span>Episode 4</span></span>
					</a>
				</li>
				<li class="_episodeItem" data-episode-no="3">
					<a href="/en/fantasy/tower/ep-3/viewer?title_no=95&episode_no=3" class="detail_list_link">
						<span class="subj"><span>Episode 3</span></span>
					</a>
				</li>
			</ul>
			<div class="paginate">
				<a href="#" class="pg_page" aria-current="true"><span>1</span></a>
				<a href="/en/fantasy/tower/list?title_no=95&page=2" class="pg_page"><span>2</span></a>
				<a href="/en/fantasy/tower/list?title_no=95&page=2" class="pg_next"><em class="blind">Next Page</em></a>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		if page == "2" {
			html := `
			<ul id="_episodeList">
				<li class="_episodeItem" data-episode-no="2">
					<a href="/en/fantasy/tower/ep-2/viewer?title_no=95&episode_no=2" class="detail_list_link">
						<span class="subj"><span>Episode 2</span></span>
					</a>
				</li>
				<li class="_episodeItem" data-episode-no="1">
					<a href="/en/fantasy/tower/ep-1/viewer?title_no=95&episode_no=1" class="detail_list_link">
						<span class="subj"><span>Episode 1</span></span>
					</a>
				</li>
			</ul>
			<div class="paginate">
				<a href="/en/fantasy/tower/list?title_no=95&page=1" class="pg_page"><span>1</span></a>
				<a href="#" class="pg_page" aria-current="true"><span>2</span></a>
				<span class="pg_next off"><em class="blind">Next Page</em></span>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		// Page 3+ clamp returns page 2 content to test deduplication stop
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	series := domain.Series{
		ID:  "95",
		URL: ts.URL + "/en/fantasy/tower/list?title_no=95",
	}

	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}

	// Chapters must be in ascending reading order: 1, 2, 3, 4
	if len(chapters) != 4 {
		t.Fatalf("expected 4 chapters, got %d", len(chapters))
	}

	for i, ch := range chapters {
		expectedNum := float64(i + 1)
		expectedTitle := fmt.Sprintf("Episode %d", i+1)
		if ch.Index != i {
			t.Errorf("chapter[%d]: expected Index %d, got %d", i, i, ch.Index)
		}
		if ch.Title != expectedTitle {
			t.Errorf("chapter[%d]: expected Title %q, got %q", i, expectedTitle, ch.Title)
		}
		if ch.Number == nil || *ch.Number != expectedNum {
			t.Errorf("chapter[%d]: expected Number %v, got %v", i, expectedNum, ch.Number)
		}
		if ch.SeriesID != "95" {
			t.Errorf("chapter[%d]: expected SeriesID '95', got %q", i, ch.SeriesID)
		}
	}
}

func TestProvider_GetChapters_SinglePage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<ul id="_episodeList">
			<li class="_episodeItem" data-episode-no="1">
				<a href="/en/fantasy/oneshot/ep-1/viewer?title_no=1&episode_no=1" class="detail_list_link">
					<span class="subj"><span>Chapter 1</span></span>
				</a>
			</li>
		</ul>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	series := domain.Series{
		ID:  "1",
		URL: ts.URL + "/en/fantasy/oneshot/list?title_no=1",
	}

	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}

	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}
	if chapters[0].Title != "Chapter 1" || chapters[0].Index != 0 {
		t.Errorf("unexpected chapter: %+v", chapters[0])
	}
}

func TestProvider_GetChapters_ContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<ul id="_episodeList"></ul>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	series := domain.Series{ID: "1", URL: ts.URL + "/list?title_no=1"}
	_, err := prov.GetChapters(ctx, series)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

func TestProvider_GetPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/viewer" {
			http.NotFound(w, r)
			return
		}

		html := `
		<div id="content" class="viewer">
			<div class="viewer_img" id="_imageList">
				<img src="https://webtoons-static.pstatic.net/image/bg_transparency.png" class="_images" data-url="https://cdn.example.com/page1.jpg" />
				<img src="https://webtoons-static.pstatic.net/image/bg_transparency.png" class="_images" data-url="https://cdn.example.com/page2.jpg" />
				<!-- Duplicate image -->
				<img src="https://webtoons-static.pstatic.net/image/bg_transparency.png" class="_images" data-url="https://cdn.example.com/page2.jpg" />
			</div>
			<!-- Sidebar thumbnail outside _imageList should NOT be included -->
			<div class="aside">
				<img src="https://cdn.example.com/thumb_123.png" data-url="https://cdn.example.com/thumb_123.png" />
			</div>
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	chapter := domain.Chapter{
		ID:  "1",
		URL: ts.URL + "/viewer?title_no=95&episode_no=1",
	}

	pages, err := prov.GetPages(context.Background(), chapter)
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}

	if pages[0].URL != "https://cdn.example.com/page1.jpg" || pages[0].Index != 0 {
		t.Errorf("unexpected page[0]: %+v", pages[0])
	}
	if pages[1].URL != "https://cdn.example.com/page2.jpg" || pages[1].Index != 1 {
		t.Errorf("unexpected page[1]: %+v", pages[1])
	}

	// Verify required Referer header
	if ref, ok := pages[0].Headers["Referer"]; !ok || len(ref) == 0 || ref[0] != "https://www.webtoons.com/" {
		t.Errorf("expected Referer header 'https://www.webtoons.com/', got: %+v", pages[0].Headers)
	}
}

func TestProvider_GetPages_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	prov := webtoons.NewWithBaseURL(ts.URL, ts.Client())
	chapter := domain.Chapter{ID: "1", URL: ts.URL + "/viewer"}
	_, err := prov.GetPages(context.Background(), chapter)
	if err == nil {
		t.Fatal("expected error on 404 status")
	}
}
