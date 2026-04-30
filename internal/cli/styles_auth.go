package cli

import "github.com/charmbracelet/lipgloss"

var (
	BadgeWrite = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#a87000")).
			Padding(0, 1).Bold(true)

	BadgeOK = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#22863a")).
		Padding(0, 1).Bold(true)

	BadgeError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#cb2431")).
			Padding(0, 1).Bold(true)

	Identifier = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79b8ff"))

	RuleName = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5fafff")).
			Bold(true).Underline(true)

	SecretBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#ff0000")).
			Padding(1, 2).
			Foreground(lipgloss.Color("#ffffff"))
)

func BadgeRole(slug string) lipgloss.Style {
	palette := []string{"#5f87ff", "#5fafd7", "#87af5f", "#d7af5f", "#ff8787"}
	h := 0
	for _, c := range slug {
		h = (h*31 + int(c)) & 0x7fff
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color(palette[h%len(palette)])).
		Padding(0, 1).Bold(true)
}
