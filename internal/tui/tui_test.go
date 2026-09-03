package tui_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/tui"
)

type dummyProv struct{}

func (d *dummyProv) ID() string   { return "dummy" }
func (d *dummyProv) Name() string { return "Dummy" }
func (d *dummyProv) Capabilities() provider.Capabilities {
	return provider.Capabilities{CanSearch: true}
}
func (d *dummyProv) MatchesURL(_ string) bool { return false }
func (d *dummyProv) Search(_ context.Context, _ string) ([]domain.Series, error) {
	return []domain.Series{{ID: "s1", Title: "Series 1"}}, nil
}
func (d *dummyProv) Browse(_ context.Context, _ provider.BrowseOptions) ([]domain.Series, error) {
	return nil, nil
}
func (d *dummyProv) GetTags(_ context.Context) ([]domain.Tag, error) { return nil, nil }
func (d *dummyProv) GetChapters(_ context.Context, _ domain.Series) ([]domain.Chapter, error) {
	return []domain.Chapter{{ID: "c1", Title: "Chapter 1"}}, nil
}
func (d *dummyProv) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	return nil, nil
}

func TestTUI_QueueWorkflow(t *testing.T) {
	reg := provider.NewRegistry()
	_ = reg.Register(&dummyProv{})

	dl := downloader.New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := tui.NewModel(reg, dl, ctx, cancel)

	// Open dummy provider
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod := updated.(*tui.Model)

	// Search
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod = updated.(*tui.Model)

	// Press 'Q' from anywhere opens Queue screen
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	mod = updated.(*tui.Model)

	viewStr := mod.View()
	if viewStr == "" {
		t.Errorf("expected view output, got empty")
	}

	// Press 'f' toggles filter
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	mod = updated.(*tui.Model)

	// Press 'Esc' returns from Queue screen
	_, _ = mod.Update(tea.KeyMsg{Type: tea.KeyEsc})
}

func TestTUI_QueueNavigationAndPersistence(t *testing.T) {
	reg := provider.NewRegistry()
	prov := &dummyProv{}
	_ = reg.Register(prov)

	dl := downloader.New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpCache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpCache)

	m := tui.NewModel(reg, dl, ctx, cancel)

	// Dump queue with nothing -> should succeed cleanly
	if err := m.DumpQueue(); err != nil {
		t.Fatalf("DumpQueue failed: %v", err)
	}

	// Open queue screen and test g / G navigation
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	mod := updated.(*tui.Model)

	// Jump to top 'g' and bottom 'G'
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	mod = updated.(*tui.Model)

	_, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
}
