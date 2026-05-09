// Package todo implements the Todo List panel for Kraken TUI.
// It provides a keyboard-driven interface to manage tasks with features like
// adding, toggling, deleting, reordering, and automatic JSON persistence.
package todo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/pkg/styles"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// todoState controls which mode the panel is in, determining how
// keyboard events are interpreted.
type todoState int

const (
	todoNormal todoState = iota // default browsing/reordering mode
	todoAdding                  // text input active: creating a new task
)

// Item represents a single task in the todo list.
// It is serialized directly to JSON.
type Item struct {
	ID        string    `json:"id"`         // unique identifier (nanosecond timestamp)
	Text      string    `json:"text"`       // task description
	Done      bool      `json:"done"`       // completion status
	CreatedAt time.Time `json:"created_at"` // when the task was added
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the Bubble Tea model for the todo list panel.
type Model struct {
	width, height int // current panel dimensions in terminal cells

	items  []Item    // all tasks, in display order
	cursor int       // index of the highlighted task
	state  todoState // current input mode

	input    textinput.Model // text input used for creating new tasks
	dataPath string          // absolute path to the ~/.kraken/todos.json file

	// Feedback shown at the bottom of the panel.
	status string // last successful action description
	err    error  // last error; shown in red
}

// New constructs a Model, ensures the ~/.kraken directory exists,
// and loads any previously saved tasks from disk.
func New() (Model, error) {
	// Resolve a platform-appropriate data directory (~/.kraken on all platforms).
	home, err := os.UserHomeDir()
	if err != nil {
		home = "." // fallback to current directory
	}
	dataDir := filepath.Join(home, ".kraken")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Model{}, fmt.Errorf("create data dir: %w", err)
	}
	dataPath := filepath.Join(dataDir, "todos.json")

	// Configure the text input for adding new tasks.
	ti := textinput.New()
	ti.Placeholder = "New task…"
	ti.CharLimit = 256

	m := Model{
		input:    ti,
		dataPath: dataPath,
	}

	// Load previously saved tasks immediately.
	m.items = m.loadItems()
	return m, nil
}

// Init satisfies the tea.Model interface. No startup commands are needed.
func (m Model) Init() tea.Cmd { return nil }

// SetSize is called by the root model when the terminal is resized.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	return m
}

// ── Update ────────────────────────────────────────────────────────────────────

// Update routes incoming messages to the appropriate sub-handler based on
// the current state machine state.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Sub-state priority: input field captures keys while active.
	if m.state == todoAdding {
		return m.updateAdding(msg)
	}

	// Normal browsing/editing mode.
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {

		// ── Navigation ───────────────────────────────────────────────────────
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "g":
			// Jump to top
			m.cursor = 0
		case "G":
			// Jump to bottom
			m.cursor = len(m.items) - 1

		// ── Actions ──────────────────────────────────────────────────────────
		case "n":
			// Enter input mode to add a new task
			m.input.SetValue("")
			m.input.Focus()
			m.state = todoAdding

		case " ":
			// Toggle completion status of the selected task
			if len(m.items) > 0 {
				m.items[m.cursor].Done = !m.items[m.cursor].Done
				m.saveItems() // persist immediately
				if m.items[m.cursor].Done {
					m.status = "✓ Done!"
				} else {
					m.status = "○ Reopened"
				}
			}

		case "d", "x":
			// Delete the selected task
			if len(m.items) > 0 {
				name := m.items[m.cursor].Text
				// Remove element by slicing around it
				m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
				// Keep cursor in bounds
				if m.cursor >= len(m.items) && m.cursor > 0 {
					m.cursor--
				}
				m.saveItems() // persist immediately
				m.status = "Deleted: " + truncate(name, 24)
			}

		// ── Reordering ───────────────────────────────────────────────────────
		case "J":
			// Move item down in the list
			if m.cursor < len(m.items)-1 {
				m.items[m.cursor], m.items[m.cursor+1] = m.items[m.cursor+1], m.items[m.cursor]
				m.cursor++
				m.saveItems()
			}

		case "K":
			// Move item up in the list
			if m.cursor > 0 {
				m.items[m.cursor], m.items[m.cursor-1] = m.items[m.cursor-1], m.items[m.cursor]
				m.cursor--
				m.saveItems()
			}
		}
	}
	return m, nil
}

