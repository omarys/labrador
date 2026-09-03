package manganato_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/providers/manganato"
)

func TestManganato_Search_And_Chapters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/story/one_piece" {
			html := `
			<div class="search-story-item">
				<a class="item-title" href="https://manganato.com/manga-aa123">One Piece</a>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		if r.URL.Path == "/manga-aa123" {
			html := `
			<ul class="row-content-chapter">
				<li class="a-h"><a class="chapter-name" href="/manga-aa123/c2">Chapter 2</a></li>
				<li class="a-h"><a class="chapter-name" href="/manga-aa123/c1">Chapter 1</a></li>
			</ul>`
			_, _ = w.Write([]byte(html))
			return
		}

		// Pages
		html := `
		<div class="container-chapter-reader">
			<img src="https://cdn.example.com/nato/p1.jpg" />
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := manganato.NewWithBaseURL(ts.URL, ts.Client())

	results, err := prov.Search(context.Background(), "one piece")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "One Piece" {
		t.Errorf("unexpected results: %+v", results)
	}

	chapters, err := prov.GetChapters(context.Background(), domain.Series{URL: ts.URL + "/manga-aa123"})
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Chapter 1" {
		t.Errorf("expected chapter 1 first after reversal, got %+v", chapters)
	}

	pages, err := prov.GetPages(context.Background(), chapters[0])
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://cdn.example.com/nato/p1.jpg" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}
