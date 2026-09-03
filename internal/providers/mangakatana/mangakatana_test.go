package mangakatana_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/providers/mangakatana"
)

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
