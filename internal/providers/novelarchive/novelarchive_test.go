package novelarchive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

const testNovelID = "6a71b6131e3db0813cb55d30"

// newTestProvider starts a stub API server and returns a provider pointed at it.
func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewWithBaseURL(srv.URL, srv.Client())
}

func TestCapabilities(t *testing.T) {
	caps := New(nil).Capabilities()

	if !caps.CanSearch || !caps.CanBrowse {
		t.Errorf("CanSearch/CanBrowse = %t/%t, want both true", caps.CanSearch, caps.CanBrowse)
	}
	if !caps.HasTags {
		t.Error("HasTags = false, want true")
	}
	if caps.Media != domain.MediaText || !caps.IsText() {
		t.Errorf("Media = %q, want %q", caps.Media, domain.MediaText)
	}

	want := map[domain.SortOrder]bool{
		domain.SortRecent:  true,
		domain.SortPopular: true,
		domain.SortRating:  true,
	}
	if len(caps.SupportedSorts) != len(want) {
		t.Fatalf("SupportedSorts = %v, want %d entries", caps.SupportedSorts, len(want))
	}
	for _, s := range caps.SupportedSorts {
		if !want[s] {
			t.Errorf("unexpected sort %q", s)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	p := New(nil)

	tests := []struct {
		url  string
		want bool
	}{
		{"https://novelarchive.st/novel?id=" + testNovelID, true},
		{"https://novelarchive.st/reader?novel=" + testNovelID + "&chapter=1", true},
		{"http://www.novelarchive.st/", true},
		{"https://novelarchive.example/novel?id=" + testNovelID, false},
		{"https://novelping.com/book/some-novel", false},
		{"not a url", false},
	}

	for _, tc := range tests {
		if got := p.MatchesURL(tc.url); got != tc.want {
			t.Errorf("MatchesURL(%q) = %t, want %t", tc.url, got, tc.want)
		}
	}
}

func TestBrowse(t *testing.T) {
	tests := []struct {
		name   string
		opts   provider.BrowseOptions
		want   map[string]string
		absent []string
	}{
		{
			name: "default sort and page",
			opts: provider.BrowseOptions{},
			want: map[string]string{"page": "1", "sort": "recent", "limit": "50"},
		},
		{
			name: "popular page 2",
			opts: provider.BrowseOptions{Sort: domain.SortPopular, Page: 2},
			want: map[string]string{"page": "2", "sort": "popular", "limit": "50"},
		},
		{
			name: "rating",
			opts: provider.BrowseOptions{Sort: domain.SortRating},
			want: map[string]string{"sort": "rating"},
		},
		{
			name: "alphabetical is not offered and degrades to recent",
			opts: provider.BrowseOptions{Sort: domain.SortAlphabetical},
			want: map[string]string{"sort": "recent"},
		},
		{
			name: "genre tag filter",
			opts: provider.BrowseOptions{Tag: &domain.Tag{ID: "martial-arts", Label: "Martial Arts"}},
			want: map[string]string{"genres_include": "martial-arts", "genre_match": "any"},
		},
		{
			name: "tag label missing falls back to id",
			opts: provider.BrowseOptions{Tag: &domain.Tag{ID: "Action"}},
			want: map[string]string{"genres_include": "Action"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]string
			p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/novels" {
					t.Errorf("path = %q, want /api/novels", r.URL.Path)
				}
				got = queryMap(r)
				writeJSON(t, w, `{"novels":[]}`)
			})

			if _, err := p.Browse(context.Background(), tc.opts); err != nil {
				t.Fatalf("Browse() error = %v", err)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("query %q = %q, want %q", k, got[k], want)
				}
			}
			for _, k := range tc.absent {
				if _, ok := got[k]; ok {
					t.Errorf("query unexpectedly contains %q", k)
				}
			}
			if tc.opts.Tag == nil {
				if _, ok := got["genres_include"]; ok {
					t.Errorf("query unexpectedly contains genres_include: %v", got)
				}
			}
		})
	}
}

