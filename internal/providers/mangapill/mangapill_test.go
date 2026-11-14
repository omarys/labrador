package mangapill_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/mangapill"
)

// searchResultsHTML is a two-card slice of https://mangapill.com/search?q=solo+leveling,
// trimmed to the grids and cards. Each card links to its series twice — through
// the cover art and through the title — which is what made the old first-anchor
// parse return nothing.
const searchResultsHTML = `
<div class="my-3 grid justify-end gap-3 grid-cols-2 md:grid-cols-3 lg:grid-cols-5">
    <div>
        <a href="/manga/8136/solo-leveling-novel" class="relative block">
            <figure class="w-full h-52 overflow-hidden bg-card rounded-md">
                <img data-src="https://cdn.readdetectiveconan.com/file/mangapill/i/8136.jpeg" alt="Solo Leveling Novel Solo Leveling Novel" class="text-transparent lazy object-cover w-full h-full"/>
            </figure>
        </a>
        <div class="flex flex-col justify-end">
            <a href="/manga/8136/solo-leveling-novel" class="mb-2">
                <div class="mt-3 font-black leading-tight line-clamp-2">Solo Leveling Novel</div>
                <div class="line-clamp-2 text-xs text-secondary mt-1">Solo Leveling Official Light Novel</div>
            </a>
            <div class="flex flex-wrap gap-1 mt-1">
                <div class="text-xs leading-5 bg-purple-500 text-black rounded px-1">novel</div>
            </div>
        </div>
    </div>
    <div>
        <a href="/manga/8202/solo-leveling-ragnarok-novel" class="relative block">
            <figure class="w-full h-52 overflow-hidden bg-card rounded-md">
                <img data-src="https://cdn.readdetectiveconan.com/file/mangapill/i/8202.jpeg" alt="Solo Leveling: Ragnarok Novel" class="text-transparent lazy object-cover w-full h-full"/>
            </figure>
        </a>
        <div class="flex flex-col justify-end">
            <a href="/manga/8202/solo-leveling-ragnarok-novel" class="mb-2">
                <div class="mt-3 font-black leading-tight line-clamp-2">Solo Leveling: Ragnarok Novel</div>
            </a>
        </div>
    </div>
</div>`

func TestMangaPill_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("q") != "solo leveling" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}

		_, _ = w.Write([]byte(searchResultsHTML))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	seriesList, err := prov.Search(context.Background(), "solo leveling")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(seriesList) != 2 {
		t.Fatalf("expected 2 series, got %d: %+v", len(seriesList), seriesList)
	}
	if seriesList[0].Title != "Solo Leveling Novel" || seriesList[0].ID != "8136/solo-leveling-novel" {
		t.Errorf("unexpected series[0]: %+v", seriesList[0])
	}
	if seriesList[0].URL != ts.URL+"/manga/8136/solo-leveling-novel" {
		t.Errorf("unexpected series[0] URL: %s", seriesList[0].URL)
	}
	if seriesList[1].Title != "Solo Leveling: Ragnarok Novel" || seriesList[1].ID != "8202/solo-leveling-ragnarok-novel" {
		t.Errorf("unexpected series[1]: %+v", seriesList[1])
	}
}

func TestMangaPill_BrowseCatalogue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		// Without a filter the site renders only the advanced search form, so the
		// catalogue listing is asked for by format.
		if r.URL.Query().Get("type") != "manga" {
			t.Errorf("catalogue browse did not ask for a listing: %s", r.URL.RawQuery)
		}

		_, _ = w.Write([]byte(searchResultsHTML))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	seriesList, err := prov.Browse(context.Background(), provider.BrowseOptions{})
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}

	if len(seriesList) != 2 {
		t.Fatalf("expected 2 series, got %d: %+v", len(seriesList), seriesList)
	}
}

// recentChaptersHTML is a one-card slice of https://mangapill.com/chapters?page=1.
// Here the card's first link is the chapter, and the series link carries the
// bold title, so the card cannot be read from its first anchor.
const recentChaptersHTML = `
<div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3">
    <div>
        <a href="/chapters/8970-10019000/bang-dream-chapter-19" class="relative block">
            <figure class="w-full h-40 overflow-hidden bg-card rounded-md">
                <img data-src="https://cdn.readdetectiveconan.com/file/mangapill/i/8970.jpeg" alt="BanG Dream! Chapter 19" class="text-transparent lazy object-cover w-full h-full"/>
            </figure>
        </a>
        <div class="px-1">
            <a href="/chapters/8970-10019000/bang-dream-chapter-19">
                <div class="mt-3 text-lg font-black leading-tight">#19</div>
            </a>
            <a href="/manga/8970/bang-dream" class="mt-1.5 leading-tight text-secondary">
                <div class="line-clamp-2 text-sm font-bold">BanG Dream! It&#39;s MyGO!!!!!</div>
                <div class="line-clamp-2 text-xs mt-1">BanG Dream! Swaying in the Rain</div>
            </a>
        </div>
    </div>
</div>`

func TestMangaPill_BrowseRecent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chapters" {
			http.NotFound(w, r)
			return
		}

		_, _ = w.Write([]byte(recentChaptersHTML))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	opts := provider.BrowseOptions{}
	opts.Sort = domain.SortRecent

	seriesList, err := prov.Browse(context.Background(), opts)
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}

	if len(seriesList) != 1 {
		t.Fatalf("expected 1 series, got %d: %+v", len(seriesList), seriesList)
	}
	if seriesList[0].ID != "8970/bang-dream" {
		t.Errorf("unexpected series ID: %+v", seriesList[0])
	}
	if seriesList[0].Title != "BanG Dream! It's MyGO!!!!!" {
		t.Errorf("unexpected series title: %q", seriesList[0].Title)
	}
}

