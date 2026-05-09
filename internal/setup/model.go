package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/internal/config"
	"github.com/faran17/kraken-tui/pkg/styles"
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type Model struct {
	cfg      *config.Config
	width    int
	height   int
	focused  int // 0: Theme List, 1: Dark Mode Toggle, 2: API Key Input

	themeList list.Model
	apiKeyIn  textinput.Model
}

func New(cfg *config.Config) Model {
	themes := []list.Item{
		item{title: "Ocean", desc: "Classic Kraken depths"},
		item{title: "Dracula", desc: "Vibrant vampire palette"},
		item{title: "Gruvbox", desc: "Retro earthy tones"},
	}

	l := list.New(themes, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Theme"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = styles.Bold.Copy().Foreground(lipgloss.Color(styles.ColorAccent))

	ti := textinput.New()
	ti.Placeholder = "Enter Gemini API Key..."
	ti.SetValue(cfg.GeminiAPIKey)
	ti.CharLimit = 100
	ti.Width = 30

	return Model{
		cfg:       cfg,
		themeList: l,
		apiKeyIn:  ti,
		focused:   0,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "down":
			if m.focused == 0 {
				m.themeList, cmd = m.themeList.Update(msg)
				cmds = append(cmds, cmd)
				// Update theme preview instantly
				m.updateThemeFromSelection()
			}
		case "tab":
			m.focused = (m.focused + 1) % 3
			if m.focused == 2 {
				m.apiKeyIn.Focus()
			} else {
				m.apiKeyIn.Blur()
			}
		case "shift+tab":
			m.focused = (m.focused - 1 + 3) % 3
			if m.focused == 2 {
				m.apiKeyIn.Focus()
			} else {
				m.apiKeyIn.Blur()
			}
		case "enter":
			if m.focused == 1 {
				m.cfg.DarkMode = !m.cfg.DarkMode
				styles.SetTheme(m.cfg.Theme, m.cfg.DarkMode)
			}
		case "esc":
			// Save config on exit
			m.cfg.GeminiAPIKey = m.apiKeyIn.Value()
			m.cfg.Save()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.themeList.SetSize(m.width-4, m.height-12)
	}

	if m.focused == 2 {
		m.apiKeyIn, cmd = m.apiKeyIn.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateThemeFromSelection() {
	if i, ok := m.themeList.SelectedItem().(item); ok {
		m.cfg.Theme = i.title
		styles.SetTheme(m.cfg.Theme, m.cfg.DarkMode)
	}
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(styles.Bold.Render("⚙️ SETUP MENU"))
	s.WriteString("\n\n")

	// 1. Theme List
	themeStyle := lipgloss.NewStyle().Padding(1).Border(lipgloss.RoundedBorder())
	if m.focused == 0 {
		themeStyle = themeStyle.BorderForeground(lipgloss.Color(styles.ColorAccent))
	} else {
		themeStyle = themeStyle.BorderForeground(lipgloss.Color(styles.ColorBorder))
	}
	s.WriteString(themeStyle.Render(m.themeList.View()))
	s.WriteString("\n")

	// 2. Dark Mode Toggle
	toggleStyle := lipgloss.NewStyle().Padding(0, 1).MarginTop(1)
	if m.focused == 1 {
		toggleStyle = toggleStyle.Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(styles.ColorAccent))
	}
	modeStr := "Dark"
	if !m.cfg.DarkMode {
		modeStr = "Light"
	}
	s.WriteString(toggleStyle.Render(fmt.Sprintf("Mode: [%s] (Press Enter to toggle)", modeStr)))
	s.WriteString("\n")

	// 3. API Key
	apiStyle := lipgloss.NewStyle().Padding(0, 1).MarginTop(1)
	if m.focused == 2 {
		apiStyle = apiStyle.Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(styles.ColorAccent))
	}
	s.WriteString(apiStyle.Render("API Key: " + m.apiKeyIn.View()))
	s.WriteString("\n\n")

	s.WriteString(styles.Dim.Render("Press TAB to switch, ESC to save and close"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s.String())
}

func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	m.themeList.SetSize(w-10, h-15)
	return m
}
