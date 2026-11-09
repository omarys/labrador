package tui_test

import (
	"context"
	"encoding/json"
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

// recordingProv records the BrowseOptions it receives so tests can assert on the
// filters the TUI forwards to the provider.
type recordingProv struct {
	caps       provider.Capabilities
	tags       []domain.Tag
	lastBrowse provider.BrowseOptions
}

func (r *recordingProv) ID() string   { return "recording" }
func (r *recordingProv) Name() string { return "Recording" }
func (r *recordingProv) Capabilities() provider.Capabilities {
	return r.caps
}
func (r *recordingProv) MatchesURL(_ string) bool { return false }
func (r *recordingProv) Search(_ context.Context, _ string) ([]domain.Series, error) {
	return nil, nil
}
func (r *recordingProv) Browse(_ context.Context, opts provider.BrowseOptions) ([]domain.Series, error) {
	r.lastBrowse = opts
	return []domain.Series{{ID: "s1", Title: "Series 1"}}, nil
}
func (r *recordingProv) GetTags(_ context.Context) ([]domain.Tag, error) { return r.tags, nil }
func (r *recordingProv) GetChapters(_ context.Context, _ domain.Series) ([]domain.Chapter, error) {
	return nil, nil
}
func (r *recordingProv) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	return nil, nil
}

// newBrowseModel opens the single registered provider and switches to its Browse tab.
func newBrowseModel(t *testing.T, prov provider.Provider) *tui.Model {
	t.Helper()

	reg := provider.NewRegistry()
	if err := reg.Register(prov); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := tui.NewModel(reg, downloader.New(nil), ctx, cancel)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod := updated.(*tui.Model)

	updated, _ = mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	return updated.(*tui.Model)
}

func pressRune(mod *tui.Model, r rune) (*tui.Model, tea.Cmd) {
	updated, cmd := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(*tui.Model), cmd
}

func updateMsg(mod *tui.Model, msg tea.Msg) (*tui.Model, tea.Cmd) {
	updated, cmd := mod.Update(msg)
	return updated.(*tui.Model), cmd
}

// runCmd executes a command and feeds its message back into Update, which is what
// the bubbletea runtime would do.
func runCmd(t *testing.T, mod *tui.Model, cmd tea.Cmd) *tui.Model {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected a command, got nil")
	}
	msg := cmd()
	updated, _ := mod.Update(msg)
	return updated.(*tui.Model)
}

func TestTUI_BrowseSortCyclesProviderSorts(t *testing.T) {
	prov := &recordingProv{caps: provider.Capabilities{
		CanSearch:      true,
		CanBrowse:      true,
		SupportedSorts: []domain.SortOrder{domain.SortPopular, domain.SortRecent, domain.SortRating},
	}}
	mod := newBrowseModel(t, prov)

	var got []domain.SortOrder
	for i := 0; i < 4; i++ {
		var cmd tea.Cmd
		mod, cmd = pressRune(mod, 's')
		mod = runCmd(t, mod, cmd)
		got = append(got, prov.lastBrowse.Sort)
	}

	want := []domain.SortOrder{domain.SortRecent, domain.SortRating, domain.SortPopular, domain.SortRecent}
	if len(got) != len(want) {
		t.Fatalf("expected %d sort values, got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort cycle = %v, want %v", got, want)
		}
	}
}

func TestTUI_BrowseSortCyclesFallbackWhenProviderDeclaresNoSorts(t *testing.T) {
	prov := &recordingProv{caps: provider.Capabilities{CanSearch: true, CanBrowse: true}}
	mod := newBrowseModel(t, prov)

	var got []domain.SortOrder
	for i := 0; i < 3; i++ {
		var cmd tea.Cmd
		mod, cmd = pressRune(mod, 's')
		mod = runCmd(t, mod, cmd)
		got = append(got, prov.lastBrowse.Sort)
	}

	want := []domain.SortOrder{domain.SortRecent, domain.SortPopular, domain.SortRecent}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fallback sort cycle = %v, want %v", got, want)
		}
	}
}

func TestTUI_TagPickerSelectsAndClearsFilter(t *testing.T) {
	prov := &recordingProv{
		caps: provider.Capabilities{
			CanSearch:      true,
			CanBrowse:      true,
			HasTags:        true,
			SupportedSorts: []domain.SortOrder{domain.SortPopular, domain.SortRecent},
		},
		tags: []domain.Tag{{ID: "action", Label: "Action"}, {ID: "fantasy", Label: "Fantasy"}},
	}
	mod := newBrowseModel(t, prov)

	// 't' fetches the tag list, whose result opens the picker.
	mod, cmd := pressRune(mod, 't')
	mod = runCmd(t, mod, cmd)
	if view := mod.View(); !strings.Contains(view, "Select a tag") {
		t.Fatalf("expected tag picker after fetch, got: %s", view)
	}

	// Esc closes the overlay instead of navigating back a screen.
	mod, _ = updateMsg(mod, tea.KeyMsg{Type: tea.KeyEsc})
	view := mod.View()
	if strings.Contains(view, "Select a tag") {
		t.Errorf("expected picker closed after Esc, got: %s", view)
	}
	if !strings.Contains(view, "Sort:") {
		t.Errorf("expected to stay on the Browse tab after Esc, got: %s", view)
	}

	// Reopen from the cache, move to the second tag and filter by it.
	mod, cmd = pressRune(mod, 't')
	if cmd != nil {
		t.Fatalf("expected cached tags, got a fetch command")
	}
	mod, _ = pressRune(mod, 'j')
	mod, cmd = updateMsg(mod, tea.KeyMsg{Type: tea.KeyEnter})
	mod = runCmd(t, mod, cmd)

	if prov.lastBrowse.Tag == nil || prov.lastBrowse.Tag.ID != "fantasy" {
		t.Fatalf("expected tag filter fantasy, got %+v", prov.lastBrowse.Tag)
	}
	view = mod.View()
	if strings.Contains(view, "Select a tag") {
		t.Errorf("expected picker closed after selection, got: %s", view)
	}
	if !strings.Contains(view, "Tag: Fantasy") {
		t.Errorf("expected header to show active tag, got: %s", view)
	}

	// 'a' clears the filter and reloads the unfiltered catalog.
	mod, cmd = pressRune(mod, 'a')
	mod = runCmd(t, mod, cmd)
	if prov.lastBrowse.Tag != nil {
		t.Errorf("expected cleared tag filter, got %+v", prov.lastBrowse.Tag)
	}
	if view := mod.View(); !strings.Contains(view, "Tag: all") {
		t.Errorf("expected header to show all genres, got: %s", view)
	}
}

