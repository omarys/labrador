package mangadex_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/providers/mangadex"
)

func TestMangaDex_Search_Chapters_Pages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/manga" {
			json := `{
				"data": [
					{
						"id": "uuid-1234",
						"attributes": {
							"title": {"en": "Frieren"}
						}
					}
				]
			}`
			_, _ = w.Write([]byte(json))
			return
		}

		if r.URL.Path == "/manga/uuid-1234/feed" {
			json := `{
				"data": [
					{
						"id": "chap-uuid-1",
						"attributes": {
							"title": "The End of the Adventure",
							"chapter": "1"
						}
					}
				]
			}`
			_, _ = w.Write([]byte(json))
			return
		}

		if r.URL.Path == "/at-home/server/chap-uuid-1" {
			json := `{
				"baseUrl": "https://uploads.mangadex.org",
				"chapter": {
					"hash": "hash123",
					"data": ["001.png", "002.png"]
				}
			}`
			_, _ = w.Write([]byte(json))
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	prov := mangadex.NewWithBaseURL(ts.URL, ts.Client())

	results, err := prov.Search(context.Background(), "frieren")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Frieren" {
		t.Errorf("unexpected search results: %+v", results)
	}

	chapters, err := prov.GetChapters(context.Background(), results[0])
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 1 || chapters[0].Title != "The End of the Adventure" {
		t.Errorf("unexpected chapters: %+v", chapters)
	}

	pages, err := prov.GetPages(context.Background(), chapters[0])
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}
	if len(pages) != 2 || pages[0].URL != "https://uploads.mangadex.org/data/hash123/001.png" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}
