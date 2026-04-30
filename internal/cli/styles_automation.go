package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	RunMarkerStarted   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FA8FF"))
	RunMarkerSucceeded = lipgloss.NewStyle().Foreground(lipgloss.Color("#4AC776"))
	RunMarkerFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C"))
	RunMarkerSkipped   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D7BA7D"))
)

// StyleDuration renders a millisecond duration with color coding:
// green-ish for fast, yellow for medium, red for slow.
func StyleDuration(ms int64) string {
	var s lipgloss.Style
	switch {
	case ms < 100:
		s = Dim
	case ms < 1000:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("#D7BA7D"))
	default:
		s = Error
	}
	if ms < 1000 {
		return s.Render(fmt.Sprintf("%dms", ms))
	}
	return s.Render(fmt.Sprintf("%.1fs", float64(ms)/1000.0))
}

// ShortCorr renders a truncated correlation ID for compact display.
func ShortCorr(id string) string {
	if len(id) < 4 {
		return Dim.Render("corr=" + id)
	}
	return Dim.Render("corr=" + id[:4] + "…")
}
