package provider_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	id         string
	name       string
	domain     string
	caps       provider.Capabilities
	tags       []domain.Tag
	seriesList []domain.Series
}

func (m *mockProvider) ID() string                          { return m.id }
func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Capabilities() provider.Capabilities { return m.caps }
func (m *mockProvider) MatchesURL(rawURL string) bool       { return strings.Contains(rawURL, m.domain) }
func (m *mockProvider) Search(_ context.Context, _ string) ([]domain.Series, error) {
	return m.seriesList, nil
}
func (m *mockProvider) Browse(_ context.Context, _ provider.BrowseOptions) ([]domain.Series, error) {
	return m.seriesList, nil
}
func (m *mockProvider) GetTags(_ context.Context) ([]domain.Tag, error) {
	return m.tags, nil
}
func (m *mockProvider) GetChapters(_ context.Context, _ domain.Series) ([]domain.Chapter, error) {
	return nil, nil
}
func (m *mockProvider) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	return nil, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := provider.NewRegistry()
	p := &mockProvider{id: "mangadex", name: "MangaDex", domain: "mangadex.org"}

	if err := reg.Register(p); err != nil {
		t.Fatalf("unexpected error registering provider: %v", err)
	}

	retrieved, ok := reg.Get("mangadex")
	if !ok {
		t.Fatal("expected provider to be found")
	}
	if retrieved.Name() != "MangaDex" {
		t.Fatalf("expected MangaDex, got %s", retrieved.Name())
	}
}

func TestRegistry_ValidationErrors(t *testing.T) {
	reg := provider.NewRegistry()

	if err := reg.Register(nil); err == nil {
		t.Fatal("expected error registering nil provider")
	}

	emptyP := &mockProvider{id: "", name: "Empty"}
	if err := reg.Register(emptyP); err == nil {
		t.Fatal("expected error registering provider with empty id")
	}

	p := &mockProvider{id: "asura", name: "Asura"}
	if err := reg.Register(p); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := reg.Register(p); err == nil {
		t.Fatal("expected error registering duplicate provider")
	}
}

func TestRegistry_FindByURL(t *testing.T) {
	reg := provider.NewRegistry()
	p1 := &mockProvider{id: "mangadex", name: "MangaDex", domain: "mangadex.org"}
	p2 := &mockProvider{id: "mangapill", name: "MangaPill", domain: "mangapill.com"}

	_ = reg.Register(p1)
	_ = reg.Register(p2)

	found, ok := reg.FindByURL("https://mangadex.org/title/12345")
	if !ok || found.ID() != "mangadex" {
		t.Fatalf("expected to find mangadex, got %v (ok=%v)", found, ok)
	}

	found, ok = reg.FindByURL("https://mangapill.com/manga/solo-leveling")
	if !ok || found.ID() != "mangapill" {
		t.Fatalf("expected to find mangapill, got %v (ok=%v)", found, ok)
	}

	_, ok = reg.FindByURL("https://unknown-site.com/comic/123")
	if ok {
		t.Fatal("expected no provider match for unknown URL")
	}
}

func TestRegistry_All(t *testing.T) {
	reg := provider.NewRegistry()
	p1 := &mockProvider{id: "z-provider", name: "Zebra Scans"}
	p2 := &mockProvider{id: "a-provider", name: "Alpha Scans"}

	_ = reg.Register(p1)
	_ = reg.Register(p2)

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(all))
	}
	if all[0].Name() != "Alpha Scans" || all[1].Name() != "Zebra Scans" {
		t.Fatalf("expected sorted order, got %s, %s", all[0].Name(), all[1].Name())
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := provider.NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := &mockProvider{
				id:     string(rune('a' + idx)),
				name:   string(rune('A' + idx)),
				domain: string(rune('a'+idx)) + ".com",
			}
			_ = reg.Register(p)
			_, _ = reg.Get(p.ID())
			_ = reg.All()
			_, _ = reg.FindByURL("https://" + p.domain)
		}(i)
	}

	wg.Wait()
}
