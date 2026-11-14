package weebcentral_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/weebcentral"
)

// The fixtures below are trimmed from live weebcentral.com responses captured
// against a desktop browser user agent: one catalogue card from
// /search/data?display_mode=Full+Display, two chapter rows from
// /series/<id>/full-chapter-list, and the body of that same endpoint for a
// series whose chapters were pulled from the site.

const seriesCard = `
<article class="bg-base-300 flex gap-4 p-4">
	<section class="w-full lg:w-[25%] xl:w-[20%]">
		<a href="https://weebcentral.com/series/01J76XY7E9FNDZ1DBBM6PBJPFK/One-Piece">
			<article class="hidden lg:block w-full aspect-4/6  overflow-hidden">
				<picture>
					<img src="https://temp.compsci88.com/cover/fallback/01J76XY7E9FNDZ1DBBM6PBJPFK.jpg" alt="One Piece cover" width="400" height="600" decoding="async">
				</picture>
			</article>
			<article class="lg:hidden relative overflow-hidden">
				<div class="absolute right-0 top-0 h-16 w-16">
					<div class="absolute transform rotate-45 bg-orange-600 text-center text-white font-semibold py-1">Official</div>
				</div>
				<div>
					<div class="w-full aspect-4/6 overflow-hidden">
						<picture>
							<img src="https://temp.compsci88.com/cover/fallback/01J76XY7E9FNDZ1DBBM6PBJPFK.jpg" alt="One Piece cover" width="200" height="300" decoding="async">
						</picture>
					</div>
					<div class="w-full h-16 absolute bottom-0 flex flex-col items-center justify-center bg-linear-to-r from-black to-slate-900 opacity-90">
						<div class="truncate text-white text-center text-lg z-20 w-[90%]">One Piece</div>
					</div>
				</div>
			</article>
		</a>
	</section>
	<section class="hidden lg:block lg:w-[75%] xl:w-[80%]">
		<div class="text-lg font-semibold flex items-center gap-1">
			<span class="tooltip tooltip-bottom" data-tip="Official Translation">
				<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none"><path d="M8.5 12.5L10 14" /></svg>
			</span>
			<a href="https://weebcentral.com/series/01J76XY7E9FNDZ1DBBM6PBJPFK/One-Piece" class="line-clamp-1 link link-hover">One Piece</a>
		</div>
	</section>
</article>`

const secondSeriesCard = `
<article class="bg-base-300 flex gap-4 p-4">
	<section class="w-full lg:w-[25%] xl:w-[20%]">
		<a href="https://weebcentral.com/series/01J76XYD7E91K8QP6CY0Y53900/Blue-Lock">
			<article class="hidden lg:block w-full aspect-4/6  overflow-hidden">
				<picture>
					<img src="https://temp.compsci88.com/cover/fallback/01J76XYD7E91K8QP6CY0Y53900.jpg" alt="Blue Lock cover" width="400" height="600" decoding="async">
				</picture>
			</article>
			<article class="lg:hidden relative overflow-hidden">
				<div>
					<div class="w-full aspect-4/6 overflow-hidden">
						<picture>
							<img src="https://temp.compsci88.com/cover/fallback/01J76XYD7E91K8QP6CY0Y53900.jpg" alt="Blue Lock cover" width="200" height="300" decoding="async">
						</picture>
					</div>
					<div class="w-full h-16 absolute bottom-0 flex flex-col items-center justify-center bg-linear-to-r from-black to-slate-900 opacity-90">
						<div class="truncate text-white text-center text-lg z-20 w-[90%]">Blue Lock</div>
					</div>
				</div>
			</article>
		</a>
	</section>
	<section class="hidden lg:block lg:w-[75%] xl:w-[80%]">
		<div class="text-lg font-semibold flex items-center gap-1">
			<a href="https://weebcentral.com/series/01J76XYD7E91K8QP6CY0Y53900/Blue-Lock" class="line-clamp-1 link link-hover">
				Blue Lock
			</a>
		</div>
	</section>
</article>`

