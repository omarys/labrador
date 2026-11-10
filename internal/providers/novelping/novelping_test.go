package novelping_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/novelping"
)

func newProvider(t *testing.T, handler http.HandlerFunc) (*novelping.Provider, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return novelping.NewWithBaseURL(ts.URL, ts.Client()), ts
}

func TestNovelPing_CapabilitiesAreText(t *testing.T) {
	prov := novelping.New(nil)
	caps := prov.Capabilities()

	if !caps.IsText() || caps.Media != domain.MediaText {
		t.Errorf("expected text media, got %+v", caps)
	}
	if !caps.CanSearch || !caps.CanBrowse || !caps.HasTags {
		t.Errorf("expected search/browse/tags to be enabled, got %+v", caps)
	}
	if prov.ID() != "novelping" {
		t.Errorf("unexpected ID: %s", prov.ID())
	}
}

func TestNovelPing_Search(t *testing.T) {
	prov, _ := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("keyword") != "reverend" {
			http.Error(w, "bad keyword", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`
<div id="search-results" data-search-results>
  <div class="row">
    <div class="col-xs-3"><img class="cover" src="https://images.novelping.com/novel_200_89/reverend-insanity.jpg" alt="Reverend Insanity"></div>
    <div class="col-xs-7"><div><h3 class="novel-title"><a href="https://novelping.com/book/reverend-insanity" title="Reverend Insanity">Reverend Insanity</a></h3><span class="author">Gu Zhen Ren</span></div></div>
  </div>
</div>`))
	})

	series, err := prov.Search(context.Background(), "reverend")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	if series[0].ID != "reverend-insanity" {
		t.Errorf("ID = %q, want reverend-insanity", series[0].ID)
	}
	if series[0].Title != "Reverend Insanity" {
		t.Errorf("Title = %q", series[0].Title)
	}
	if series[0].URL == "" || series[0].URL[len(series[0].URL)-len("/book/reverend-insanity"):] != "/book/reverend-insanity" {
		t.Errorf("unexpected URL: %s", series[0].URL)
	}
}

func TestNovelPing_BrowseSortAndGenre(t *testing.T) {
	var gotQuery string
	prov, _ := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`
<div class="row">
  <div class="col-xs-7"><h3 class="novel-title"><a href="/book/shadow-slave" title="Shadow Slave">Shadow Slave</a></h3></div>
</div>`))
	})

	series, err := prov.Browse(context.Background(), provider.BrowseOptions{Page: 2})
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}
	if gotQuery != "/sort/popular?page=2" {
		t.Errorf("default browse requested %q, want /sort/popular?page=2", gotQuery)
	}
	if len(series) != 1 || series[0].ID != "shadow-slave" {
		t.Fatalf("unexpected series: %+v", series)
	}

	if _, err := prov.Browse(context.Background(), provider.BrowseOptions{Sort: domain.SortRecent, Page: 3}); err != nil {
		t.Fatalf("Browse(recent) failed: %v", err)
	}
	if gotQuery != "/sort/updates?page=3" {
		t.Errorf("recent browse requested %q, want /sort/updates?page=3", gotQuery)
	}

	if _, err := prov.Browse(context.Background(), provider.BrowseOptions{Tag: &domain.Tag{ID: "fantasy"}}); err != nil {
		t.Fatalf("Browse(genre) failed: %v", err)
	}
	if gotQuery != "/novelping-genres/fantasy?page=1&sort=ALL_TIME" {
		t.Errorf("genre browse requested %q", gotQuery)
	}
}