// genreFilterHTML is the genre block of the advanced search form: one checkbox
// per genre, whose value is the genre name the search accepts.
const genreFilterHTML = `
<div class="form-group">
    <label>Choose Genres</label>
    <div class="grid gap-1 grid-cols-2 md:grid-cols-5 lg:grid-cols-8">
        <div class="m-1">
            <label class="text-sm">
                <input
                    type="checkbox"
                    onchange="this.form.submit()" name="genre"
                    value="Action"
                     />
                Action
            </label>
        </div>
        <div class="m-1">
            <label class="text-sm">
                <input
                    type="checkbox"
                    onchange="this.form.submit()" name="genre"
                    value="Gender Bender"
                     />
                Gender Bender
            </label>
        </div>
    </div>
</div>
`

func TestMangaPill_GetTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}

		_, _ = w.Write([]byte(genreFilterHTML))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	tags, err := prov.GetTags(context.Background())
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}

	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %+v", len(tags), tags)
	}
	if tags[0].ID != "Action" || tags[0].Label != "Action" {
		t.Errorf("unexpected tag[0]: %+v", tags[0])
	}
	if tags[1].ID != "Gender Bender" || tags[1].Label != "Gender Bender" {
		t.Errorf("unexpected tag[1]: %+v", tags[1])
	}
}

func TestMangaPill_BrowseTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("genre"); got != "Gender Bender" {
			t.Errorf("genre browse sent genre=%q", got)
		}

		_, _ = w.Write([]byte(searchResultsHTML))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	opts := provider.BrowseOptions{}
	opts.Tag = &domain.Tag{ID: "Gender Bender", Label: "Gender Bender"}

	seriesList, err := prov.Browse(context.Background(), opts)
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}
	if len(seriesList) != 2 {
		t.Fatalf("expected 2 series, got %d: %+v", len(seriesList), seriesList)
	}
}

func TestMangaPill_GetChapters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
    <div id="chapters" class="p-3">
        <div data-filter-list class="my-3 grid grid-cols-1 md:grid-cols-3 lg:grid-cols-6">
            <a class="border border-border p-1 hover:bg-brand hover:text-white" href="/chapters/2-11198000/one-piece-chapter-1198" title=" Chapter 1198">Chapter 1198</a>
            <a class="border border-border p-1 hover:bg-brand hover:text-white" href="/chapters/2-11197000/one-piece-chapter-1197" title=" Chapter 1197">Chapter 1197</a>
        </div>
    </div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	series := domain.Series{ID: "2/one-piece", URL: ts.URL + "/manga/2/one-piece"}

	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title != "Chapter 1198" || chapters[0].URL != ts.URL+"/chapters/2-11198000/one-piece-chapter-1198" {
		t.Errorf("unexpected chapter[0]: %+v", chapters[0])
	}
	if chapters[0].ID != "2-11198000/one-piece-chapter-1198" {
		t.Errorf("unexpected chapter[0] ID: %q", chapters[0].ID)
	}
	if chapters[0].Number == nil || *chapters[0].Number != 1198 {
		t.Errorf("unexpected chapter[0] number: %v", chapters[0].Number)
	}
	if chapters[1].Index != 1 {
		t.Errorf("unexpected chapter[1] index: %d", chapters[1].Index)
	}
}

func TestManagaPill_GetPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
    <chapter-page>
      <div class="relative bg-card flex justify-center items-center">
        <picture>
          <img class="js-page" data-src="https://cdn.example.com/1.jpeg" alt="One Piece Chapter 1198 Page 1" loading="lazy" width="784" height="1145"/>
        </picture>
      </div>
    </chapter-page>
    <chapter-page>
      <div class="relative bg-card flex justify-center items-center">
        <picture>
          <img class="js-page" data-src="https://cdn.example.com/2.jpeg" alt="One Piece Chapter 1198 Page 2" loading="lazy" width="784" height="1145"/>
        </picture>
      </div>
    </chapter-page>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	chapter := domain.Chapter{ID: "2-11198000/one-piece-chapter-1198", URL: ts.URL + "/chapters/2-11198000/one-piece-chapter-1198"}

	pages, err := prov.GetPages(context.Background(), chapter)
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
	if pages[0].URL != "https://cdn.example.com/1.jpeg" || pages[0].Index != 0 {
		t.Errorf("unexpected page[0]: %+v", pages[0])
	}
	if pages[1].URL != "https://cdn.example.com/2.jpeg" || pages[1].Index != 1 {
		t.Errorf("unexpected page[1]: %+v", pages[1])
	}
	// The image host answers 403 unless the request is referred by mangapill.
	if got := pages[0].Headers["Referer"]; len(got) != 1 || got[0] != ts.URL {
		t.Errorf("unexpected page[0] referer: %v", got)
	}
}

func TestMangaPill_MatchesURL(t *testing.T) {
	prov := mangapill.New(nil)

	if !prov.MatchesURL("https://mangapill.com/manga/2/solo-leveling") {
		t.Error("expected URL to match mangapill.com")
	}
	if !prov.MatchesURL("https://MANGAPILL.COM/ck/123") {
		t.Error("expected case-insensitive URL match")
	}
	if prov.MatchesURL("https://mangadex.org/title/123") {
		t.Error("expected other domains not to match")
	}
}
