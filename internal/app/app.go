// Package app contains the root Bubble Tea model that composes the file browser,
// Gemini chat, and todo list panels into a single cohesive interface.
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/internal/chat"
	"github.com/faran17/kraken-tui/internal/config"
	"github.com/faran17/kraken-tui/internal/filebrowser"
	"github.com/faran17/kraken-tui/internal/help"
	"github.com/faran17/kraken-tui/internal/setup"
	"github.com/faran17/kraken-tui/internal/terminal"
	"github.com/faran17/kraken-tui/internal/todo"
	"github.com/faran17/kraken-tui/pkg/styles"
)

// ── Constants ─────────────────────────────────────────────────────────────────

// Panel indices for routing keyboard events and drawing active borders.
const (
	PanelFiles    = iota // index 0: File Browser
	PanelChat            // index 1: Gemini Chat
	PanelTodo            // index 2: Todo List
	PanelTerminal        // index 3: Terminal
	panelCount           // total number of panels (4)
)

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the root (compositor) Bubble Tea model. It owns the three child panels
// and handles global hotkeys (like Tab and Ctrl+C).
type Model struct {
	width  int // terminal width
	height int // terminal height

	activePanel int // which panel currently receives key events

	// The child components
	files filebrowser.Model
	chat  chat.Model
	todo  todo.Model
	term  terminal.Model
	setup setup.Model
	help  help.Model

	// Layout proportions
	panelWidths [3]float64 // Width percentages for the three top panels
	termHeight  int        // Fixed line height for the terminal panel

	// Global UI elements
	spinner   spinner.Model // loading spinner used before first render
	ready     bool          // becomes true after receiving the first WindowSizeMsg
	showSetup bool          // toggle setup menu overlay
	showHelp  bool          // toggle help overlay

	// Persistent configuration
	cfg *config.Config

	// Terminal size state
	termSize int // 0: Compact, 1: Half, 2: Full
}

// New constructs the root model. It initializes the three child panels and
// passes settings from the config through.
func New(cfg *config.Config) (Model, error) {
	// A generic loading spinner used if startup takes a moment.
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.ChatSpinner

	// Initialize the chat panel. It handles its own persistence and API setup.
	chatModel, err := chat.New(cfg.GeminiAPIKey)
	if err != nil {
		return Model{}, fmt.Errorf("chat init: %w", err)
	}

	// Initialize the todo panel. It loads items from disk immediately.
	todoModel, err := todo.New()
	if err != nil {
		return Model{}, fmt.Errorf("todo init: %w", err)
	}

	// Initialize the terminal panel.
	termModel := terminal.New()

	return Model{
		activePanel: PanelFiles, // start with the file browser focused
		files:       filebrowser.New(),
		chat:        chatModel,
		todo:        todoModel,
		term:        termModel,
		setup:       setup.New(cfg),
		help:        help.New(),
		spinner:     s,
		panelWidths: [3]float64{cfg.PanelWidths[0], cfg.PanelWidths[1], cfg.PanelWidths[2]},
		termHeight:  cfg.TermHeight,
		cfg:         cfg,
	}, nil
}

// ── Tea interface ─────────────────────────────────────────────────────────────

// Init returns a batch of all startup commands needed by the child panels.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick, // start the global loading spinner
		m.chat.Init(),  // start cursor blink in chat input
		m.todo.Init(),  // (currently no-op)
		m.files.Init(), // (currently no-op)
		m.term.Init(),  // start terminal
	)
}

