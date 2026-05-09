package filebrowser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/pkg/styles"
)

// ── State machine ─────────────────────────────────────────────────────────────

type fbState int

const (
	stateNormal fbState = iota
	stateConfirmDelete
	stateNewName // create file / create dir / rename
	stateSearch
)

// ── Entry ─────────────────────────────────────────────────────────────────────

type entry struct {
	info     os.FileInfo
	name     string
	fullPath string
}

func (e entry) isHidden() bool {
	return strings.HasPrefix(e.name, ".")
}

func (e entry) isDir() bool {
	return e.info.IsDir()
}

func (e entry) isExec() bool {
	return !e.isDir() && (e.info.Mode()&0o111 != 0)
}

func (e entry) icon() string {
	switch {
	case e.isDir():
		return "▶ "
	case e.isExec():
		return "✦ "
	default:
		return "  "
	}
}

func (e entry) sizeStr() string {
	if e.isDir() {
		return "     -"
	}
	sz := e.info.Size()
	switch {
	case sz < 1024:
		return fmt.Sprintf("%5dB", sz)
	case sz < 1024*1024:
		return fmt.Sprintf("%4dKB", sz/1024)
	case sz < 1024*1024*1024:
		return fmt.Sprintf("%4dMB", sz/1024/1024)
	default:
		return fmt.Sprintf("%4dGB", sz/1024/1024/1024)
	}
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the file browser panel.
type Model struct {
	width, height int

	cwd        string
	entries    []entry
	cursor     int
	showHidden bool

	state     fbState
	nameInput textinput.Model
	isDir     bool // true → creating a dir, false → creating a file
	renaming  bool // true → renaming existing entry

	clipboard    string
	clipboardCut bool

	search      string
	searchInput textinput.Model

	status string
	err    error
}

// New constructs the file browser starting in the working directory.
func New() Model {
	cwd, _ := os.Getwd()

	ni := textinput.New()
	ni.Placeholder = "name…"
	ni.CharLimit = 255

	si := textinput.New()
	si.Placeholder = "search…"
	si.CharLimit = 128

	m := Model{
		cwd:         cwd,
		showHidden:  false,
		nameInput:   ni,
		searchInput: si,
	}
	m.entries = m.loadEntries()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// SetSize is called by the root model on resize.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	return m
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state {
	case stateNewName:
		return m.updateNewName(msg)
	case stateSearch:
		return m.updateSearch(msg)
	case stateConfirmDelete:
		return m.updateConfirm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "enter", "right", "l":
			m = m.enterSelected()
		case "backspace", "left", "h":
			m = m.goUp()
		case "n":
			m.isDir = false
			m.renaming = false
			m.nameInput.SetValue("")
			m.nameInput.Placeholder = "new file name…"
			m.nameInput.Focus()
			m.state = stateNewName
		case "N":
			m.isDir = true
			m.renaming = false
			m.nameInput.SetValue("")
			m.nameInput.Placeholder = "new directory name…"
			m.nameInput.Focus()
			m.state = stateNewName
		case "r":
			if len(m.entries) > 0 {
				m.renaming = true
				m.nameInput.SetValue(m.entries[m.cursor].name)
				m.nameInput.Placeholder = "rename…"
				m.nameInput.Focus()
				m.state = stateNewName
			}
		case "d":
			if len(m.entries) > 0 {
				m.state = stateConfirmDelete
			}
		case "y":
			if len(m.entries) > 0 {
				m.clipboard = m.entries[m.cursor].fullPath
				m.clipboardCut = false
				m.status = "Copied: " + m.entries[m.cursor].name
			}
		case "x":
			if len(m.entries) > 0 {
				m.clipboard = m.entries[m.cursor].fullPath
				m.clipboardCut = true
				m.status = "Cut: " + m.entries[m.cursor].name
			}
		case "p":
			m = m.paste()
		case "o":
			if len(m.entries) > 0 {
				m = m.openFile(m.entries[m.cursor].fullPath)
			}
		case ".":
			m.showHidden = !m.showHidden
			m.entries = m.loadEntries()
			if m.cursor >= len(m.entries) {
				m.cursor = max(0, len(m.entries)-1)
			}
		case "/":
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			m.state = stateSearch
		case "~":
			if home, err := os.UserHomeDir(); err == nil {
				m.cwd = home
				m.entries = m.loadEntries()
				m.cursor = 0
			}
		}
	}
	_ = cmd
	return m, nil
}

