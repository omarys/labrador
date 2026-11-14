package mangakatana_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/mangakatana"
)

// TestMangaKatana_GetTags uses a fragment of the live /genres filter block,
// trimmed to three genres (one of them labelled with the extra "hot" class),
// wrapped in the grid markup the site nests items in. The trailing
// .genres.uk-hidden-small block is the per-series genre nav that sits next to
// the filter and must not be mistaken for the tag list.
func TestMangaKatana_GetTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<div class="genres">
			<div class="uk-grid uk-grid-medium">
				<div class="uk-width-medium-1-4 uk-width-1-2"><div class="item">
					<span class="wrap_chk"><input type="checkbox" name="include_genre_chk" id="in-44" data-op="ex-44" value="4-koma"><label class="include" for="in-44"></label></span> <span class="wrap_chk"><input type="checkbox" name="exclude_genre_chk" id="ex-44" data-op="in-44" value="4-koma"><label class="exclude" for="ex-44"></label></span> <span class="name">4 koma</span>
				</div></div><div class="uk-width-medium-1-4 uk-width-1-2"><div class="item">
					<span class="wrap_chk"><input type="checkbox" name="include_genre_chk" id="in-14" data-op="ex-14" value="action"><label class="include" for="in-14"></label></span> <span class="wrap_chk"><input type="checkbox" name="exclude_genre_chk" id="ex-14" data-op="in-14" value="action"><label class="exclude" for="ex-14"></label></span> <span class="name">Action</span>
				</div></div><div class="uk-width-medium-1-4 uk-width-1-2"><div class="item">
					<span class="wrap_chk"><input type="checkbox" name="include_genre_chk" id="in-34" data-op="ex-34" value="erotica"><label class="include" for="in-34"></label></span> <span class="wrap_chk"><input type="checkbox" name="exclude_genre_chk" id="ex-34" data-op="in-34" value="erotica"><label class="exclude" for="ex-34"></label></span> <span class="name hot">Erotica</span>
				</div></div>
			</div>
			<div class="item"><button type="submit" class="filter_bt transition">Apply</button></div>
		</div>
		<div class="genres uk-hidden-small">
			<a href="https://mangakatana.com/genre/action">Action</a><a href="https://mangakatana.com/genre/erotica">Erotica</a>		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangakatana.NewWithBaseURL(ts.URL, ts.Client())
	tags, err := prov.GetTags(context.Background())
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}

	want := []domain.Tag{
		{ID: "4-koma", Label: "4 koma"},
		{ID: "action", Label: "Action"},
		{ID: "erotica", Label: "Erotica"},
	}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %d: %+v", len(want), len(tags), tags)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Errorf("tag %d: expected %+v, got %+v", i, want[i], tags[i])
		}
	}
}

// TestMangaKatana_Browse_Tag checks a returned tag ID reaches the genre listing
// the site filters by, so a tag the user picks narrows Browse rather than
// bouncing off a dead URL.
func TestMangaKatana_Browse_Tag(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		html := `
		<div id="book_list">
			<div class="item">
				<h3 class="title"><a href="/manga/chainsaw-man.12345">Chainsaw Man</a></h3>
			</div>
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangakatana.NewWithBaseURL(ts.URL, ts.Client())
	results, err := prov.Browse(context.Background(), provider.BrowseOptions{
		Tag: &domain.Tag{ID: "action", Label: "Action"},
	})
	if err != nil {
		t.Fatalf("Browse with tag failed: %v", err)
	}
	if gotPath != "/genre/action/page/1" {
		t.Errorf("expected Browse to request /genre/action/page/1, got %q", gotPath)
	}
	if len(results) != 1 || results[0].ID != "12345" {
		t.Errorf("unexpected series: %+v", results)
	}
}

func TestMangaKatana_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<div id="book_list">
			<div class="item">
				<h3 class="title"><a href="/manga/chainsaw-man.12345">Chainsaw Man</a></h3>
			</div>
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangakatana.NewWithBaseURL(ts.URL, ts.Client())
	results, err := prov.Search(context.Background(), "chainsaw")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Chainsaw Man" || results[0].ID != "12345" {
		t.Errorf("unexpected series: %+v", results[0])
	}
}

func TestMangaKatana_GetChapters_And_Pages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manga/sample.123" {
			html := `
			<div class="chapters">
				<table>
					<tr><td class="chapter"><a href="/manga/sample.123/c2">Chapter 2</a></td></tr>
					<tr><td class="chapter"><a href="/manga/sample.123/c1">Chapter 1</a></td></tr>
				</table>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		// Chapter reader page with thzq JS array
		html := `
		<html>
		<script>
		var thzq = ['https://cdn.example.com/p1.jpg','https://cdn.example.com/p2.jpg'];
		</script>
		</html>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangakatana.NewWithBaseURL(ts.URL, ts.Client())
	chapters, err := prov.GetChapters(context.Background(), domain.Series{URL: ts.URL + "/manga/sample.123"})
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	// Reversal check: Chapter 1 must be first
	if chapters[0].Title != "Chapter 1" {
		t.Errorf("expected Chapter 1 first after sort, got %s", chapters[0].Title)
	}

	pages, err := prov.GetPages(context.Background(), chapters[0])
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}
	if len(pages) != 2 || pages[0].URL != "https://cdn.example.com/p1.jpg" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}