// A tag and a sort must combine into a single request: genre pages carry their
// own ?sort= vocabulary (ALL_TIME / LASTEST), separate from the /sort/* feeds.
func TestNovelPing_BrowseTagAndSortCombine(t *testing.T) {
	var gotQuery string
	prov, _ := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`
<div class="row"><div class="col-xs-7"><h3 class="novel-title"><a href="/book/shadow-slave" title="Shadow Slave">Shadow Slave</a></h3></div></div>`))
	})

	tests := []struct {
		name string
		sort domain.SortOrder
		page int
		want string
	}{
		{"popular", domain.SortPopular, 1, "/novelping-genres/fantasy?page=1&sort=ALL_TIME"},
		{"recent", domain.SortRecent, 2, "/novelping-genres/fantasy?page=2&sort=LASTEST"},
		{"unset sort", "", 3, "/novelping-genres/fantasy?page=3&sort=ALL_TIME"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotQuery = ""
			if _, err := prov.Browse(context.Background(), provider.BrowseOptions{
				Tag:  &domain.Tag{ID: "fantasy"},
				Sort: tc.sort,
				Page: tc.page,
			}); err != nil {
				t.Fatalf("Browse failed: %v", err)
			}
			if gotQuery != tc.want {
				t.Errorf("requested %q, want %q", gotQuery, tc.want)
			}
		})
	}
}

func TestNovelPing_GetTags(t *testing.T) {
	prov, _ := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`
<div class="col-xs-6"><a href="/novelping-genres/action" title="Action">Action</a></div>
<div class="col-xs-6"><a href="https://novelping.com/novelping-genres/fantasy">Fantasy</a></div>
<div class="col-xs-6"><a href="/novelping-genres/fantasy/ongoing">Ongoing Fantasy</a></div>`))
	})

	tags, err := prov.GetTags(context.Background())
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 deduped tags, got %d: %+v", len(tags), tags)
	}
	if tags[0].ID != "action" || tags[0].Label != "Action" {
		t.Errorf("unexpected tag[0]: %+v", tags[0])
	}
	if tags[1].ID != "fantasy" || tags[1].Label != "Fantasy" {
		t.Errorf("unexpected tag[1]: %+v", tags[1])
	}
}

func TestNovelPing_GetChapters(t *testing.T) {
	prov, _ := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ajax/chapter-option" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("novelId") != "reverend-insanity" {
			http.Error(w, "bad novelId", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"novel_id":"reverend-insanity","chapters":[
			{"chapter_id":"chapter-1","chapter_name":"Chapter 1: The heart of a demon","url":"https://novelping.com/book/reverend-insanity/chapter-1","premium":false,"platinum":false},
			{"chapter_id":"chapter-2","chapter_name":"Chapter 2: A new beginning","premium":false,"platinum":false}
		]}`))
	})

	series := domain.Series{ID: "reverend-insanity", Title: "Reverend Insanity", URL: "https://novelping.com/book/reverend-insanity"}
	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].ID != "chapter-1" || chapters[0].Index != 0 {
		t.Errorf("unexpected chapter[0]: %+v", chapters[0])
	}
	if chapters[0].Number == nil || *chapters[0].Number != 1 {
		t.Errorf("chapter[0].Number = %v, want 1", chapters[0].Number)
	}
	if chapters[1].URL[len(chapters[1].URL)-len("/book/reverend-insanity/chapter-2"):] != "/book/reverend-insanity/chapter-2" {
		t.Errorf("chapter[1] URL should be derived from the slug, got %q", chapters[1].URL)
	}
	if chapters[1].SeriesID != "reverend-insanity" {
		t.Errorf("chapter[1].SeriesID = %q", chapters[1].SeriesID)
	}
}

// novelWithChapters serves /ajax/chapter-option with the given chapter names,
// one chapter per name, and returns the provider plus the series to fetch.
func novelWithChapters(t *testing.T, names ...string) (*novelping.Provider, domain.Series) {
	t.Helper()
	entries := make([]string, len(names))
	for i, name := range names {
		entries[i] = fmt.Sprintf(`{"chapter_id":"chapter-%d","chapter_name":%q}`, i+1, name)
	}
	prov, _ := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ajax/chapter-option" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"success":true,"novel_id":"mushoku-tensei-novel","chapters":[%s]}`,
			strings.Join(entries, ","))
	})
	return prov, domain.Series{ID: "mushoku-tensei-novel", Title: "Mushoku Tensei", URL: "https://novelping.com/book/mushoku-tensei-novel"}
}

