package tui

import (
	"fmt"
	"os"
	"path/filepath"
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
		lines := strings.Split(out, "\n")
		if len(lines) > m.height {
			out = strings.Join(lines[:m.height], "\n")
		}
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

func (m *Model) renderTouchBar(buttons []string) string {
	var rendered []string
	for i, btn := range buttons {
		if i == 0 {
			rendered = append(rendered, touchBtnPrimaryStyle.Render(btn))
		} else {
			rendered = append(rendered, touchBtnStyle.Render(btn))
		}
	}
	return "\n\n" + strings.Join(rendered, "  ") + "\n"
}

func (m *Model) renderSeriesDetails(item searchResultItem, width int) string {
	if width < 25 {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(draculaPurple).Render(item.Series.Title) + "\n\n")

	b.WriteString(metadataLabelStyle.Render("Provider: ") + metadataValueStyle.Render(item.Provider.Name()) + "\n")
	if item.Series.URL != "" {
		urlStr := item.Series.URL
		if len([]rune(urlStr)) > width-12 {
			urlStr = string([]rune(urlStr)[:width-13]) + "…"
		}
		b.WriteString(metadataLabelStyle.Render("URL: ") + subtitleStyle.Render(urlStr) + "\n")
	}

	caps := item.Provider.Capabilities()
	var capList []string
	if caps.CanSearch {
		capList = append(capList, "Search")
	}
	if caps.CanBrowse {
		capList = append(capList, "Browse")
	}
	if len(capList) > 0 {
		b.WriteString(metadataLabelStyle.Render("Features: ") + metadataValueStyle.Render(strings.Join(capList, ", ")) + "\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(draculaCyan).Render("Controls") + "\n")
	if m.selectMode {
		b.WriteString("• Enter / Tap: Select for Dewey\n")
		b.WriteString("• i: Search query (I: clear & type)\n")
		b.WriteString("• Esc: Exit input / Cancel\n")
	} else {
		b.WriteString("• Enter / Tap: Open chapters\n")
		b.WriteString("• j / k (↑/↓): Navigate list\n")
		b.WriteString("• g / G: Top / Bottom\n")
		b.WriteString("• ctrl+d / u: Half-page scroll\n")
		b.WriteString("• i: Search query (I: clear & type)\n")
		b.WriteString("• Esc: Exit input / Back\n")
		b.WriteString("• Q: View download queue\n")
	}

	return paneBorderStyle.Width(width).Render(b.String())
}

func (m *Model) renderChapterDetails(width int) string {
	if width < 25 {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(draculaPurple).Render(m.activeSeries.Title) + "\n\n")

	b.WriteString(metadataLabelStyle.Render("Provider: ") + metadataValueStyle.Render(m.activeProvider.Name()) + "\n")
	b.WriteString(metadataLabelStyle.Render("Total Chapters: ") + metadataValueStyle.Render(fmt.Sprintf("%d", len(m.chapters))) + "\n")
	b.WriteString(metadataLabelStyle.Render("Selected: ") + lipgloss.NewStyle().Foreground(draculaGreen).Bold(true).Render(fmt.Sprintf("%d", len(m.selectedChapters))) + "\n")

	homeDir, _ := os.UserHomeDir()
	dest := filepath.Join(homeDir, "Downloads", "Manga", m.activeSeries.Title)
	if len([]rune(dest)) > width-14 {
		dest = string([]rune(dest)[:width-15]) + "…"
	}
	b.WriteString(metadataLabelStyle.Render("Destination: ") + subtitleStyle.Render(dest) + "\n")
	b.WriteString(metadataLabelStyle.Render("Archive: ") + metadataValueStyle.Render(".cbz (Store)") + "\n")

	b.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(draculaCyan).Render("Controls") + "\n")
	b.WriteString("• Space / Tap: Toggle selected\n")
	b.WriteString("• a / tab: Select all / none\n")
	b.WriteString("• d / Enter: Queue download\n")
	b.WriteString("• j / k (↑/↓): Navigate list\n")
	b.WriteString("• g / G: Top / Bottom\n")
	b.WriteString("• ctrl+d / u: Half-page scroll\n")
	b.WriteString("• Q: Open queue screen\n")
	b.WriteString("• Esc: Back to series\n")

	return paneBorderStyle.Width(width).Render(b.String())
}

func (m *Model) renderQueueDetails(visible []*QueueItem, width int) string {
	if width < 25 {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(draculaPurple).Render("Queue Summary") + "\n\n")

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

	b.WriteString(metadataLabelStyle.Render("Total Items: ") + metadataValueStyle.Render(fmt.Sprintf("%d", len(m.queue))) + "\n")
	b.WriteString(metadataLabelStyle.Render("Downloading: ") + statusDownloadingStyle.Render(fmt.Sprintf("%d", nDownloading)) + "\n")
	b.WriteString(metadataLabelStyle.Render("Queued: ") + statusQueuedStyle.Render(fmt.Sprintf("%d", nQueued)) + "\n")
	b.WriteString(metadataLabelStyle.Render("Completed: ") + statusCompletedStyle.Render(fmt.Sprintf("%d", nDone)) + "\n")
	b.WriteString(metadataLabelStyle.Render("Failed: ") + statusFailedStyle.Render(fmt.Sprintf("%d", nFailed)) + "\n\n")

	if len(visible) > 0 && m.queueCursor < len(visible) {
		selected := visible[m.queueCursor]
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(draculaPink).Render("Selected Item") + "\n")
		b.WriteString(metadataLabelStyle.Render("Series: ") + metadataValueStyle.Render(selected.Series.Title) + "\n")
		b.WriteString(metadataLabelStyle.Render("Chapter: ") + metadataValueStyle.Render(selected.Chapter.Title) + "\n")
		if selected.ErrorMessage != "" {
			b.WriteString(statusFailedStyle.Render("Error: "+selected.ErrorMessage) + "\n")
		}
	}

	b.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(draculaCyan).Render("Controls") + "\n")
	b.WriteString("• r: Resume failed downloads\n")
	b.WriteString("• f: Toggle hide completed\n")
	b.WriteString("• c: Clear completed\n")
	b.WriteString("• j/k: Navigate • Esc/Q: Back\n")

	return paneBorderStyle.Width(width).Render(b.String())
}