// chapterListFragment is newest-first, the order the endpoint returns.
const chapterListFragment = `
<div class="flex items-center" x-data="{ new_chapter: checkNewChapter('2026-11-06T13:39:21.278645Z') }" x-show="mark_chapters == 'null' || (mark_chapters == 'read' && 'False'.toLowerCase() === 'false') || (mark_chapters == 'unread' && 'False'.toLowerCase() === 'true')">
	<a href="/chapters/01M6RHBP1Y3A03BK0TZMBJ2YZN" class="hover:bg-base-300 flex-1 flex items-center p-2">
		<span class="me-2">
			<img src="/static/images/chapter-badge.svg" alt="" width="16" height="16" class="w-4 h-4" decoding="async">
		</span>
		<span class="grow flex items-center gap-2">
			<span class="">Chapter 1198</span>
			<span class="flex gap-1 items-center link-info" x-show="last_read_chapter === '01M6RHBP1Y3A03BK0TZMBJ2YZN'">
				<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 384 512"><path d="M215.7 499.2z" /></svg>
				<span class="hidden md:inline">Last Read</span>
			</span>
			<span x-show="new_chapter">
				<img src="/static/images/new-chapter.svg" alt="" width="20" height="20" class="w-5 h-5" decoding="async">
			</span>
		</span>
		<time class="text-datetime opacity-50" datetime="2026-11-06T13:39:21.278Z">2026-11-06T13:39:21.278645Z</time>
	</a>
	<input type="checkbox" value="01M6RHBP1Y3A03BK0TZMBJ2YZN" name="chapter_id" form="mark-as-read-form" class="checkbox ms-4" x-show="mark_chapters == 'read'">
</div>

<div class="flex items-center" x-data="{ new_chapter: checkNewChapter('2026-10-29T20:05:15.069022Z') }" x-show="mark_chapters == 'null'">
	<a href="/chapters/01M64M8H5WP6CF79EKV0NTH2BD" class="hover:bg-base-300 flex-1 flex items-center p-2">
		<span class="me-2">
			<img src="/static/images/chapter-badge-official.svg" alt="" width="16" height="16" class="w-4 h-4" decoding="async">
		</span>
		<span class="grow flex items-center gap-2">
			<span class="">Chapter 1197</span>
		</span>
		<time class="text-datetime opacity-50" datetime="2026-10-29T20:05:15.069Z">2026-10-29T20:05:15.069022Z</time>
	</a>
</div>`

// unavailableSeriesFragment is the whole body /full-chapter-list returned for a
// series whose chapters are gone: HTTP 200, no chapter rows, just the back-to-top
// link that every fragment on the site carries.
const unavailableSeriesFragment = `
<a href="#top" class="bg-secondary opacity-30 hover:opacity-70 fixed bottom-12 right-0 p-4" aria-label="back to top">
	<svg class="w-4 h-4" fill="currentColor" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
		<path d="M233.4 105.4c12.5-12.5 32.8-12.5 45.3 0l192 192c12.5-12.5 12.5 32.8 0 45.3s-32.8 12.5-45.3 0L256 173.3 86.6 342.6z" />
	</svg>
</a>`

const pageFragment = `
<img
	src="https://scans-hot.planeptune.us/manga/One-Piece/1198-001.png"
	class="max-w-full h-auto mx-auto"
	style="min-height: 50vh"
	alt="Page 1"
	decoding="async"
	loading="lazy">`

func newServer(t *testing.T, handler http.HandlerFunc) *weebcentral.Provider {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return weebcentral.NewWithBaseURL(ts.URL, ts.Client())
}

func TestSearchChaptersAndPages(t *testing.T) {
	const seriesID = "01J76XY7E9FNDZ1DBBM6PBJPFK"

	prov := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/data":
			_, _ = w.Write([]byte(seriesCard))
		case "/series/" + seriesID + "/full-chapter-list":
			_, _ = w.Write([]byte(chapterListFragment))
		default:
			_, _ = w.Write([]byte(pageFragment))
		}
	})

	results, err := prov.Search(context.Background(), "one piece")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "One Piece" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if results[0].ID != seriesID {
		t.Errorf("series id = %q, want %q", results[0].ID, seriesID)
	}

	chapters, err := prov.GetChapters(context.Background(), results[0])
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Chapter 1197" || chapters[1].Title != "Chapter 1198" {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}

	pages, err := prov.GetPages(context.Background(), chapters[1])
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://scans-hot.planeptune.us/manga/One-Piece/1198-001.png" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}