// chaptersNumbered fetches chapters and returns their numbers, failing the test
// if any chapter has no number.
func chaptersNumbered(t *testing.T, prov *novelping.Provider, series domain.Series) []float64 {
	t.Helper()
	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	nums := make([]float64, len(chapters))
	for i, ch := range chapters {
		if ch.Number == nil {
			t.Fatalf("chapter[%d] %q has no number", i, ch.Title)
		}
		nums[i] = *ch.Number
		if ch.Index != i {
			t.Errorf("chapter[%d].Index = %d, want %d", i, ch.Index, i)
		}
	}
	return nums
}

// NovelPing names are volume-scoped: "Volume 1" is the first number in the
// title, so every chapter of a volume would otherwise share one number.
func TestNovelPing_GetChaptersNumbersByVolumeAndPosition(t *testing.T) {
	prov, series := novelWithChapters(t,
		"Volume 1 Prologue",
		"Volume 1  1 — Could This Be Another World?",
		"Volume 1  2 — Expressionless Maid",
		"Volume 2 Side Story 2 — Half a Year After Fedoa Region Vanishes",
		"Volume 2  1 — A Boy's Resolve",
		"Chapter 7 — Something",
	)

	got := chaptersNumbered(t, prov, series)
	want := []float64{1.0, 1.001, 1.002, 2.0, 2.001, 7.0}
	if len(got) != len(want) {
		t.Fatalf("got %d chapters, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chapter[%d].Number = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestNovelPing_GetChaptersVolumePrefixVariants(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  []float64
	}{
		{
			name:  "many spaces",
			names: []string{"Volume 12 7 — Many Spaces", "Volume 12 8 — Second"},
			want:  []float64{12.0, 12.001},
		},
		{
			name:  "lowercase",
			names: []string{"volume 3 5 — Lowercase", "volume 3 6 — Also Lowercase"},
			want:  []float64{3.0, 3.001},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prov, series := novelWithChapters(t, tc.names...)
			got := chaptersNumbered(t, prov, series)
			for i, want := range tc.want {
				if got[i] != want {
					t.Errorf("chapter[%d] (%q) Number = %v, want %v", i, tc.names[i], got[i], want)
				}
			}
		})
	}
}

// Downstream consumers key chapters by number, so a multi-volume payload must
// not repeat a single value.
func TestNovelPing_GetChaptersNumbersAreUnique(t *testing.T) {
	prov, series := novelWithChapters(t,
		"Volume 1 Prologue",
		"Volume 1  1 — Could This Be Another World?",
		"Volume 1  2 — Expressionless Maid",
		"Volume 1  3 — Master and Student",
		"Volume 2  1 — A Boy's Resolve",
		"Volume 2 Side Story 1 — The Maid's Day Off",
		"Volume 2 Side Story 2 — Half a Year After Fedoa Region Vanishes",
		"Volume 10  1 — Late Volume",
	)

	seen := make(map[float64]int)
	for i, n := range chaptersNumbered(t, prov, series) {
		if prev, ok := seen[n]; ok {
			t.Fatalf("chapter[%d] repeats number %v from chapter[%d]", i, n, prev)
		}
		seen[n] = i
	}
}