func (m Model) updateNewName(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = stateNormal
			m.nameInput.Blur()
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			if name != "" {
				if m.renaming {
					m = m.doRename(name)
				} else {
					m = m.doCreate(name)
				}
			}
			m.state = stateNormal
			m.nameInput.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter":
			m.search = m.searchInput.Value()
			m.entries = m.loadEntries()
			m.cursor = 0
			m.state = stateNormal
			m.searchInput.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfirm(msg tea.Msg) (Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch strings.ToLower(key.String()) {
		case "y":
			m = m.doDelete()
			m.state = stateNormal
		case "n", "esc":
			m.state = stateNormal
			m.status = "Cancelled."
		}
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	// Path bar
	pathStyle := styles.FilePath.Width(m.width)
	b.WriteString(pathStyle.Render("  " + m.cwd))
	b.WriteString("\n")

	// Column headers
	b.WriteString(styles.Dim.Render(fmt.Sprintf("  %-*s %6s  %s\n",
		m.width-20, "Name", "Size", "Modified")))

	// File list
	visibleH := m.height - 5
	if visibleH < 1 {
		visibleH = 1
	}
	start, end := m.scrollWindow(visibleH)

	for i := start; i < end && i < len(m.entries); i++ {
		e := m.entries[i]
		nameW := m.width - 22
		if nameW < 8 {
			nameW = 8
		}
		name := e.icon() + e.name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		modTime := e.info.ModTime().Format("01/02 15:04")
		line := fmt.Sprintf("%-*s %s  %s", nameW, name, e.sizeStr(), modTime)

		var rendered string
		if i == m.cursor {
			rendered = styles.FileSelected.Width(m.width).Render(line)
		} else if e.isHidden() {
			rendered = styles.FileHidden.Render(line)
		} else if e.isDir() {
			rendered = styles.FileDir.Render(line)
		} else if e.isExec() {
			rendered = styles.FileExec.Render(line)
		} else {
			rendered = styles.FileRegular.Render(line)
		}
		b.WriteString(rendered + "\n")
	}

	// Status / prompt area
	b.WriteString("\n")
	switch m.state {
	case stateConfirmDelete:
		if len(m.entries) > 0 {
			msg := styles.FileConfirmDanger.Render(
				fmt.Sprintf("Delete '%s'? [y/N]", m.entries[m.cursor].name))
			b.WriteString(msg)
		}
	case stateNewName:
		label := "New file: "
		if m.isDir {
			label = "New dir:  "
		} else if m.renaming {
			label = "Rename:   "
		}
		b.WriteString(styles.FilePrompt.Render(label))
		b.WriteString(m.nameInput.View())
	case stateSearch:
		b.WriteString(styles.FilePrompt.Render("/"))
		b.WriteString(m.searchInput.View())
	default:
		if m.err != nil {
			b.WriteString(styles.StatusErr.Render("Error: " + m.err.Error()))
		} else if m.status != "" {
			b.WriteString(styles.StatusOk.Render(m.status))
		} else if m.clipboard != "" {
			action := "Copied"
			if m.clipboardCut {
				action = "Cut"
			}
			b.WriteString(styles.Dim.Render(action + ": " + filepath.Base(m.clipboard)))
		}
	}

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}

// ── File operations ───────────────────────────────────────────────────────────

func (m Model) loadEntries() []entry {
	infos, err := os.ReadDir(m.cwd)
	if err != nil {
		m.err = err
		return nil
	}

	var entries []entry
	for _, d := range infos {
		info, err := d.Info()
		if err != nil {
			continue
		}
		e := entry{
			info:     info,
			name:     d.Name(),
			fullPath: filepath.Join(m.cwd, d.Name()),
		}
		if e.isHidden() && !m.showHidden {
			continue
		}
		if m.search != "" && !strings.Contains(strings.ToLower(e.name), strings.ToLower(m.search)) {
			continue
		}
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir() != entries[j].isDir() {
			return entries[i].isDir()
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})
	return entries
}

func (m Model) enterSelected() Model {
	if len(m.entries) == 0 {
		return m
	}
	e := m.entries[m.cursor]
	if e.isDir() {
		m.cwd = e.fullPath
		m.entries = m.loadEntries()
		m.cursor = 0
		m.search = ""
		m.status = ""
		m.err = nil
	} else {
		m = m.openFile(e.fullPath)
	}
	return m
}

func (m Model) goUp() Model {
	parent := filepath.Dir(m.cwd)
	if parent == m.cwd {
		return m
	}
	prev := filepath.Base(m.cwd)
	m.cwd = parent
	m.entries = m.loadEntries()
	m.cursor = 0
	m.search = ""
	// Try to restore cursor to the folder we came from
	for i, e := range m.entries {
		if e.name == prev {
			m.cursor = i
			break
		}
	}
	return m
}

func (m Model) doCreate(name string) Model {
	target := filepath.Join(m.cwd, name)
	var err error
	if m.isDir {
		err = os.MkdirAll(target, 0o755)
	} else {
		f, e := os.Create(target)
		if e == nil {
			f.Close()
		}
		err = e
	}
	if err != nil {
		m.err = err
	} else {
		m.status = "Created: " + name
		m.err = nil
	}
	m.entries = m.loadEntries()
	return m
}

func (m Model) doRename(newName string) Model {
	if len(m.entries) == 0 {
		return m
	}
	src := m.entries[m.cursor].fullPath
	dst := filepath.Join(m.cwd, newName)
	if err := os.Rename(src, dst); err != nil {
		m.err = err
	} else {
		m.status = "Renamed to: " + newName
		m.err = nil
	}
	m.entries = m.loadEntries()
	return m
}

func (m Model) doDelete() Model {
	if len(m.entries) == 0 {
		return m
	}
	e := m.entries[m.cursor]
	var err error
	if e.isDir() {
		err = os.RemoveAll(e.fullPath)
	} else {
		err = os.Remove(e.fullPath)
	}
	if err != nil {
		m.err = err
	} else {
		m.status = "Deleted: " + e.name
		m.err = nil
		if m.cursor >= len(m.entries)-1 {
			m.cursor = max(0, m.cursor-1)
		}
	}
	m.entries = m.loadEntries()
	return m
}

func (m Model) paste() Model {
	if m.clipboard == "" {
		m.status = "Clipboard empty"
		return m
	}
	src := m.clipboard
	dst := filepath.Join(m.cwd, filepath.Base(src))
	if dst == src {
		m.status = "Already here"
		return m
	}
	if err := copyPath(src, dst); err != nil {
		m.err = err
		return m
	}
	if m.clipboardCut {
		_ = os.RemoveAll(src)
		m.clipboard = ""
	}
	m.status = "Pasted: " + filepath.Base(dst)
	m.err = nil
	m.entries = m.loadEntries()
	return m
}

func (m Model) openFile(path string) Model {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		m.err = err
	} else {
		m.status = "Opened: " + filepath.Base(path)
	}
	return m
}

// scrollWindow returns [start, end) indices to display within the visible height.
func (m Model) scrollWindow(height int) (int, int) {
	total := len(m.entries)
	if total == 0 {
		return 0, 0
	}
	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}
	end := start + height
	if end > total {
		end = total
	}
	return start, end
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// copyPath copies src to dst recursively (file or directory).
func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPath(
			filepath.Join(src, e.Name()),
			filepath.Join(dst, e.Name()),
		); err != nil {
			return err
		}
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
