package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

func chapterKey(ch domain.Chapter) string {
	if ch.ID != "" {
		return ch.ID
	}
	return ch.URL
}

func (m *Model) View() string {
	var b strings.Builder

	// Header Banner
	if m.selectMode {
		b.WriteString(titleStyle.Render(" Labrador ") + " " + subtitleStyle.Render("Select Series URL for Dewey") + "\n\n")
	} else {
		b.WriteString(titleStyle.Render(" Labrador "))
		b.WriteString(" " + subtitleStyle.Render("Webcomic Scraper & Archiver"))

		var activeCount, queuedCount int
		for _, it := range m.queue {
			switch it.Status {
			case StatusDownloading:
				activeCount++
			case StatusQueued:
				queuedCount++
			}
		}
		if activeCount > 0 || queuedCount > 0 {
			b.WriteString("  " + statusDownloadingStyle.Render(fmt.Sprintf("[Queue: %d active, %d pending - press 'Q']", activeCount, queuedCount)))
		} else if len(m.queue) > 0 {
			b.WriteString("  " + statusCompletedStyle.Render(fmt.Sprintf("[Queue: %d completed - press 'Q']", len(m.queue))))
		}
		b.WriteString("\n\n")
	}

	// Render Screen Content
	switch m.screen {
	case screenProviders:
		b.WriteString(m.viewProviders())
	case screenSearch:
		b.WriteString(m.viewSearch())
	case screenBrowse:
		b.WriteString(m.viewBrowse())
	case screenChapters:
		b.WriteString(m.viewChapters())
	case screenQueue:
		b.WriteString(m.viewQueue())
	}

	// Status Bar / Helper Keybindings at bottom
	b.WriteString("\n" + m.viewStatusBar())

	out := b.String()
	if m.height > 0 {
		out = lipgloss.NewStyle().MaxHeight(m.height).Render(out)
	}
	return out
}

func (m *Model) viewProviders() string {
	var s strings.Builder
	s.WriteString("Select provider(s) to search or browse:\n\n")

	for i, p := range m.providers {
		cursor := " "
		if i == m.providerCursor {
			cursor = ">"
		}

		checked := "[ ]"
		if m.selectedProviders[p.ID()] {
			checked = badgeChecked
		}

		line := fmt.Sprintf("%s %s %s (%s)", cursor, checked, p.Name(), p.ID())
		if i == m.providerCursor {
			s.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			s.WriteString(normalItemStyle.Render(line) + "\n")
		}
	}

	s.WriteString("\n" + subtitleStyle.Render("Space: toggle select • Enter: open • j/k: navigate • Q: queue • q: quit"))
	return s.String()
}

func (m *Model) renderTabs() string {
	if m.activeProvider == nil {
		return ""
	}

	tab1 := tabStyle.Render("1: Title Search")
	if m.activeTab == tabSearch {
		tab1 = activeTabStyle.Render("1: Title Search")
	}

	tab2 := tabStyle.Render("2: Browse Catalog")
	if m.activeTab == tabBrowse {
		tab2 = activeTabStyle.Render("2: Browse Catalog")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tab1, tab2) + "\n\n"
}

func (m *Model) viewSearch() string {
	var s strings.Builder

	if !m.selectMode {
		s.WriteString(m.renderTabs())
	}

	s.WriteString("Search: " + m.textInput.View() + "\n\n")

	if m.isLoading {
		s.WriteString(m.spinner.View() + " Searching...\n")
		return s.String()
	}

	if len(m.searchResults) == 0 {
		s.WriteString(subtitleStyle.Render("Type a title above and press Enter to search.") + "\n")
		return s.String()
	}

	// Window viewport around cursor to prevent vertical overflow
	maxVisible := 14
	if m.height > 10 {
		maxVisible = m.height - 8
	}
	start := 0
	if m.searchCursor >= maxVisible {
		start = m.searchCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.searchResults) {
		end = len(m.searchResults)
	}

	// Calculate maximum width for title truncation
	availWidth := m.width - 4
	if availWidth < 20 {
		availWidth = 76
	}

	for i := start; i < end; i++ {
		item := m.searchResults[i]
		cursor := " "
		if i == m.searchCursor {
			cursor = ">"
		}

		line := fmt.Sprintf("%s [%s] %s", cursor, item.Provider.Name(), item.Series.Title)
		runes := []rune(line)
		if len(runes) > availWidth && availWidth > 1 {
			line = string(runes[:availWidth-1]) + "…"
		}

		if i == m.searchCursor {
			s.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			s.WriteString(normalItemStyle.Render(line) + "\n")
		}
	}

	if len(m.searchResults) > maxVisible {
		s.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d series", start+1, end, len(m.searchResults))))
		s.WriteByte('\n')
	}

	if m.selectMode {
		s.WriteString(subtitleStyle.Render("j/k: navigate • Enter: select series for Dewey • /: edit query • Esc/q: cancel"))
	} else {
		s.WriteString(subtitleStyle.Render("Tab: Browse catalog • /: Search • Enter: Open • Q: Queue • Esc: Back"))
	}
	return s.String()
}

