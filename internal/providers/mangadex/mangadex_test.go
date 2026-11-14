package mangadex_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
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
						"id": "official-uuid",
						"attributes": {
							"title": "Official release on another site",
							"chapter": "1",
							"pages": 0,
							"externalUrl": "https://www.tappytoon.com/en/chapters/95"
						}
					},
					{
						"id": "chap-uuid-1",
						"attributes": {
							"title": "The End of the Adventure",
							"chapter": "1",
							"pages": 2
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
	// The external chapter is not hosted by MangaDex and cannot be downloaded, so
	// it must not appear in the list at all — least of all first, where it would
	// produce an empty archive.
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

// TestMangaDex_SendsAnIdentifyingUserAgent guards the bot-challenge fix: the API
// answers a browser User-Agent with a redirect to a challenge page, so the
// provider must override the stealth client's Chrome UA on every API call.
func TestMangaDex_SendsAnIdentifyingUserAgent(t *testing.T) {
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	p := mangadex.NewWithBaseURL(ts.URL, ts.Client())
	ctx := context.Background()

	_, _ = p.Search(ctx, "frieren")
	_, _ = p.Browse(ctx, provider.BrowseOptions{})
	_, _ = p.GetChapters(ctx, domain.Series{ID: "uuid-1234"})
	_, _ = p.GetTags(ctx)

	if len(seen) == 0 {
		t.Fatal("no requests were made")
	}
	for i, ua := range seen {
		if ua == "" {
			t.Errorf("request %d sent no User-Agent", i)
		}
		if strings.HasPrefix(ua, "Mozilla/") {
			t.Errorf("request %d sent a browser User-Agent %q, which trips the bot challenge", i, ua)
		}
		if !strings.Contains(ua, "Labrador") {
			t.Errorf("request %d sent %q, expected it to name Labrador", i, ua)
		}
	}
}
