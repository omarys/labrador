package mangapill_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/providers/mangapill"
)

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

		html := `
    <div class="grid">
      <div class="border-border">
        <a href="/manga/2/solo-leveling">
          <div class="font-bold">Solo Leveling</div>
        </a>
      </div>
      <div class="border-border">
        <a href="/manga/3/solo-leveling-ragnarok">
          <div class="font-bold">Solor Leveling: Ragnarok</div>
        </a>
      </div>
    </div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	seriesList, err := prov.Search(context.Background(), "solo leveling")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(seriesList) != 2 {
		t.Fatalf("expected 2 series, dot %d", len(seriesList))
	}
	if seriesList[0].Title != "Solo Leveling" || seriesList[0].ID != "2/solo-leveling" {
		t.Errorf("unexpected series[0]: %+v", seriesList[0])
	}
	if seriesList[0].URL != ts.URL+"/manga/2/solo-leveling" {
		t.Errorf("unexpected series[0] URL: %s", seriesList[0].URL)
	}
}

func TestMangaPill_GetChapters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
    <div id="chapters">
      <a href="/ck/2-10001000/solo-leveling-chapter-1">Chapter 1</a>
      <a href="/ck/2-10002000/solo-leveling-chapter-2">Chapter 2</a>
    </div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	series := domain.Series{ID: "2/solo-leveling", URL: ts.URL + "/manga/2/solo-leveling"}

	chapters, err := prov.GetChapters(context.Background(), series)
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title != "Chapter 1" || chapters[0].URL != ts.URL+"/ck/2-10001000/solo-leveling-chapter-1" {
		t.Errorf("unexpected chapter[0]: %+v", chapters[0])
	}
}

func TestManagaPill_GetPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
    <picture>
      <img data-src="https://cdn.example.com/page1.jpg" />
    </picture>
    <picture>
      <img data-src="https://cdn.example.com/page2.png" />
    </picture>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := mangapill.NewWithBaseURL(ts.URL, ts.Client())
	chapter := domain.Chapter{ID: "ch-1", URL: ts.URL + "/ck/ch-1"}

	pages, err := prov.GetPages(context.Background(), chapter)
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
	if pages[0].URL != "https://cdn.example.com/page1.jpg" || pages[0].Index != 0 {
		t.Errorf("unexpected page[0]: %+v", pages[0])
	}
	if pages[1].URL != "https://cdn.example.com/page2.png" || pages[1].Index != 1 {
		t.Errorf("unexpected page[1]: %+v", pages[1])
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