func TestBrowseMapsNovels(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, `{"novels":[
			{"id":"`+testNovelID+`","title":"  The Quest for Immortality Novel  ","genres":"Action, Fantasy","chapter_names":["Chapter 1: Mo Hua"]},
			{"id":"","title":"dropped: no id"}
		]}`)
	})

	series, err := p.Browse(context.Background(), provider.BrowseOptions{})
	if err != nil {
		t.Fatalf("Browse() error = %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("got %d series, want 1 (novels without an id are dropped)", len(series))
	}
	if series[0].ID != testNovelID {
		t.Errorf("ID = %q, want %q", series[0].ID, testNovelID)
	}
	if series[0].Title != "The Quest for Immortality Novel" {
		t.Errorf("Title = %q, want trimmed title", series[0].Title)
	}
	if want := p.baseURL + "/novel?id=" + testNovelID; series[0].URL != want {
		t.Errorf("URL = %q, want %q", series[0].URL, want)
	}
}

func TestSearch(t *testing.T) {
	var gotQuery, gotAcceptLanguage, gotAccept string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("search")
		gotAcceptLanguage = r.Header.Get("Accept-Language")
		gotAccept = r.Header.Get("Accept")
		writeJSON(t, w, `{"novels":[{"id":"`+testNovelID+`","title":"Lord of the Mysteries"}]}`)
	})

	series, err := p.Search(context.Background(), "lord of the mysteries")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if gotQuery != "lord of the mysteries" {
		t.Errorf("search param = %q, want unescaped query round-trip", gotQuery)
	}
	if gotAcceptLanguage == "" {
		t.Error("request sent no Accept-Language; Cloudflare answers 403 without it")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
	if len(series) != 1 || series[0].Title != "Lord of the Mysteries" {
		t.Errorf("series = %+v, want one titled result", series)
	}
}

func TestGetChapters(t *testing.T) {
	var gotPath string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, `{"novel":{"id":"`+testNovelID+`","title":"The Quest for Immortality Novel",
			"chapter_names":["Chapter 1: Mo Hua","Chapter 2: The Dao Stele","Chapter 1684"]}}`)
	})

	// A series resolved from a pasted URL carries no id — only the URL.
	series := domain.Series{URL: "https://novelarchive.st/novel?id=" + testNovelID}
	chapters, err := p.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters() error = %v", err)
	}

	if gotPath != "/api/novels/"+testNovelID {
		t.Errorf("path = %q, want /api/novels/%s", gotPath, testNovelID)
	}
	if len(chapters) != 3 {
		t.Fatalf("got %d chapters, want 3", len(chapters))
	}

	for i, want := range []struct {
		id     string
		number float64
	}{{"1", 1}, {"2", 2}, {"3", 1684}} {
		ch := chapters[i]
		if ch.ID != want.id {
			t.Errorf("chapters[%d].ID = %q, want ordinal %q", i, ch.ID, want.id)
		}
		if ch.Index != i {
			t.Errorf("chapters[%d].Index = %d, want %d", i, ch.Index, i)
		}
		if ch.Number == nil || *ch.Number != want.number {
			t.Errorf("chapters[%d].Number = %v, want %v", i, ch.Number, want.number)
		}
		if ch.SeriesID != testNovelID {
			t.Errorf("chapters[%d].SeriesID = %q, want resolved novel id", i, ch.SeriesID)
		}
		wantURL := p.baseURL + "/reader?novel=" + testNovelID + "&chapter=" + want.id
		if ch.URL != wantURL {
			t.Errorf("chapters[%d].URL = %q, want %q", i, ch.URL, wantURL)
		}
	}

	if chapters[0].Title != "Chapter 1: Mo Hua" || chapters[0].OriginalLabel != "Chapter 1: Mo Hua" {
		t.Errorf("chapter 1 title/label = %q/%q, want the API name preserved",
			chapters[0].Title, chapters[0].OriginalLabel)
	}
}

// TestGetChaptersFromReaderURL covers a reader (chapter) link pasted where a
// series URL belongs: it carries ?novel= instead of ?id=, and chapter fetching
// was the flow that reached novelID with it.
func TestGetChaptersFromReaderURL(t *testing.T) {
	var gotPath string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, `{"novel":{"id":"`+testNovelID+`","title":"Shadow Slave",
			"chapter_names":["Chapter 1: Nightmare Begins","Chapter 2: Slave Caravan"]}}`)
	})

	series := domain.Series{URL: p.baseURL + "/reader?novel=" + testNovelID + "&chapter=2"}
	chapters, err := p.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters() error = %v", err)
	}

	if gotPath != "/api/novels/"+testNovelID {
		t.Errorf("path = %q, want /api/novels/%s resolved from the reader URL", gotPath, testNovelID)
	}
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2", len(chapters))
	}
	if chapters[0].SeriesID != testNovelID {
		t.Errorf("chapters[0].SeriesID = %q, want %q", chapters[0].SeriesID, testNovelID)
	}
}

