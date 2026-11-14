//go:build live

// Package providers_test holds the live smoke checks. They talk to each
// provider's real site and assert the shape of what comes back, so that a change
// upstream — renamed selectors, a moved API, a new image host — fails here
// instead of quietly breaking a download.
//
// They are excluded from the ordinary test run: they need the network, they take
// minutes, and a failure may mean the site is down rather than that our code is
// wrong. Run them deliberately:
//
//	go test -tags live ./internal/providers/                       # every provider
//	go test -tags live ./internal/providers/ -run 'TestLive/novelping'   # one
//	LABRADOR_LIVE=mangadex,novelarchive go test -tags live ./internal/providers/
//
// LABRADOR_LIVE_QUERY sets the search term used for providers that cannot browse.
package providers_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/httpclient"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers"
)

// smokeQueries are search terms that suit a provider whose catalogue is not
// manga, so the shared default is not mistaken for a broken search: a Western
// comic site answering "solo leveling" with nothing is correct behaviour.
var smokeQueries = map[string]string{
	"readcomicsonline": "batman",
}

// smokeQuery picks the term to search with, unless the caller set one
// explicitly — an explicit LABRADOR_LIVE_QUERY always wins.
func smokeQuery(providerID, explicit string) string {
	if os.Getenv("LABRADOR_LIVE_QUERY") != "" {
		return explicit
	}
	if q, ok := smokeQueries[providerID]; ok {
		return q
	}
	return explicit
}

// liveProviderBudget caps one provider's whole walk so a hung site cannot stall
// the run.
const liveProviderBudget = 3 * time.Minute

func TestLive(t *testing.T) {
	only := envList("LABRADOR_LIVE")
	query := os.Getenv("LABRADOR_LIVE_QUERY")
	if query == "" {
		query = "solo leveling"
	}
	client := httpclient.NewStealthClient(45 * time.Second)
	reg := provider.NewRegistry()
	providers.RegisterAll(reg, client)

	all := reg.All()
	if len(all) == 0 {
		t.Fatal("no providers are registered")
	}

	ran := 0
	for _, p := range all {
		if len(only) > 0 && !slices.Contains(only, p.ID()) {
			continue
		}
		ran++
		t.Run(p.ID(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), liveProviderBudget)
			defer cancel()
			walkProvider(t, ctx, client, p, smokeQuery(p.ID(), query))
		})
	}

	if ran == 0 {
		t.Fatalf("LABRADOR_LIVE=%q matched no provider; registered: %s",
			strings.Join(only, ","), strings.Join(ids(all), ", "))
	}
}

// liveSeriesTries bounds how many catalogue entries are asked for chapters.
// A title whose chapters are all hosted elsewhere (MangaDex points licensed
// manga at the publisher's reader) legitimately has none, so the first entry is
// not always the right one to probe.
const liveSeriesTries = 5

// walkProvider follows the path a real download takes — catalogue, chapters, one
// chapter's payload — and reports how much of each it saw, so a run that
// silently shrinks (three chapters where there were three hundred, no tags) is
// readable without waiting for a failure.
func walkProvider(t *testing.T, ctx context.Context, client *http.Client, p provider.Provider, query string) {
	t.Helper()

	caps := p.Capabilities()
	initial, source := catalogue(t, ctx, p, caps, query)

	for _, s := range initial {
		if strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.URL) == "" {
			t.Errorf("%s returned a series without a title or a url: %+v", source, s)
			break
		}
	}

	series, chapters := chaptersFrom(t, ctx, p, initial)
	if !p.MatchesURL(series.URL) {
		t.Errorf("%s returned %q, which MatchesURL rejects: pasting that URL would select another provider", source, series.URL)
	}

	report := fmt.Sprintf("%d series, %d chapters", len(initial), len(chapters))

	if caps.HasTags {
		tags, err := p.GetTags(ctx)
		switch {
		case err != nil:
			t.Errorf("GetTags: %v", err)
		case len(tags) == 0:
			t.Error("GetTags returned no tags: filtering by tag is broken")
		default:
			for _, tag := range tags {
				if tag.ID == "" || tag.Label == "" {
					t.Errorf("GetTags returned a tag without an id or a label: %+v", tag)
					break
				}
			}
			report += fmt.Sprintf(", %d tags", len(tags))
		}
	}

	if caps.IsText() {
		body := prose(t, ctx, p, chapters[0])
		report += fmt.Sprintf(", %d bytes of prose", len(body))
	} else {
		pages := images(t, ctx, p, chapters[0])
		report += fmt.Sprintf(", %d images", len(pages))
		probeFirstImage(t, ctx, client, pages[0])
	}

	t.Logf("%s: %s", p.Name(), report)
}

