package readcomicsonline_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/providers/readcomicsonline"
)

// searchShellHTML is a trimmed card from https://readcomicsonline.lol/search?q=...
// The search page is a client-rendered app shell: every result slot is a
// placeholder like this one and no series link or cover alt text is in the
// markup, so a provider that scrapes this page finds nothing.
const searchShellHTML = `<div class="animate-pulse" aria-hidden="true"><div class="aspect-[2/3] bg-[var(--bg-skeleton)] rounded-[var(--radius)]"></div><div class="mt-3 px-1 space-y-2"><div class="h-4 bg-[var(--bg-skeleton)] rounded w-3/4"></div><div class="flex justify-between"><div class="h-3 bg-[var(--bg-skeleton)] rounded w-16"></div><div class="h-3 bg-[var(--bg-skeleton)] rounded w-12"></div></div></div></div>`

// searchJSON is https://readcomicsonline.lol/api/search?q=batman trimmed to two
// entries, keeping the exact field set the site's own search page reads.
const searchJSON = `{"results":[{"id":"cmkmrp35l00j7ltiz0gt0s6f5","title":"Absolute Batman","slug":"Absolute-Batman","cover":"https://cdn.readcomicsonline.lol/covers/Absolute-Batman/1.webp","genres":"Action, Superhero"},{"id":"cmkmri74p0000ltizg1obt5ui","title":"Batman (2025)","slug":"Batman-2025","cover":"https://cdn.readcomicsonline.lol/covers/Batman-2025/1.webp","genres":"Action, Superhero"}]}`

// chaptersHTML is the issue list of https://readcomicsonline.lol/comic/Absolute-Batman,
// trimmed to the "Start Reading" button and the anchors for issues 2, 1 and 1.1.
// Each issue anchor carries its label and date in spans next to the cover image;
// the hover classes and data URIs of the original markup are dropped.
const chaptersHTML = `<div class="flex flex-col divide-y divide-[var(--border)]">
<a data-smartlink="true" class="flex items-center justify-center gap-2 w-full bg-[var(--accent)]" href="/comic/Absolute-Batman/1"><svg aria-hidden="true" class="lucide lucide-book-open w-5 h-5 shrink-0"><path d="M12 7v14"></path></svg><span class="line-clamp-1">Start Reading</span></a>
<a data-smartlink="strong" class="flex flex-1 items-center justify-between min-w-0" href="/comic/Absolute-Batman/2"><div class="flex items-center gap-2.5 min-w-0"><div class="relative w-[36px] h-[47px] shrink-0 overflow-hidden"><img alt="Absolute Batman Issue #2 cover" loading="lazy" src="https://cdn.readcomicsonline.lol/covers/Absolute-Batman/2.webp"/></div><span class="text-[14px] truncate">Absolute Batman Issue #2</span></div><div class="flex items-center gap-2 shrink-0 ml-3"><span class="text-[14px] whitespace-nowrap">2024-11-13</span></div></a>
<a data-smartlink="strong" class="flex flex-1 items-center justify-between min-w-0" href="/comic/Absolute-Batman/1"><div class="flex items-center gap-2.5 min-w-0"><div class="relative w-[36px] h-[47px] shrink-0 overflow-hidden"><img alt="Absolute Batman Issue #1 cover" loading="lazy" src="https://cdn.readcomicsonline.lol/covers/Absolute-Batman/1.webp"/></div><span class="text-[14px] truncate">Absolute Batman Issue #1</span></div><div class="flex items-center gap-2 shrink-0 ml-3"><span class="text-[14px] whitespace-nowrap">2024-10-08</span></div></a>
<a data-smartlink="strong" class="flex flex-1 items-center justify-between min-w-0" href="/comic/Absolute-Batman/1.1"><div class="flex items-center gap-2.5 min-w-0"><div class="relative w-[36px] h-[47px] shrink-0 overflow-hidden"><img alt="Absolute Batman #1 Noir Edition cover" loading="lazy" src="https://cdn.readcomicsonline.lol/covers/Absolute-Batman/1.1.webp"/></div><span class="text-[14px] truncate">Absolute Batman #1 Noir Edition</span></div><div class="flex items-center gap-2 shrink-0 ml-3"><span class="text-[14px] whitespace-nowrap">2024-12-12</span></div></a>
</div>`

// readerHTML is https://readcomicsonline.lol/comic/Absolute-Batman/1 trimmed to
// the metadata block that references the issue cover and the page list that the
// provider walks for images.
const readerHTML = `<script>self.__next_f.push([1,"5f:T4d1,{\"@context\":\"https://schema.org\",\"@type\":\"ComicIssue\",\"@id\":\"https://readcomicsonline.lol/comic/Absolute-Batman/1\",\"name\":\"Absolute Batman Issue #1\",\"image\":\"https://cdn.readcomicsonline.lol/covers/Absolute-Batman/1.webp\"}"])</script>
<script>self.__next_f.push([1,"62:[\"$\",\"$L65\",null,{\"pages\":[{\"id\":\"cmn7hysac00snb98xwmu58klz\",\"issueId\":\"cmkmrpm5p00m3ltizo6qqwfgv\",\"pageNumber\":1,\"url\":\"https://cdn.readcomicsonline.lol/pages/Absolute-Batman/1/p001.webp\"},{\"id\":\"cmn7hyt0800spb98xbvbut234\",\"issueId\":\"cmkmrpm5p00m3ltizo6qqwfgv\",\"pageNumber\":2,\"url\":\"https://cdn.readcomicsonline.lol/pages/Absolute-Batman/1/p002.webp\"}]}"])</script>`