// Update handles global key presses and routes all other messages to the
// currently active child panel.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// ── Setup/Help Overlay Handlers ──────────────────────────────────────────
	if m.showSetup {
		var cmd tea.Cmd
		m.setup, cmd = m.setup.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "esc" || key.String() == "ctrl+s") {
			m.showSetup = false
		}
		return m, cmd
	}
	if m.showHelp {
		var cmd tea.Cmd
		m.help, cmd = m.help.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "esc" || key.String() == "ctrl+h") {
			m.showHelp = false
		}
		return m, cmd
	}

	switch msg := msg.(type) {

	// ── Keyboard events ──────────────────────────────────────────────────────
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			// Global quit sequence
			return m, tea.Quit

		case "ctrl+s":
			m.showSetup = true
			return m, nil

		case "ctrl+h":
			m.showHelp = true
			return m, nil

		case "tab", "shift+tab":
			if msg.String() == "tab" {
				m.activePanel = (m.activePanel + 1) % panelCount
			} else {
				m.activePanel = (m.activePanel - 1 + panelCount) % panelCount
			}
			return m, nil

		case "shift+left":
			m.resizeHorizontal(-0.05)
			m.applySize()
			return m, nil

		case "shift+right":
			m.resizeHorizontal(0.05)
			m.applySize()
			return m, nil

		case "ctrl+t":
			// Cycle terminal size: Compact -> Half -> Full
			m.termSize = (m.termSize + 1) % 3
			m.applySize()
			return m, nil
		}

	// ── Terminal resize events ───────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.applySize()
		return m, nil

	// ── Spinner ticks ────────────────────────────────────────────────────────
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// ── Routing ──────────────────────────────────────────────────────────────
	// If the message wasn't consumed globally (like Tab or Ctrl+C), route it
	// to the active child panel.
	switch m.activePanel {
	case PanelFiles:
		newFiles, cmd := m.files.Update(msg)
		m.files = newFiles
		cmds = append(cmds, cmd)

	case PanelChat:
		newChat, cmd := m.chat.Update(msg)
		m.chat = newChat
		cmds = append(cmds, cmd)

	case PanelTodo:
		newTodo, cmd := m.todo.Update(msg)
		m.todo = newTodo
		cmds = append(cmds, cmd)

	case PanelTerminal:
		newTerm, cmd := m.term.Update(msg)
		m.term = newTerm
		cmds = append(cmds, cmd)
	}

	// Because child updates might return cmds (like file browser search keystrokes
	// or chat token streaming), we return a batch of all accumulated commands.
	return m, tea.Batch(cmds...)
}

// View constructs the final string drawn to the terminal.
func (m Model) View() string {
	if !m.ready {
		// Terminal hasn't provided a WindowSizeMsg yet; show loading.
		return "\n  " + m.spinner.View() + "  Loading Kraken…"
	}

	widths, h := m.calculateDimensions()

	// Render each panel as a distinct block of text.
	filesPanel := m.renderPanel(PanelFiles, "󰉋  Files", m.files.View(), widths[0], h)
	chatPanel := m.renderPanel(PanelChat, "  Gemini", m.chat.View(), widths[1], h)
	todoPanel := m.renderPanel(PanelTodo, "  Tasks", m.todo.View(), widths[2], h)

	// Join the three main panels horizontally side-by-side.
	mainBody := lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, chatPanel, todoPanel)

	// Render the bottom terminal panel.
	termPanel := m.renderPanel(PanelTerminal, "  Terminal", m.term.View(), m.width-2, m.termHeight)

	// Render the top and bottom bars.
	header := m.renderHeader()
	status := m.renderStatus(m.width)

	// Join everything vertically.
	view := lipgloss.JoinVertical(lipgloss.Left, header, mainBody, termPanel, status)

	if m.showSetup {
		overlay := m.setup.View()
		return m.placeOverlay(view, overlay)
	}

	if m.showHelp {
		overlay := m.help.View()
		return m.placeOverlay(view, overlay)
	}

	return view
}

func (m Model) placeOverlay(base, overlay string) string {
	// Simple overlay placement
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceBackground(lipgloss.Color(styles.ColorBg)))
}

// ── Rendering helpers ─────────────────────────────────────────────────────────

// renderPanel wraps a child's raw view output in a stylized border.
func (m Model) renderPanel(idx int, title, content string, w, h int) string {
	active := m.activePanel == idx
	titleStr := styles.PanelTitle(active).Render(title)

	// Join the title text with the main content text.
	inner := lipgloss.JoinVertical(lipgloss.Left, titleStr, content)

	// Apply the bright orange border if active, otherwise dim gray.
	if active {
		return styles.PanelActive(w, h).Render(inner)
	}
	return styles.PanelInactive(w, h).Render(inner)
}