func TestGetChaptersErrors(t *testing.T) {
	t.Run("api error field", func(t *testing.T) {
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, `{"error":"Not found"}`)
		})

		_, err := p.GetChapters(context.Background(), domain.Series{ID: testNovelID})
		if err == nil || !strings.Contains(err.Error(), "Not found") {
			t.Fatalf("error = %v, want it to surface the API error", err)
		}
	})

	t.Run("no novel id", func(t *testing.T) {
		p := New(nil)
		if _, err := p.GetChapters(context.Background(), domain.Series{}); err == nil {
			t.Fatal("error = nil, want a missing-id error")
		}
	})

	t.Run("empty chapter list is not an error", func(t *testing.T) {
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, `{"novel":{"id":"`+testNovelID+`","chapter_names":[]}}`)
		})

		chapters, err := p.GetChapters(context.Background(), domain.Series{ID: testNovelID})
		if err != nil {
			t.Fatalf("GetChapters() error = %v, want nil for a novel with no chapters", err)
		}
		if len(chapters) != 0 {
			t.Errorf("got %d chapters, want 0", len(chapters))
		}
	})
}

func TestGetChapterText(t *testing.T) {
	t.Run("single-newline prose gains blank-line paragraphs", func(t *testing.T) {
		var gotPath string
		p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			writeJSON(t, w, `{"chapter":{"number":1,"name":"Chapter 1: Crimson",
				"content":"Chapter 1: Crimson\nTranslator:\nAtlasStudios\n\nPainful!\nHow painful!\n"}}`)
		})

		ch := domain.Chapter{
			ID:  "1",
			URL: p.baseURL + "/reader?novel=" + testNovelID + "&chapter=1",
		}
		body, err := p.GetChapterText(context.Background(), ch)
		if err != nil {
			t.Fatalf("GetChapterText() error = %v", err)
		}
		if gotPath != "/api/novels/"+testNovelID+"/chapters/1" {
			t.Errorf("path = %q, want the ordinal-scoped chapter endpoint", gotPath)
		}

		// Every line is a paragraph, whether the source separated them with a
		// blank line or not.
		want := "Chapter 1: Crimson\n\nTranslator:\n\nAtlasStudios\n\nPainful!\n\nHow painful!"
		if body != want {
			t.Errorf("body = %q, want %q", body, want)
		}
	})

	t.Run("blank-line prose is preserved", func(t *testing.T) {
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, `{"chapter":{"content":"  First paragraph.\n\nSecond paragraph.\n\n"}}`)
		})

		body, err := p.GetChapterText(context.Background(), domain.Chapter{ID: "2", SeriesID: testNovelID})
		if err != nil {
			t.Fatalf("GetChapterText() error = %v", err)
		}
		if want := "First paragraph.\n\nSecond paragraph."; body != want {
			t.Errorf("body = %q, want %q", body, want)
		}
	})

	t.Run("from ids only", func(t *testing.T) {
		var gotPath string
		p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			writeJSON(t, w, `{"chapter":{"content":"Body."}}`)
		})

		ch := domain.Chapter{ID: "7", SeriesID: testNovelID}
		if _, err := p.GetChapterText(context.Background(), ch); err != nil {
			t.Fatalf("GetChapterText() error = %v", err)
		}
		if gotPath != "/api/novels/"+testNovelID+"/chapters/7" {
			t.Errorf("path = %q, want the ordinal-scoped chapter endpoint", gotPath)
		}
	})

	t.Run("missing chapter", func(t *testing.T) {
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, `{"error":"Chapter does not exist"}`)
		})

		_, err := p.GetChapterText(context.Background(), domain.Chapter{ID: "1", SeriesID: testNovelID})
		if err == nil || !strings.Contains(err.Error(), "Chapter does not exist") {
			t.Fatalf("error = %v, want it to surface the API error", err)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, `{"chapter":{"content":"   "}}`)
		})

		_, err := p.GetChapterText(context.Background(), domain.Chapter{ID: "1", SeriesID: testNovelID})
		if err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("error = %v, want an empty-body error", err)
		}
	})
}