func TestTUI_BrowseCommandForwardsActiveTag(t *testing.T) {
	prov := &recordingProv{
		caps: provider.Capabilities{
			CanSearch:      true,
			CanBrowse:      true,
			HasTags:        true,
			SupportedSorts: []domain.SortOrder{domain.SortPopular, domain.SortRecent, domain.SortRating},
		},
		tags: []domain.Tag{{ID: "action", Label: "Action"}, {ID: "fantasy", Label: "Fantasy"}},
	}
	mod := newBrowseModel(t, prov)

	mod, cmd := pressRune(mod, 't')
	mod = runCmd(t, mod, cmd)
	mod, _ = pressRune(mod, 'j')
	mod, cmd = updateMsg(mod, tea.KeyMsg{Type: tea.KeyEnter})
	mod = runCmd(t, mod, cmd)

	if prov.lastBrowse.Tag == nil || prov.lastBrowse.Tag.ID != "fantasy" {
		t.Fatalf("selecting a tag should forward it to Browse, got %+v", prov.lastBrowse.Tag)
	}

	// Cycling the sort keeps the tag filter applied.
	mod, cmd = pressRune(mod, 's')
	_ = runCmd(t, mod, cmd)
	if prov.lastBrowse.Tag == nil || prov.lastBrowse.Tag.ID != "fantasy" {
		t.Errorf("sort cycling should keep the tag filter, got %+v", prov.lastBrowse.Tag)
	}
	if prov.lastBrowse.Sort != domain.SortRecent {
		t.Errorf("expected sort recent after cycling from popular, got %s", prov.lastBrowse.Sort)
	}
}

func TestTUI_SelectModeStartsOnProviderScreen(t *testing.T) {
	reg := provider.NewRegistry()
	_ = reg.Register(&dummyProv{})

	dl := downloader.New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A preselected id must be checked, and the caller's query must not skip the
	// provider screen even though it is supplied up front.
	m := tui.NewSelectModel(reg, dl, ctx, cancel, "Series 1", []string{"dummy"})

	view := m.View()
	if !strings.Contains(view, "Select provider(s)") {
		t.Fatalf("select mode should open on the provider screen, got:\n%s", view)
	}
	if !strings.Contains(view, "[x]") {
		t.Fatalf("expected preselected provider to be checked, got:\n%s", view)
	}
}

func TestTUI_SelectModeEscReturnsToProvidersInsteadOfQuitting(t *testing.T) {
	reg := provider.NewRegistry()
	_ = reg.Register(&dummyProv{})

	dl := downloader.New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := tui.NewSelectModel(reg, dl, ctx, cancel, "", []string{"dummy"})

	// Providers -> search (no query, so nothing is dispatched).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod := updated.(*tui.Model)
	if strings.Contains(mod.View(), "Select provider(s)") {
		t.Fatalf("expected to leave the provider screen on enter")
	}

	// Esc must step back to the provider screen, not quit select mode.
	updated, cmd := mod.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mod = updated.(*tui.Model)
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatalf("esc from the search screen must go back to providers, not quit")
		}
	}
	if !strings.Contains(mod.View(), "Select provider(s)") {
		t.Fatalf("expected provider screen after esc, got:\n%s", mod.View())
	}
}

func TestSelectedSeriesInfoJSONCarriesProviderSet(t *testing.T) {
	info := tui.SelectedSeriesInfo{
		Title:     "Chainsaw Man",
		URL:       "https://weebcentral.com/series/01J",
		Provider:  "Weeb Central",
		Providers: []string{"weebcentral", "webtoons"},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"title"`, `"url"`, `"provider"`, `"providers"`, `"weebcentral"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("expected %s in %s", want, data)
		}
	}

	// Dewey relies on the field being optional for older binaries.
	var bare tui.SelectedSeriesInfo
	if err := json.Unmarshal([]byte(`{"title":"x","url":"u"}`), &bare); err != nil {
		t.Fatalf("unmarshal without providers: %v", err)
	}
	if len(bare.Providers) != 0 {
		t.Fatalf("expected empty providers, got %v", bare.Providers)
	}
}
