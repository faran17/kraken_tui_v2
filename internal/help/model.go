package help

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/pkg/styles"
)

type Model struct {
	width  int
	height int
}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(styles.Bold.Render("🐙 KRAKEN TUI v2 HELP"))
	s.WriteString("\n\n")

	sections := []struct {
		title string
		keys  [][2]string
	}{
		{
			"Global",
			[][2]string{
				{"Tab", "Next Panel"},
				{"Shift+Tab", "Prev Panel"},
				{"s", "Setup Menu"},
				{"?", "Show Help"},
				{"Ctrl+C", "Quit"},
			},
		},
		{
			"Files",
			[][2]string{
				{"↑/↓", "Navigate"},
				{"Enter", "Open"},
				{"n/N", "New File/Dir"},
				{"r", "Rename"},
				{"d", "Delete"},
				{"y/x/p", "Copy/Cut/Paste"},
				{".", "Toggle Hidden"},
			},
		},
		{
			"Chat",
			[][2]string{
				{"Enter", "Send Message"},
				{"Alt+N", "New Session"},
				{"Alt+←/→", "Switch Sessions"},
			},
		},
	}

	for _, sec := range sections {
		s.WriteString(styles.Bold.Foreground(lipgloss.Color(styles.ColorAccent)).Render(sec.title))
		s.WriteString("\n")
		for _, k := range sec.keys {
			keyStr := lipgloss.NewStyle().Width(12).Foreground(lipgloss.Color(styles.ColorAccentLight)).Render(k[0])
			s.WriteString(fmt.Sprintf("%s %s\n", keyStr, k[1]))
		}
		s.WriteString("\n")
	}

	s.WriteString(styles.Dim.Render("Press ESC or ? to close"))

	box := lipgloss.NewStyle().
		Padding(1, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(styles.ColorAccent)).
		Render(s.String())

	return box
}

func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	return m
}
