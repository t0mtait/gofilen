package tui

import "github.com/charmbracelet/lipgloss"

var (
	// ── Palette ───────────────────────────────────────────────────────────────
	cBase    = lipgloss.Color("#1E1E2E") // dark background
	cSurface = lipgloss.Color("#313244") // slightly lighter surface
	cMuted   = lipgloss.Color("#6C7086") // muted text
	cSubtext = lipgloss.Color("#A6ADC8") // secondary text
	cText    = lipgloss.Color("#CDD6F4") // primary text
	cLavender = lipgloss.Color("#B4BEFE") // accent
	cBlue    = lipgloss.Color("#89B4FA") // user messages
	cGreen   = lipgloss.Color("#A6E3A1") // AI messages
	cYellow  = lipgloss.Color("#F9E2AF") // tool calls
	cPeach   = lipgloss.Color("#FAB387") // tool results
	cRed     = lipgloss.Color("#F38BA8") // errors
	cTeal    = lipgloss.Color("#94E2D5") // system / header

	// ── Header ────────────────────────────────────────────────────────────────
	headerStyle = lipgloss.NewStyle().
			Background(cBase).
			Padding(0, 2)

	headerTitleStyle = lipgloss.NewStyle().
				Foreground(cTeal).
				Bold(true)

	headerDimStyle = lipgloss.NewStyle().
			Foreground(cMuted)

	headerSpinnerStyle = lipgloss.NewStyle().
				Foreground(cLavender)

	// ── Viewport border ───────────────────────────────────────────────────────
	viewportBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(cSurface)

	// ── Message labels ────────────────────────────────────────────────────────
	userLabelStyle = lipgloss.NewStyle().
			Foreground(cBlue).
			Bold(true)

	aiLabelStyle = lipgloss.NewStyle().
			Foreground(cGreen).
			Bold(true)

	// ── Message bodies ────────────────────────────────────────────────────────
	userBodyStyle = lipgloss.NewStyle().
			Foreground(cText)

	aiBodyStyle = lipgloss.NewStyle().
			Foreground(cText)

	// ── Tool call box ─────────────────────────────────────────────────────────
	toolCallBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(cYellow).
				Foreground(cYellow).
				Padding(0, 1)

	// ── Tool result sidebar ───────────────────────────────────────────────────
	toolResultBarStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.ThickBorder()).
				BorderLeft(true).
				BorderForeground(cPeach).
				Foreground(cSubtext).
				PaddingLeft(1)

	// ── System message ────────────────────────────────────────────────────────
	systemStyle = lipgloss.NewStyle().
			Foreground(cMuted).
			Italic(true)

	// ── Error ─────────────────────────────────────────────────────────────────
	errorStyle = lipgloss.NewStyle().
			Foreground(cRed).
			Bold(true)

	// ── Input area ────────────────────────────────────────────────────────────
	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(cLavender).
				Padding(0, 1)

	inputIdleStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cSurface).
			Padding(0, 1)

	// ── Footer ────────────────────────────────────────────────────────────────
	footerStyle = lipgloss.NewStyle().
			Foreground(cMuted).
			Padding(0, 2)

	keyStyle = lipgloss.NewStyle().
			Foreground(cLavender).
			Bold(true)
)