// updateAdding handles key events while the new-task text input is active.
func (m Model) updateAdding(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel without adding
			m.state = todoNormal
			m.input.Blur()
			return m, nil

		case "enter":
			// Commit the text and create the task
			text := strings.TrimSpace(m.input.Value())
			if text != "" {
				item := Item{
					ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
					Text:      text,
					Done:      false,
					CreatedAt: time.Now(),
				}
				m.items = append(m.items, item)
				m.cursor = len(m.items) - 1 // automatically select the new item
				m.saveItems()
				m.status = "Added: " + truncate(text, 24)
			}
			m.state = todoNormal
			m.input.Blur()
			return m, nil
		}
	}

	// Forward all other keys to the text input widget.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the todo list: summary line -> tasks -> input/status area.
func (m Model) View() string {
	if m.width == 0 {
		return "" // avoid rendering until sized
	}

	// Calculate completion stats for the header
	done := 0
	for _, it := range m.items {
		if it.Done {
			done++
		}
	}
	countLine := styles.TodoCount.Render(fmt.Sprintf("  %d/%d complete", done, len(m.items)))

	var b strings.Builder
	b.WriteString(countLine + "\n\n")

	// Empty state
	if len(m.items) == 0 && m.state != todoAdding {
		b.WriteString(styles.Dim.Render("  No tasks yet — press 'n' to add one\n"))
	}

	// ── Task List (with scroll window) ────────────────────────────────────────
	// Reserve rows for summary (3) and input/status area (2).
	visibleH := m.height - 5
	if visibleH < 1 {
		visibleH = 1
	}
	start, end := m.scrollWindow(visibleH)

	for i := start; i < end && i < len(m.items); i++ {
		item := m.items[i]

		// Text truncation width
		w := m.width - 6
		if w < 10 {
			w = 10
		}

		// Unicode checkbox icon
		check := styles.TodoCheckPending.Render("○")
		if item.Done {
			check = styles.TodoCheckDone.Render("✓")
		}

		text := truncate(item.Text, w)

		var line string
		if i == m.cursor {
			// Selected items get a distinct background and color, overriding done state
			content := check + " " + text
			line = styles.TodoSelected.Width(m.width - 2).Render(content)
		} else if item.Done {
			// Unselected completed items are struck through and dimmed
			line = "  " + check + " " + styles.TodoDone.Render(text)
		} else {
			// Unselected pending items
			line = "  " + check + " " + styles.TodoPending.Render(text)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")

	// ── Status / Input area ───────────────────────────────────────────────────
	switch m.state {
	case todoAdding:
		b.WriteString(styles.TodoInput.Render("+ "))
		b.WriteString(m.input.View())
	default:
		// Show error or success message
		if m.err != nil {
			b.WriteString(styles.StatusErr.Render("Error: " + m.err.Error()))
		} else if m.status != "" {
			b.WriteString(styles.StatusOk.Render(m.status))
		}
	}

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}

// ── Persistence ───────────────────────────────────────────────────────────────

// loadItems reads tasks from ~/.kraken/todos.json. Returns nil if the file
// doesn't exist or is corrupt.
func (m Model) loadItems() []Item {
	data, err := os.ReadFile(m.dataPath)
	if err != nil {
		return nil
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

// saveItems writes the current tasks to disk as pretty-printed JSON.
// Errors are ignored to avoid disrupting the UI.
func (m Model) saveItems() {
	data, err := json.MarshalIndent(m.items, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.dataPath, data, 0o644)
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// scrollWindow calculates the [start, end) index slice of items that should be
// drawn to the screen to keep the cursor row visible.
func (m Model) scrollWindow(height int) (int, int) {
	total := len(m.items)
	if total == 0 {
		return 0, 0
	}
	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1 // scroll down so cursor stays on screen
	}
	end := start + height
	if end > total {
		end = total
	}
	return start, end
}

// truncate shortens a string to max characters, appending an ellipsis if needed.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
