package cli

import "github.com/charmbracelet/lipgloss"

var (
	BadgeRead  = lipgloss.NewStyle().Background(lipgloss.Color("#3B82F6")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1)
	BadgeCall  = lipgloss.NewStyle().Background(lipgloss.Color("#10B981")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1)
	BadgeAdmin = lipgloss.NewStyle().Background(lipgloss.Color("#F59E0B")).Foreground(lipgloss.Color("#1F2937")).Bold(true).Padding(0, 1)
	BadgeWarn  = lipgloss.NewStyle().Background(lipgloss.Color("#FCD34D")).Foreground(lipgloss.Color("#1F2937")).Bold(true).Padding(0, 1)

	ToolNameStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	SubtleText    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)
