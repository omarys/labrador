package manganelo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarys/labrador/internal/providers/manganelo"
)

func TestManganelo_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<div class="search-story-item">
			<a class="item-title" href="/manga-bb456">Naruto</a>
		</div>`
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	prov := manganelo.NewWithBaseURL(ts.URL, ts.Client())
	results, err := prov.Search(context.Background(), "naruto")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Naruto" {
		t.Errorf("unexpected results: %+v", results)
	}
}
