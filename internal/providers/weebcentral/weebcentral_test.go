package weebcentral_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/providers/weebcentral"
)

func TestWeebCentral_Search_Chapters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/data" {
			html := `
			<article class="bg-base-300">
				<a class="line-clamp-1" href="/series/01JJ1234/solo-leveling">Solo Leveling</a>
			</article>`
			_, _ = w.Write([]byte(html))
			return
		}

		if r.URL.Path == "/series/01JJ1234/solo-leveling/full-chapter-list" {
			html := `
			<div>
				<a href="/chapters/01JJ9999/chapter-2">Chapter 2</a>
				<a href="/chapters/01JJ8888/chapter-1">Chapter 1</a>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		// Images
		html := `
		<section>
			<img src="https://cdn.example.com/wc/p1.jpg" />
		</section>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := weebcentral.NewWithBaseURL(ts.URL, ts.Client())

	results, err := prov.Search(context.Background(), "solo")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Solo Leveling" {
		t.Errorf("unexpected results: %+v", results)
	}

	chapters, err := prov.GetChapters(context.Background(), results[0])
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Chapter 1" {
		t.Errorf("unexpected chapters: %+v", chapters)
	}

	pages, err := prov.GetPages(context.Background(), chapters[0])
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://cdn.example.com/wc/p1.jpg" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}
