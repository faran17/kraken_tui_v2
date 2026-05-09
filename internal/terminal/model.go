// Package terminal implements a command-runner panel for Kraken TUI.
// It allows users to execute shell commands and see their output in a scrollable viewport.
package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/pkg/styles"
)

// ── Messages ──────────────────────────────────────────────────────────────────

// commandFinishedMsg is sent when an async shell command completes.
type commandFinishedMsg struct {
	cmd    string
	output string
	err    error
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the Bubble Tea model for the terminal command runner panel.
type Model struct {
	width, height int

	input    textinput.Model // where the user types commands
	viewport viewport.Model  // where the output is displayed

	running bool     // true if a command is currently executing
	history []string // history of executed commands and their output
	cwd     string   // the current working directory for the terminal session
}

// New constructs a Model for the terminal panel.
func New() Model {
	cwd, _ := os.Getwd()

	ti := textinput.New()
	ti.Placeholder = "Enter command..."
	ti.Prompt = "❯ "
	ti.CharLimit = 1024
	ti.Focus() // Must be focused to receive keyboard events!

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	return Model{
		input:    ti,
		viewport: vp,
		history:  make([]string, 0),
		cwd:      cwd,
	}
}

// Init satisfies the tea.Model interface.
func (m Model) Init() tea.Cmd {
	return nil
}

// SetSize is called by the root model when the terminal is resized.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h

	// Allocate space: viewport takes everything except 2 lines (1 for border, 1 for input)
	vpHeight := h - 3
	if vpHeight < 1 {
		vpHeight = 1
	}

	m.viewport.Width = w - 2 // -2 for margins
	m.viewport.Height = vpHeight

	m.input.Width = w - 4 // -4 for prompt and margins

	// Re-render viewport content to fit new width
	m.updateViewport()

	return m
}

// ── Update ────────────────────────────────────────────────────────────────────

// Update handles key presses (scrolling, typing) and async command results.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if !m.running {
				v := strings.TrimSpace(m.input.Value())
				if v != "" {
					m.running = true
					m.history = append(m.history, lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("❯ "+v))
					m.input.SetValue("")

					// Intercept 'cd' commands
					if strings.HasPrefix(v, "cd ") || v == "cd" {
						dir := strings.TrimSpace(strings.TrimPrefix(v, "cd"))
						if dir == "" || dir == "~" {
							dir, _ = os.UserHomeDir()
						} else if strings.HasPrefix(dir, "~/") {
							home, _ := os.UserHomeDir()
							dir = filepath.Join(home, strings.TrimPrefix(dir, "~/"))
						}
						// handle absolute vs relative paths
						if !filepath.IsAbs(dir) {
							dir = filepath.Join(m.cwd, dir)
						}
						dir = filepath.Clean(dir)

						// Check if dir exists
						if info, err := os.Stat(dir); err == nil && info.IsDir() {
							m.cwd = dir
						} else {
							m.history = append(m.history, fmt.Sprintf("cd: %s: No such file or directory", dir))
						}
						m.running = false
						m.updateViewport()
						return m, nil
					}

					m.updateViewport()
					cmds = append(cmds, m.runCommand(v, m.cwd))
				}
			}
		}

	case commandFinishedMsg:
		m.running = false
		if msg.err != nil {
			m.history = append(m.history, styles.StatusErr.Render(msg.err.Error()))
		}
		if msg.output != "" {
			m.history = append(m.history, msg.output)
		}
		m.updateViewport()
		return m, nil
	}

	// Always handle viewport scrolling first if possible.
	// We route up/down arrows to viewport if input is empty, otherwise to input?
	// Actually, just route PageUp/PageDown to viewport.
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "pgup", "pgdown", "up", "down":
			// If input is empty, or explicitly pg keys, scroll the viewport.
			if key.String() == "pgup" || key.String() == "pgdown" || (m.input.Value() == "" && (key.String() == "up" || key.String() == "down")) {
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		}
	}

	// Forward remaining keys to the text input.
	if !m.running {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	// Build the main display block.
	var b strings.Builder

	// The viewport showing history
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Divider
	b.WriteString(styles.Dim.Render(strings.Repeat("─", m.width-2)) + "\n")

	// The input line
	if m.running {
		b.WriteString(styles.Dim.Render(" Running..."))
	} else {
		b.WriteString(" " + m.input.View())
	}

	return b.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *Model) updateViewport() {
	m.viewport.SetContent(strings.Join(m.history, "\n"))
	m.viewport.GotoBottom()
}

// runCommand returns a tea.Cmd that executes the shell command in the background.
func (m Model) runCommand(command, cwd string) tea.Cmd {
	return func() tea.Msg {
		var c *exec.Cmd
		if runtime.GOOS == "windows" {
			c = exec.Command("cmd", "/C", command)
		} else {
			c = exec.Command("sh", "-c", command)
		}
		c.Dir = cwd

		out, err := c.CombinedOutput()

		return commandFinishedMsg{
			cmd:    command,
			output: string(out),
			err:    err,
		}
	}
}

// Focus enables the input field.
func (m Model) Focus() Model {
	m.input.Focus()
	return m
}

// Blur disables the input field.
func (m Model) Blur() Model {
	m.input.Blur()
	return m
}