func TestNovelPing_GetChapterText(t *testing.T) {
	prov, ts := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<h2><a class="chr-title"><span class="chr-text">Chapter 1: The heart of a demon</span></a></h2>
<div id="chr-content" class="chr-c" data-chapter-id="chapter-1">
  <p> “Fang Yuan, hand it over.” </p>
  <p></p>
  <p>The cave was silent.</p>
</div>`))
	})

	text, err := prov.GetChapterText(context.Background(), domain.Chapter{URL: ts.URL + "/book/x/chapter-1"})
	if err != nil {
		t.Fatalf("GetChapterText failed: %v", err)
	}
	want := "“Fang Yuan, hand it over.”\n\nThe cave was silent."
	if text != want {
		t.Fatalf("unexpected text:\n got %q\nwant %q", text, want)
	}
}

// TestNovelPing_GetChapterTextMarkdown covers the Markdown conversion: inline
// markup becomes Markdown, literal prose is escaped, and no HTML survives.
func TestNovelPing_GetChapterTextMarkdown(t *testing.T) {
	prov, ts := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<div id="chr-content" class="chr-c">
  <p>“<em>Fang Yuan</em>, hand it over,” the <strong>elder</strong> said.</p>
  <p>[Skill acquired] 2 * 3 = 6<br>and so it went on.</p>
</div>`))
	})

	body, err := prov.GetChapterText(context.Background(), domain.Chapter{URL: ts.URL + "/book/x/chapter-1"})
	if err != nil {
		t.Fatalf("GetChapterText failed: %v", err)
	}

	want := "“*Fang Yuan*, hand it over,” the **elder** said.\n\n" +
		"\\[Skill acquired\\] 2 \\* 3 = 6\\\nand so it went on."
	if body != want {
		t.Fatalf("unexpected markdown:\n got %q\nwant %q", body, want)
	}
	if strings.Contains(body, "<") {
		t.Errorf("expected no HTML in the body, got %q", body)
	}
}

// TestNovelPing_GetChapterTextDropsInjectedNotices covers the anti-piracy
// notices novelping embeds inside chapter paragraphs: one hidden with the
// `hidden` attribute, one with inline display:none.
func TestNovelPing_GetChapterTextDropsInjectedNotices(t *testing.T) {
	prov, ts := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<div id="chr-content" class="chr-c">
  <p> Fang Yuan did not care about him.<span hidden=""> Read the latest chapters at novelping dot com.</span></p>
  <p> This moment, Bai Ning Bing’s iris shrunk.<span style="display:none"> Find the original chapter at novelping(.)com.</span></p>
</div>`))
	})

	body, err := prov.GetChapterText(context.Background(), domain.Chapter{URL: ts.URL + "/book/x/chapter-137"})
	if err != nil {
		t.Fatalf("GetChapterText failed: %v", err)
	}

	want := "Fang Yuan did not care about him.\n\nThis moment, Bai Ning Bing’s iris shrunk."
	if body != want {
		t.Fatalf("unexpected markdown:\n got %q\nwant %q", body, want)
	}
	for _, notice := range []string{"latest chapters", "original chapter", "novelping dot com", "novelping(.)com"} {
		if strings.Contains(body, notice) {
			t.Errorf("injected notice %q survived: %q", notice, body)
		}
	}
}

func TestNovelPing_GetChapterTextMissingContainer(t *testing.T) {
	prov, ts := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div>nothing here</div></body></html>`))
	})
	if _, err := prov.GetChapterText(context.Background(), domain.Chapter{URL: ts.URL + "/book/x/chapter-1"}); err == nil {
		t.Fatal("expected an error when the content container is missing")
	}
}

func TestNovelPing_GetPagesIsTextOnly(t *testing.T) {
	prov := novelping.New(nil)
	if _, err := prov.GetPages(context.Background(), domain.Chapter{}); !errors.Is(err, novelping.ErrTextOnly) {
		t.Fatalf("GetPages error = %v, want ErrTextOnly", err)
	}
}

func TestNovelPing_MatchesURL(t *testing.T) {
	prov := novelping.New(nil)
	if !prov.MatchesURL("https://novelping.com/book/reverend-insanity/chapter-1") {
		t.Error("expected novelping URL to match")
	}
	if prov.MatchesURL("https://mangadex.org/title/abc") {
		t.Error("did not expect mangadex URL to match")
	}
}