func TestGetTags(t *testing.T) {
	var gotPath string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, `{"genres":[
			{"value":"action","label":"Action"},
			{"value":"fantasy","label":"Fantasy"},
			{"value":"action","label":"Action (duplicate)"},
			{"value":"","label":"No value"},
			{"value":"orphan","label":""},
			{"value":"martial-arts","label":"Martial Arts"}
		]}`)
	})

	tags, err := p.GetTags(context.Background())
	if err != nil {
		t.Fatalf("GetTags() error = %v", err)
	}

	if gotPath != "/api/novels/genres" {
		t.Errorf("path = %q, want /api/novels/genres", gotPath)
	}

	want := []domain.Tag{
		{ID: "action", Label: "Action"},
		{ID: "fantasy", Label: "Fantasy"},
		{ID: "martial-arts", Label: "Martial Arts"},
	}
	if len(tags) != len(want) {
		t.Fatalf("tags = %+v, want %d entries", tags, len(want))
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Errorf("tag[%d] = %+v, want %+v", i, tag, want[i])
		}
	}
}

func TestGetTagsFailsInsteadOfReturningAnEmptyVocabulary(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		// 404 is not retried, so this fails immediately.
		http.NotFound(w, r)
	})

	tags, err := p.GetTags(context.Background())
	if err == nil {
		t.Fatal("GetTags() error = nil, want a failure when the vocabulary cannot be read")
	}
	if tags != nil {
		t.Errorf("tags = %v, want nil: an empty list would erase a caller's stored vocabulary", tags)
	}
}

func TestBrowseSendsTagValueNotLabel(t *testing.T) {
	var gotQuery map[string]string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = queryMap(r)
		writeJSON(t, w, `{"novels":[]}`)
	})

	// "Completed" as a label matches nothing upstream; "completed" matches two.
	tag := &domain.Tag{ID: "completed", Label: "Completed"}
	if _, err := p.Browse(context.Background(), provider.BrowseOptions{Tag: tag}); err != nil {
		t.Fatalf("Browse() error = %v", err)
	}

	if got := gotQuery["genres_include"]; got != "completed" {
		t.Errorf("genres_include = %q, want the filter value \"completed\"", got)
	}
}

func TestGetPagesIsTextOnly(t *testing.T) {
	p := New(nil)
	_, err := p.GetPages(context.Background(), domain.Chapter{ID: "1"})
	if !errors.Is(err, ErrTextOnly) {
		t.Fatalf("error = %v, want ErrTextOnly", err)
	}
}

func queryMap(r *http.Request) map[string]string {
	out := make(map[string]string)
	for k, v := range r.URL.Query() {
		out[k] = strings.Join(v, ",")
	}
	return out
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("writing fixture: %v", err)
	}
}

func TestGetJSONRetriesTransientFailures(t *testing.T) {
	// Collapse the backoff so the retry tests stay instant.
	original := retryDelay
	retryDelay = time.Millisecond
	t.Cleanup(func() { retryDelay = original })

	t.Run("retries a 504 once", func(t *testing.T) {
		var calls int
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			writeJSON(t, w, `{"novels":[{"id":"`+testNovelID+`","title":"Recovered"}]}`)
		})

		series, err := p.Browse(context.Background(), provider.BrowseOptions{})
		if err != nil {
			t.Fatalf("Browse() error = %v, want the retry to succeed", err)
		}
		if calls != 2 {
			t.Errorf("calls = %d, want 2", calls)
		}
		if len(series) != 1 || series[0].Title != "Recovered" {
			t.Errorf("series = %+v, want the retried response", series)
		}
	})

	t.Run("gives up after the last attempt", func(t *testing.T) {
		var calls int
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(http.StatusBadGateway)
		})

		_, err := p.Browse(context.Background(), provider.BrowseOptions{})
		if err == nil || !strings.Contains(err.Error(), "502") {
			t.Fatalf("error = %v, want the 502 surfaced", err)
		}
		if calls != maxAttempts {
			t.Errorf("calls = %d, want %d", calls, maxAttempts)
		}
	})

	t.Run("does not retry a 404", func(t *testing.T) {
		var calls int
		p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(http.StatusNotFound)
		})

		if _, err := p.Browse(context.Background(), provider.BrowseOptions{}); err == nil {
			t.Fatal("error = nil, want a 404 error")
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1 (a 404 is not transient)", calls)
		}
	})
}