// renderHeader draws the top bar containing the app title and the panel tabs.
func (m Model) renderHeader() string {
	logo := styles.AppTitle.Render("🐙 KRAKEN")
	sub := styles.Dim.Render(" — AI · Files · Tasks")

	tabs := []string{"[Files]", "[Chat]", "[Tasks]"}
	var tabStr strings.Builder
	for i, t := range tabs {
		// Highlight the tab corresponding to the active panel
		if i == m.activePanel {
			tabStr.WriteString(styles.ChatSessionTabActive.Render(t))
		} else {
			tabStr.WriteString(styles.ChatSessionTab.Render(t))
		}
	}

	left := logo + sub
	right := tabStr.String()

	// Calculate spacing needed to push the tabs to the right edge
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	line := left + strings.Repeat(" ", gap) + right

	return styles.StatusBar.Width(m.width).Render(line)
}

// renderStatus draws the bottom bar displaying context-aware keybindings.
func (m Model) renderStatus(width int) string {
	// Base commands always available
	pills := []string{
		styles.HelpPill("Tab", "switch panel"),
		styles.HelpPill("^S", "setup"),
		styles.HelpPill("^T", "term size"),
		styles.HelpPill("^H", "help"),
		styles.HelpPill("^C", "quit"),
	}

	// Panel-specific commands
	switch m.activePanel {
	case PanelFiles:
		pills = append(pills,
			styles.HelpPill("↑↓", "navigate"),
			styles.HelpPill("Enter", "open"),
			styles.HelpPill("n", "new file"),
			styles.HelpPill("N", "new dir"),
			styles.HelpPill("r", "rename"),
			styles.HelpPill("d", "delete"),
			styles.HelpPill("y/p", "copy/paste"),
			styles.HelpPill("x", "cut"),
			styles.HelpPill(".", "hidden"),
		)
	case PanelChat:
		pills = append(pills,
			styles.HelpPill("Enter", "send"),
			styles.HelpPill("Alt+N", "new session"),
			styles.HelpPill("Alt+←/→", "sessions"),
		)
	case PanelTodo:
		pills = append(pills,
			styles.HelpPill("n", "new task"),
			styles.HelpPill("Space", "toggle"),
			styles.HelpPill("d", "delete"),
		)
	case PanelTerminal:
		pills = append(pills,
			styles.HelpPill("Enter", "run command"),
			styles.HelpPill("PgUp/PgDn", "scroll history"),
		)
	}

	bar := strings.Join(pills, " ")
	return styles.StatusBar.Width(width).Render(bar)
}

// ── Layout Engine ─────────────────────────────────────────────────────────────

func (m *Model) resizeHorizontal(delta float64) {
	if m.activePanel >= 3 {
		return // Cannot horizontally resize terminal
	}

	target := m.activePanel
	neighbor := m.activePanel + 1
	if target == 2 {
		neighbor = target - 1
	}

	// Apply constraint (min 10% width)
	if m.panelWidths[target]+delta < 0.10 {
		return
	}
	if m.panelWidths[neighbor]-delta < 0.10 {
		return
	}

	m.panelWidths[target] += delta
	m.panelWidths[neighbor] -= delta
}

func (m *Model) applySize() {
	if !m.ready {
		return
	}

	// Calculate termHeight based on preset
	switch m.termSize {
	case 1: // Half
		m.termHeight = m.height / 2
	case 2: // Full
		m.termHeight = m.height - 6
	default: // Compact
		m.termHeight = 12
	}

	widths, h := m.calculateDimensions()
	m.files = m.files.SetSize(widths[0], h)
	m.chat = m.chat.SetSize(widths[1], h)
	m.todo = m.todo.SetSize(widths[2], h)
	m.term = m.term.SetSize(m.width, m.termHeight)
	m.setup = m.setup.SetSize(m.width, m.height)
	m.help = m.help.SetSize(m.width, m.height)
}

func (m Model) calculateDimensions() ([3]int, int) {
	headerH := 1
	statusH := 1
	termH := m.termHeight + 2 // include borders

	panelH := m.height - headerH - statusH - termH - 2
	if panelH < 5 {
		panelH = 5
	}

	var widths [3]int
	totalW := m.width

	for i := 0; i < 3; i++ {
		w := int(float64(totalW)*m.panelWidths[i]) - 2 // -2 for borders
		if w < 10 {
			w = 10
		}
		widths[i] = w
	}
	return widths, panelH
}