func TestSearch(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []domain.Series
	}{
		{
			name: "site returns matches",
			body: searchJSON,
			want: []domain.Series{
				{ID: "Absolute-Batman", Title: "Absolute Batman", URL: "/comic/Absolute-Batman"},
				{ID: "Batman-2025", Title: "Batman (2025)", URL: "/comic/Batman-2025"},
			},
		},
		{
			name: "site has no matching series",
			body: `{"results":[]}`,
			want: nil,
		},
		{
			name: "entry without a slug cannot be addressed and is skipped",
			body: `{"results":[{"id":"c1","title":"Unaddressable"},{"id":"cmkmrp35l00j7ltiz0gt0s6f5","title":"Absolute Batman","slug":"Absolute-Batman","cover":"https://cdn.readcomicsonline.lol/covers/Absolute-Batman/1.webp","genres":"Action, Superhero"},{"id":"cmkmri74p0000ltizg1obt5ui","title":"Batman (2025)","slug":"Batman-2025","cover":"https://cdn.readcomicsonline.lol/covers/Batman-2025/1.webp","genres":"Action, Superhero"}]}`,
			want: []domain.Series{
				{ID: "Absolute-Batman", Title: "Absolute Batman", URL: "/comic/Absolute-Batman"},
				{ID: "Batman-2025", Title: "Batman (2025)", URL: "/comic/Batman-2025"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotQuery string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/search" {
					// The rendered search page: placeholder cards, nothing to scrape.
					_, _ = w.Write([]byte(searchShellHTML))
					return
				}
				gotPath = r.URL.Path
				gotQuery = r.URL.Query().Get("q")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			prov := readcomicsonline.NewWithBaseURL(ts.URL, ts.Client())
			got, err := prov.Search(context.Background(), "batman")
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}
			if gotPath != "/api/search" {
				t.Fatalf("Search asked for %q: the site renders results client side, only /api/search carries them", gotPath)
			}
			if gotQuery != "batman" {
				t.Errorf("Search sent q=%q, want %q", gotQuery, "batman")
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d series, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, want := range tt.want {
				want.URL = ts.URL + want.URL
				gotSeries := got[i]
				if gotSeries.ID != want.ID || gotSeries.Title != want.Title || gotSeries.URL != want.URL {
					t.Errorf("series %d = %+v, want %+v", i, gotSeries, want)
				}
			}
		})
	}
}

func TestGetChapters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(chaptersHTML))
	}))
	defer ts.Close()

	prov := readcomicsonline.NewWithBaseURL(ts.URL, ts.Client())
	series := domain.Series{ID: "Absolute-Batman", Title: "Absolute Batman", URL: ts.URL + "/comic/Absolute-Batman"}

	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}

	want := []struct {
		title string
		id    string
		url   string
	}{
		{"Absolute Batman Issue #1", "1", "/comic/Absolute-Batman/1"},
		{"Absolute Batman #1 Noir Edition", "1.1", "/comic/Absolute-Batman/1.1"},
		{"Absolute Batman Issue #2", "2", "/comic/Absolute-Batman/2"},
	}
	if len(chapters) != len(want) {
		t.Fatalf("got %d chapters (\"Start Reading\" must be skipped), want %d: %+v", len(chapters), len(want), chapters)
	}
	for i, w := range want {
		got := chapters[i]
		if got.Title != w.title || got.ID != w.id || got.URL != ts.URL+w.url {
			t.Errorf("chapter %d = {%q %q %q}, want {%q %q %q}", i, got.Title, got.ID, got.URL, w.title, w.id, ts.URL+w.url)
		}
		if got.Index != i {
			t.Errorf("chapter %d is indexed %d", i, got.Index)
		}
	}
}

func TestGetPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(readerHTML))
	}))
	defer ts.Close()

	prov := readcomicsonline.NewWithBaseURL(ts.URL, ts.Client())
	chapter := domain.Chapter{ID: "1", SeriesID: "Absolute-Batman", Title: "Absolute Batman Issue #1", URL: ts.URL + "/comic/Absolute-Batman/1"}

	pages, err := prov.GetPages(context.Background(), chapter)
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}

	want := []string{
		"https://cdn.readcomicsonline.lol/pages/Absolute-Batman/1/p001.webp",
		"https://cdn.readcomicsonline.lol/pages/Absolute-Batman/1/p002.webp",
	}
	if len(pages) != len(want) {
		t.Fatalf("got %d pages (the issue cover is not a page), want %d: %+v", len(pages), len(want), pages)
	}
	for i, url := range want {
		if pages[i].URL != url || pages[i].Index != i {
			t.Errorf("page %d = %+v, want {%s %d}", i, pages[i], url, i)
		}
	}
}
