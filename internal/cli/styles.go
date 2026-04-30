// Package cli owns the gohome CLI command tree, styling, and output helpers.
package cli

import "github.com/charmbracelet/lipgloss"

var (
	Header      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C7CFF"))
	EntityID    = lipgloss.NewStyle().Foreground(lipgloss.Color("#4EC9B0"))
	Kind        = lipgloss.NewStyle().Foreground(lipgloss.Color("#DCDCAA"))
	Timestamp   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Correlation = lipgloss.NewStyle().Foreground(lipgloss.Color("#C586C0"))
	Error       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C")).Bold(true)
	Success     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4AC776"))
	Dim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)
