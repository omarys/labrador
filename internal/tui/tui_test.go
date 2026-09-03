package tui_test

import (
	"context"
	"strings"
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

func TestTUI_SearchVimMode(t *testing.T) {
	reg := provider.NewRegistry()
	_ = reg.Register(&dummyProv{})

	dl := downloader.New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := tui.NewModel(reg, dl, ctx, cancel)

	// 1. Enter provider screen -> enter search screen
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod := updated.(*tui.Model)

	// In search screen, starts in Normal mode
	viewNormal := mod.View()
	if !strings.Contains(viewNormal, "[NORMAL]") {
		t.Errorf("expected [NORMAL] mode indicator in view, got: %s", viewNormal)
	}

	// 2. Press 'i' to enter Insert mode
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	mod = updated.(*tui.Model)

	viewInsert := mod.View()
	if !strings.Contains(viewInsert, "[INSERT]") {
		t.Errorf("expected [INSERT] mode indicator in view, got: %s", viewInsert)
	}

	// 3. Type letters including 'j', 'Q', 'q' in Insert mode -> they should not trigger shortcuts
	for _, r := range []rune{'j', 'u', 'j', 'u', 't', 's', 'u', 'Q', 'q'} {
		updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		mod = updated.(*tui.Model)
	}

	viewWithQuery := mod.View()
	if !strings.Contains(viewWithQuery, "jujutsuQq") {
		t.Errorf("expected query text to contain 'jujutsuQq', got: %s", viewWithQuery)
	}

	// 4. Press 'Esc' to exit Insert mode back to Normal mode
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mod = updated.(*tui.Model)

	viewBackToNormal := mod.View()
	if !strings.Contains(viewBackToNormal, "[NORMAL]") {
		t.Errorf("expected [NORMAL] mode after Esc, got: %s", viewBackToNormal)
	}

	// 5. Press 'I' (capital I) -> clears query and enters Insert mode
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	mod = updated.(*tui.Model)

	viewClearedInsert := mod.View()
	if !strings.Contains(viewClearedInsert, "[INSERT]") {
		t.Errorf("expected [INSERT] mode after capital I, got: %s", viewClearedInsert)
	}
	if strings.Contains(viewClearedInsert, "jujutsuQq") {
		t.Errorf("expected previous query to be cleared, but found it in view: %s", viewClearedInsert)
	}

	// 6. Press 'Esc' to exit Insert mode
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mod = updated.(*tui.Model)

	// 7. Additional 'Esc' returns to provider select screen
	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mod = updated.(*tui.Model)

	viewProviders := mod.View()
	if !strings.Contains(viewProviders, "Select provider(s)") {
		t.Errorf("expected provider selection screen after second Esc, got: %s", viewProviders)
	}
}