func (m *Model) viewBrowse() string {
	var s strings.Builder
	s.WriteString(m.renderTabs())

	fmt.Fprintf(&s, "Sort: %s (press 's' to toggle)\n\n", m.browseSort)

	if m.isLoading {
		s.WriteString(m.spinner.View() + " Loading catalog...\n")
		return s.String()
	}

	if len(m.searchResults) == 0 {
		s.WriteString(subtitleStyle.Render("No series found.") + "\n")
		return s.String()
	}

	maxVisible := 14
	if m.height > 12 {
		maxVisible = m.height - 12
	}
	start := 0
	if m.searchCursor >= maxVisible {
		start = m.searchCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.searchResults) {
		end = len(m.searchResults)
	}

	for i := start; i < end; i++ {
		item := m.searchResults[i]
		cursor := " "
		if i == m.searchCursor {
			cursor = ">"
		}

		line := fmt.Sprintf("%s %s", cursor, item.Series.Title)
		if i == m.searchCursor {
			s.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			s.WriteString(normalItemStyle.Render(line) + "\n")
		}
	}

	if len(m.searchResults) > maxVisible {
		s.WriteByte('\n')
		s.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d series", start+1, end, len(m.searchResults))))
	}

	s.WriteString("\n" + subtitleStyle.Render("Tab: Title search • s: Toggle Popular/Recent • Enter: Open • Q: Queue • Esc: Back"))
	return s.String()
}

func (m *Model) viewChapters() string {
	var s strings.Builder
	fmt.Fprintf(&s, "Series: %s\n\n", m.activeSeries.Title)

	if m.isLoading {
		s.WriteString(m.spinner.View() + " Loading chapters...\n")
		return s.String()
	}

	if len(m.chapters) == 0 {
		s.WriteString(subtitleStyle.Render("No chapters found for this series.") + "\n")
		return s.String()
	}

	// Window viewport around cursor
	maxVisible := 16
	if m.height > 10 {
		maxVisible = m.height - 10
	}
	start := 0
	if m.chapterCursor >= maxVisible {
		start = m.chapterCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.chapters) {
		end = len(m.chapters)
	}

	for i := start; i < end; i++ {
		ch := m.chapters[i]
		cursor := " "
		if i == m.chapterCursor {
			cursor = ">"
		}

		key := chapterKey(ch)
		checked := "[ ]"
		if m.selectedChapters[key] {
			checked = badgeChecked
		}

		line := fmt.Sprintf("%s %s %s", cursor, checked, ch.Title)
		if i == m.chapterCursor {
			s.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			s.WriteString(normalItemStyle.Render(line) + "\n")
		}
	}

	s.WriteByte('\n')
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d chapters", start+1, end, len(m.chapters))))
	s.WriteByte('\n')
	s.WriteString(subtitleStyle.Render("Space: toggle • a/tab: select all/none • d/enter: queue download • Q: view queue • esc: back"))
	return s.String()
}

