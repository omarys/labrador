package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Dracula Theme Palette
	draculaBg        = lipgloss.Color("#282a36")
	draculaSelection = lipgloss.Color("#44475a")
	draculaFg        = lipgloss.Color("#f8f8f2")
	draculaComment   = lipgloss.Color("#6272a4")
	draculaCyan      = lipgloss.Color("#8be9fd")
	draculaGreen     = lipgloss.Color("#50fa7b")
	draculaPink      = lipgloss.Color("#ff79c6")
	draculaPurple    = lipgloss.Color("#bd93f9")
	draculaRed       = lipgloss.Color("#ff5555")
	draculaYellow    = lipgloss.Color("#f1fa8c")

	// Base Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(draculaBg).
			Background(draculaPurple).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(draculaComment).
			Italic(true)

	// Tab Styles
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(draculaComment)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(draculaPink).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(draculaPink).
			Padding(0, 2)

	// List Item Styles
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(draculaFg).
				Background(draculaSelection).
				Bold(true).
				PaddingLeft(1).
				MaxHeight(1)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(draculaFg).
			PaddingLeft(1).
			MaxHeight(1)

	checkedStyle = lipgloss.NewStyle().
			Foreground(draculaGreen).
			Bold(true)

	// Status Bar Style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(draculaCyan).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(draculaSelection).
			MarginTop(1).
			Padding(0, 1)

	// Queue Status Styles
	statusQueuedStyle = lipgloss.NewStyle().
				Foreground(draculaComment)

	statusDownloadingStyle = lipgloss.NewStyle().
				Foreground(draculaYellow).
				Bold(true)

	statusCompletedStyle = lipgloss.NewStyle().
				Foreground(draculaGreen).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(draculaRed).
				Bold(true)

	seriesHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(draculaPurple).
				MarginTop(1)

	// Pre-rendered static badges (avoids repeated Lipgloss rendering in loops)
	badgeChecked = checkedStyle.Render("[x]")
	badgeDone    = statusCompletedStyle.Render("[✓ DONE]")
	badgeFailed  = statusFailedStyle.Render("[✗ FAILED]")
	badgeQueued  = statusQueuedStyle.Render("[⏳ QUEUED]")
)
