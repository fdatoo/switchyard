package cli

import "github.com/charmbracelet/lipgloss"

var (
	PackVerified = lipgloss.NewStyle().Foreground(lipgloss.Color("#4AC776")).Bold(true)
	PackUnsigned = lipgloss.NewStyle().Foreground(lipgloss.Color("#D7BA7D"))
	PackExpired  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C"))
	PackName     = lipgloss.NewStyle().Bold(true)
	PackVersion  = lipgloss.NewStyle().Foreground(lipgloss.Color("#4EC9B0"))
)
