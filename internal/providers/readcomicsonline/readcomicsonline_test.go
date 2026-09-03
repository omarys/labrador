package readcomicsonline_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/providers/readcomicsonline"
)

func TestReadComicsOnline_Search_And_Chapters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			html := `
			<div>
				<a href="/comic/batman-2025">
					<img alt="Batman (2025) comic cover" src="/thumb.jpg" />
				</a>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		if r.URL.Path == "/comic/batman-2025" {
			html := `
			<div>
				<a href="/comic/batman-2025/1">Start Reading</a>
				<a href="/comic/batman-2025/1">Batman (2025) Issue #1 2025-01-01</a>
				<a href="/comic/batman-2025/2">Batman (2025) Issue #2 2025-02-01</a>
			</div>`
			_, _ = w.Write([]byte(html))
			return
		}

		// Reader page
		html := `
		<div>
			<script>
				const payload = "https://cdn.readcomics.online/pages/batman/p001.webp https://cdn.readcomics.online/pages/batman/p002.webp";
			</script>
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := readcomicsonline.NewWithBaseURL(ts.URL, ts.Client())

	results, err := prov.Search(context.Background(), "batman")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Batman (2025)" {
		t.Errorf("unexpected results: %+v", results)
	}

	chapters, err := prov.GetChapters(context.Background(), domain.Series{URL: ts.URL + "/comic/batman-2025"})
	if err != nil {
		t.Fatalf("GetChapters failed: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters (Start Reading skipped), got %d", len(chapters))
	}
	if chapters[0].Title != "Batman (2025) Issue #1" {
		t.Errorf("unexpected title: %s", chapters[0].Title)
	}

	pages, err := prov.GetPages(context.Background(), chapters[0])
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}
	if len(pages) != 2 || pages[0].URL != "https://cdn.readcomics.online/pages/batman/p001.webp" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}