// TestGetChaptersCurrentChapterList pins the chapter row markup the site serves
// today: the label lives in the first span of span.grow, the chapter id is the
// last path segment of the href, and rows arrive newest-first.
func TestGetChaptersCurrentChapterList(t *testing.T) {
	const seriesID = "01J76XY7E9FNDZ1DBBM6PBJPFK"

	prov := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if want := "/series/" + seriesID + "/full-chapter-list"; r.URL.Path != want {
			t.Errorf("GetChapters requested %q, want %q", r.URL.Path, want)
		}
		_, _ = w.Write([]byte(chapterListFragment))
	})

	chapters, err := prov.GetChapters(context.Background(), domain.Series{ID: seriesID, Title: "One Piece"})
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2: %+v", len(chapters), chapters)
	}

	want := []struct {
		id     string
		title  string
		number float64
	}{
		{"01M64M8H5WP6CF79EKV0NTH2BD", "Chapter 1197", 1197},
		{"01M6RHBP1Y3A03BK0TZMBJ2YZN", "Chapter 1198", 1198},
	}
	for i, w := range want {
		got := chapters[i]
		if got.ID != w.id {
			t.Errorf("chapter %d id = %q, want %q", i, got.ID, w.id)
		}
		if got.Title != w.title {
			t.Errorf("chapter %d title = %q, want %q", i, got.Title, w.title)
		}
		if got.Number == nil || *got.Number != w.number {
			t.Errorf("chapter %d number = %v, want %v", i, got.Number, w.number)
		}
		if got.Index != i {
			t.Errorf("chapter %d index = %d: chapters are not in ascending reading order", i, got.Index)
		}
		if !strings.HasSuffix(got.URL, "/chapters/"+w.id) {
			t.Errorf("chapter %d url = %q, want it to end in /chapters/%s", i, got.URL, w.id)
		}
	}
}

// TestGetChaptersUnavailableSeries covers the case that made the provider look
// broken: the end of the catalogue holds series whose chapters were withdrawn, so
// their chapter endpoint answers 200 with an empty fragment. That is no chapters,
// not an error and not a silently-kept choke point.
func TestGetChaptersUnavailableSeries(t *testing.T) {
	prov := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(unavailableSeriesFragment))
	})

	chapters, err := prov.GetChapters(context.Background(), domain.Series{ID: "01J76XY7HGZVKWMF67BF06HJ1S", Title: "Junjou Romantica"})
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 0 {
		t.Errorf("got %d chapters for a series with none, want 0: %+v", len(chapters), chapters)
	}
}

// TestBrowseAsksForDescendingOrder pins the query Browse sends. /search/data
// ranks ascending unless order is named, and ascending starts at the bottom of
// the catalogue — in the live capture the first result was a series with no
// chapters and the runner-up had 8, against 1198 for the top of the descending
// list.
func TestBrowseAsksForDescendingOrder(t *testing.T) {
	var queries []url.Values

	prov := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/data" {
			t.Errorf("Browse requested %q, want /search/data", r.URL.Path)
		}
		queries = append(queries, r.URL.Query())
		_, _ = w.Write([]byte(seriesCard + secondSeriesCard))
	})

	for _, opts := range []provider.BrowseOptions{{}, {Sort: domain.SortRecent}} {
		series, err := prov.Browse(context.Background(), opts)
		if err != nil {
			t.Fatalf("Browse(%v) failed: %v", opts, err)
		}
		if len(series) != 2 || series[0].Title != "One Piece" || series[1].Title != "Blue Lock" {
			t.Errorf("Browse(%v) returned %+v", opts, series)
		}
	}

	if len(queries) != 2 {
		t.Fatalf("got %d browse requests, want 2", len(queries))
	}
	wantSorts := []string{"Popularity", "Latest Updates"}
	for i, want := range wantSorts {
		if got := queries[i].Get("sort"); got != want {
			t.Errorf("browse %d sort = %q, want %q", i, got, want)
		}
		if got := queries[i].Get("order"); got != "Descending" {
			t.Errorf("browse %d order = %q, want %q: without it the catalogue is walked from the least popular end", i, got, "Descending")
		}
	}
}