func (m *Model) viewQueue() string {
	var s strings.Builder

	filterLabel := "Showing All"
	if m.hideCompleted {
		filterLabel = "Hiding Completed"
	}

	fmt.Fprintf(&s, "Download Queue [%s] (press 'f' to toggle filter)\n\n", filterLabel)

	// 1. Unified Progress Bar (similar to mangal)
	var nQueued, nDownloading, nDone, nFailed int
	for _, item := range m.queue {
		switch item.Status {
		case StatusQueued:
			nQueued++
		case StatusDownloading:
			nDownloading++
		case StatusCompleted:
			nDone++
		case StatusFailed:
			nFailed++
		}
	}

	total := len(m.queue)
	if total > 0 {
		finished := nDone + nFailed
		pct := int(float64(finished) / float64(total) * 100)

		counts := fmt.Sprintf("%3d queued · %3d downloading · %3d done · %3d failed (%d%%)", nQueued, nDownloading, nDone, nFailed, pct)
		s.WriteString(counts + "\n")

		barWidth := len([]rune(counts))
		if barWidth < 35 {
			barWidth = 35
		}
		if barWidth > 60 {
			barWidth = 60
		}

		fill := int(float64(finished) / float64(total) * float64(barWidth))
		if fill > barWidth {
			fill = barWidth
		}

		filledPart := lipgloss.NewStyle().Foreground(draculaGreen).Render(strings.Repeat("▰", fill))
		emptyPart := lipgloss.NewStyle().Foreground(draculaComment).Render(strings.Repeat("▱", barWidth-fill))
		s.WriteString(filledPart + emptyPart + "\n\n")
	}

	visible := m.filteredQueue()
	if len(visible) == 0 {
		if len(m.queue) == 0 {
			s.WriteString(subtitleStyle.Render("Download queue is empty. Select chapters with 'd' to queue them.") + "\n")
		} else {
			s.WriteString(subtitleStyle.Render("All downloads completed! (press 'f' to show completed)") + "\n")
		}
		s.WriteString("\n" + subtitleStyle.Render("esc / Q: back to previous screen"))
		return s.String()
	}

	// Window queue viewport to visible lines
	maxVisible := 14
	if m.height > 12 {
		maxVisible = m.height - 10
	}
	start := 0
	if m.queueCursor >= maxVisible {
		start = m.queueCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(visible) {
		end = len(visible)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}
	windowed := visible[start:end]

	// 2. Group chapters by series for windowed subset
	type seriesGroup struct {
		seriesKey string
		series    domain.Series
		provider  provider.Provider
		items     []struct {
			indexInVisible int
			item           *QueueItem
		}
	}

	var groups []seriesGroup
	groupMap := make(map[string]int)

	for relIdx, item := range windowed {
		absIdx := start + relIdx
		key := fmt.Sprintf("%s:%s", item.Provider.ID(), item.Series.ID)
		gIdx, exists := groupMap[key]
		if !exists {
			gIdx = len(groups)
			groupMap[key] = gIdx
			groups = append(groups, seriesGroup{
				seriesKey: key,
				series:    item.Series,
				provider:  item.Provider,
			})
		}
		groups[gIdx].items = append(groups[gIdx].items, struct {
			indexInVisible int
			item           *QueueItem
		}{
			indexInVisible: absIdx,
			item:           item,
		})
	}

	for _, g := range groups {
		header := fmt.Sprintf("📚 %s (%s)", g.series.Title, g.provider.Name())
		s.WriteString(seriesHeaderStyle.Render(header) + "\n")

		for _, entry := range g.items {
			cursor := "  "
			if entry.indexInVisible == m.queueCursor {
				cursor = "> "
			}

			var statusStr string
			switch entry.item.Status {
			case StatusDownloading:
				statusStr = statusDownloadingStyle.Render(fmt.Sprintf("[%s DOWNLOADING]", m.spinner.View()))
			case StatusCompleted:
				statusStr = badgeDone
			case StatusFailed:
				statusStr = badgeFailed
			default:
				statusStr = badgeQueued
			}

			line := fmt.Sprintf("%s%s %s", cursor, statusStr, entry.item.Chapter.Title)
			if entry.item.ErrorMessage != "" {
				line += fmt.Sprintf(" - %s", statusFailedStyle.Render(entry.item.ErrorMessage))
			}

			if entry.indexInVisible == m.queueCursor {
				s.WriteString(selectedItemStyle.Render(line) + "\n")
			} else {
				s.WriteString(normalItemStyle.Render(line) + "\n")
			}
		}
	}

	s.WriteByte('\n')
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d items (%d total in queue)", start+1, end, len(visible), len(m.queue))))
	s.WriteByte('\n')
	s.WriteString(subtitleStyle.Render("j/k: navigate • g/G: top/bottom • r: resume • f: filter • c: clear • esc/Q: back"))
	return s.String()
}

func (m *Model) viewStatusBar() string {
	status := m.statusMsg
	if status == "" {
		status = "Ready"
	}
	return statusBarStyle.Render(status)
}