func (m *Model) viewSearch() string {
	var s strings.Builder

	if !m.selectMode {
		s.WriteString(m.renderTabs())
	}

	var modeTag string
	var modeHelp string
	if m.textInput.Focused() {
		modeTag = lipgloss.NewStyle().Bold(true).Foreground(draculaPink).Render("[INSERT]")
		modeHelp = subtitleStyle.Render(" (Esc: normal mode • Enter: search)")
	} else {
		modeTag = lipgloss.NewStyle().Bold(true).Foreground(draculaGreen).Render("[NORMAL]")
		modeHelp = subtitleStyle.Render(" (i: type query • I: clear & type • Esc: back)")
	}

	s.WriteString("Search " + modeTag + ": " + m.textInput.View() + modeHelp + "\n\n")

	if m.isLoading {
		s.WriteString(m.spinner.View() + " Searching...\n")
		return s.String()
	}

	if len(m.searchResults) == 0 {
		if m.textInput.Focused() {
			s.WriteString(subtitleStyle.Render("Type title above and press Enter to search.") + "\n")
		} else {
			s.WriteString(subtitleStyle.Render("Press 'i' to type search query, or 'I' to clear & type. Press Esc to return.") + "\n")
		}
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

	isDesktop := !m.IsCompact()
	listWidth := m.width - 4
	if isDesktop {
		listWidth = (m.width * 54) / 100
	}
	if listWidth < 20 {
		listWidth = 76
	}

	var listBuilder strings.Builder
	for i := start; i < end; i++ {
		item := m.searchResults[i]
		cursor := " "
		if i == m.searchCursor {
			cursor = ">"
		}

		line := fmt.Sprintf("%s [%s] %s", cursor, item.Provider.Name(), item.Series.Title)
		runes := []rune(line)
		if len(runes) > listWidth && listWidth > 1 {
			line = string(runes[:listWidth-1]) + "…"
		}

		if i == m.searchCursor {
			listBuilder.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			listBuilder.WriteString(normalItemStyle.Render(line) + "\n")
		}
	}

	if len(m.searchResults) > maxVisible {
		listBuilder.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d series", start+1, end, len(m.searchResults))))
		listBuilder.WriteByte('\n')
	}

	if isDesktop {
		rightWidth := m.width - listWidth - 5
		var rightPane string
		if m.searchCursor < len(m.searchResults) {
			rightPane = m.renderSeriesDetails(m.searchResults[m.searchCursor], rightWidth)
		}
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listBuilder.String(), "  ", rightPane))
		s.WriteString("\n\n" + subtitleStyle.Render("j/k: navigate • Enter: open • i: search • I: clear • Esc: back • Q: queue"))
	} else {
		s.WriteString(listBuilder.String())
		if m.selectMode {
			s.WriteString(m.renderTouchBar([]string{"[ ⏎ Select ]", "[ 🔍 Search ]", "[ ✖ Cancel ]"}))
		} else {
			s.WriteString(m.renderTouchBar([]string{"[ ⏎ Open ]", "[ 🔍 Search ]", "[ 📋 Queue ]", "[ ✖ Back ]"}))
		}
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

	isDesktop := !m.IsCompact()
	listWidth := m.width - 4
	if isDesktop {
		listWidth = (m.width * 54) / 100
	}
	if listWidth < 20 {
		listWidth = 76
	}

	var listBuilder strings.Builder
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
		runes := []rune(line)
		if len(runes) > listWidth && listWidth > 1 {
			line = string(runes[:listWidth-1]) + "…"
		}

		if i == m.chapterCursor {
			listBuilder.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			listBuilder.WriteString(normalItemStyle.Render(line) + "\n")
		}
	}

	if len(m.chapters) > maxVisible {
		listBuilder.WriteByte('\n')
		listBuilder.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d chapters", start+1, end, len(m.chapters))))
		listBuilder.WriteByte('\n')
	}

	if isDesktop {
		rightWidth := m.width - listWidth - 5
		rightPane := m.renderChapterDetails(rightWidth)
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listBuilder.String(), "  ", rightPane))
		s.WriteString("\n\n" + subtitleStyle.Render("Space: toggle • a: select all/none • d/Enter: download • j/k: navigate • Q: queue • Esc: back"))
	} else {
		s.WriteString(listBuilder.String())
		s.WriteString(m.renderTouchBar([]string{"[ ␣ Toggle ]", "[ ✓ All ]", "[ 📥 Download ]", "[ 📋 Queue ]", "[ ✖ Back ]"}))
	}
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

	isDesktop := !m.IsCompact()
	listWidth := m.width - 4
	if isDesktop {
		listWidth = (m.width * 56) / 100
	}
	if listWidth < 20 {
		listWidth = 76
	}

	var listBuilder strings.Builder
	for _, g := range groups {
		header := fmt.Sprintf("📚 %s (%s)", g.series.Title, g.provider.Name())
		listBuilder.WriteString(seriesHeaderStyle.Render(header) + "\n")

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
			runes := []rune(line)
			if len(runes) > listWidth && listWidth > 1 {
				line = string(runes[:listWidth-1]) + "…"
			}

			if entry.indexInVisible == m.queueCursor {
				listBuilder.WriteString(selectedItemStyle.Render(line) + "\n")
			} else {
				listBuilder.WriteString(normalItemStyle.Render(line) + "\n")
			}
		}
	}

	if len(visible) > maxVisible {
		listBuilder.WriteByte('\n')
		listBuilder.WriteString(subtitleStyle.Render(fmt.Sprintf("Showing %d-%d of %d items (%d total in queue)", start+1, end, len(visible), len(m.queue))))
		listBuilder.WriteByte('\n')
	}

	if isDesktop {
		rightWidth := m.width - listWidth - 5
		rightPane := m.renderQueueDetails(visible, rightWidth)
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listBuilder.String(), "  ", rightPane))
		s.WriteString("\n\n" + subtitleStyle.Render("j/k: navigate • g/G: top/bottom • ctrl+d/u: half-page • r: resume • f: filter • c: clear • Esc/Q: back"))
	} else {
		s.WriteString(listBuilder.String())
		s.WriteString(m.renderTouchBar([]string{"[ 🔄 Resume ]", "[ 👁 Filter ]", "[ 🧹 Clear ]", "[ ✖ Back ]"}))
	}
	return s.String()
}

func (m *Model) viewStatusBar() string {
	status := m.statusMsg
	if status == "" {
		status = "Ready"
	}
	return statusBarStyle.Render(status)
}