// catalogue lists series, preferring Browse because it needs no search term and
// so cannot fail merely because a title went out of print.
func catalogue(t *testing.T, ctx context.Context, p provider.Provider, caps provider.Capabilities, query string) ([]domain.Series, string) {
	t.Helper()

	if caps.CanBrowse {
		series, err := p.Browse(ctx, provider.BrowseOptions{})
		switch {
		case err != nil:
			t.Logf("Browse: %v (falling back to Search)", err)
		case len(series) > 0:
			return series, "Browse"
		default:
			t.Log("Browse returned no series (falling back to Search)")
		}
	}

	if !caps.CanSearch {
		t.Fatal("provider can neither browse nor search")
	}
	series, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search(%q): %v", query, err)
	}
	if len(series) == 0 {
		t.Fatalf("Search(%q) returned no series: either the search endpoint moved or the term no longer matches", query)
	}
	return series, "Search"
}

// chaptersFrom asks the catalogue for chapters in order and returns the first
// series that has any, so an all-external title is not mistaken for a broken
// provider. It still fails when no series has a single chapter.
func chaptersFrom(t *testing.T, ctx context.Context, p provider.Provider, series []domain.Series) (domain.Series, []domain.Chapter) {
	t.Helper()

	for i, s := range series {
		if i == liveSeriesTries {
			break
		}
		chapters, err := p.GetChapters(ctx, s)
		if err != nil {
			t.Errorf("GetChapters(%q): %v", s.Title, err)
			continue
		}
		if len(chapters) == 0 {
			t.Logf("GetChapters(%q) returned nothing, trying the next series", s.Title)
			continue
		}
		for j, ch := range chapters {
			if strings.TrimSpace(ch.URL) == "" {
				t.Errorf("chapter %d of %q has no url", j, s.Title)
				break
			}
			if ch.Index != j {
				t.Errorf("chapter %d of %q is indexed %d: ordering is broken", j, s.Title, ch.Index)
				break
			}
		}
		return s, chapters
	}

	t.Fatalf("none of the first %d series returned a chapter: the series page or the chapter endpoint may have changed shape", min(liveSeriesTries, len(series)))
	return domain.Series{}, nil
}

func images(t *testing.T, ctx context.Context, p provider.Provider, chapter domain.Chapter) []domain.Page {
	t.Helper()

	pages, err := p.GetPages(ctx, chapter)
	if err != nil {
		t.Fatalf("GetPages(%q): %v", chapter.Title, err)
	}
	if len(pages) == 0 {
		t.Fatalf("GetPages(%q) returned no pages: the reader page may have changed shape", chapter.Title)
	}
	for i, page := range pages {
		if strings.TrimSpace(page.URL) == "" {
			t.Errorf("page %d has no url", i)
			break
		}
	}
	return pages
}

// probeFirstImage fetches one page image, which is the only way to notice that a
// provider still lists pages but its image host now needs a different referer or
// has started returning an error document.
func probeFirstImage(t *testing.T, ctx context.Context, client *http.Client, page domain.Page) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page.URL, nil)
	if err != nil {
		t.Errorf("page url %q is not requestable: %v", page.URL, err)
		return
	}
	for name, values := range page.Headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("fetching the first page: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("first page returned %s: the image host may have moved or now requires another header", resp.Status)
		return
	}
	written, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Errorf("reading first page: %v", err)
		return
	}
	if written == 0 {
		t.Error("first page is empty")
		return
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Logf("first page content-type is %q, not an image", ct)
	}
}

var htmlTag = regexp.MustCompile(`</?[a-zA-Z][a-zA-Z0-9]*(\s[^>]*)?>`)

// prose checks a text chapter for the two ways a site change reaches the reader
// as visible damage: markup left in the text, and a fence that a Markdown reader
// draws as a code box.
func prose(t *testing.T, ctx context.Context, p provider.Provider, chapter domain.Chapter) string {
	t.Helper()

	text, ok := p.(provider.TextProvider)
	if !ok {
		t.Fatalf("%s declares text media but does not implement TextProvider", p.ID())
	}
	body, err := text.GetChapterText(ctx, chapter)
	if err != nil {
		t.Fatalf("GetChapterText(%q): %v", chapter.Title, err)
	}
	if strings.TrimSpace(body) == "" {
		t.Fatalf("GetChapterText(%q) returned an empty chapter", chapter.Title)
	}
	if tag := htmlTag.FindString(body); tag != "" {
		t.Errorf("chapter body contains raw HTML %q: the site now serves markup where we expect text", tag)
	}
	if strings.Contains(body, "```") {
		t.Error("chapter body contains a code fence: a reader draws that as a code box")
	}
	return body
}

func ids(providers []provider.Provider) []string {
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.ID())
	}
	return out
}

func envList(name string) []string {
	raw := os.Getenv(name)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
