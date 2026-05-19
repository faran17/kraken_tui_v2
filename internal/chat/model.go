// Package chat implements the Gemini AI chat panel for Kraken TUI.
// It manages multi-turn conversations with the Gemini 2.0 Flash model,
// streams responses token-by-token to keep the UI responsive, and
// persists up to 3 chat sessions across restarts.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faran17/kraken-tui/pkg/styles"
	"google.golang.org/genai"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	// maxSessions caps how many sessions are kept in memory and on disk.
	maxSessions = 3

	// modelName is the Gemini model used for all requests.
	modelName = "gemini-2.0-flash"
)

// ── Persistence types ─────────────────────────────────────────────────────────

// Message represents a single turn in a conversation (user or model).
type Message struct {
	Role    string    `json:"role"`    // "user" or "model"
	Content string    `json:"content"` // plaintext content of the turn
	Time    time.Time `json:"time"`    // when the message was sent/received
}

// Session holds all messages for one conversation.
type Session struct {
	ID        string    `json:"id"`         // unique identifier (nanosecond timestamp)
	Title     string    `json:"title"`      // first user message, truncated
	CreatedAt time.Time `json:"created_at"` // session creation time
	Messages  []Message `json:"messages"`   // ordered list of turns
}

// history is the top-level JSON structure persisted to disk.
type history struct {
	Sessions []Session `json:"sessions"`
}

// ── Bubble Tea internal message types ────────────────────────────────────────
// These are sent between goroutines and the Bubble Tea event loop.

// streamTokenMsg carries one incremental text chunk from the Gemini stream.
type streamTokenMsg struct{ token string }

// streamDoneMsg signals that the stream finished; carries the full response.
type streamDoneMsg struct{ fullResponse string }

// streamErrMsg carries any error that terminated the stream unexpectedly.
type streamErrMsg struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the Bubble Tea model for the Gemini AI chat panel.
// It owns all UI state (viewport, input box, spinner) and all chat state
// (sessions, streaming channel, Gemini client).
type Model struct {
	width, height int // current terminal dimensions for this panel

	// Gemini client (nil when GEMINI_API_KEY is not set)
	apiKey string
	client *genai.Client

	// sessions holds up to maxSessions conversations; activeSession is the index.
	sessions      []Session
	activeSession int

	// Streaming state ─────────────────────────────────────────────────────────
	// streaming is true while a Gemini response is in flight.
	streaming bool
	// streamResponse accumulates tokens as they arrive so we can show a live preview.
	streamResponse string
	// tokenChan is the buffered channel through which the goroutine delivers tokens.
	// A new channel is created for every send to avoid cross-contamination.
	tokenChan chan string
	// ctx / cancelCtx allow cancelling an in-flight request (e.g. when a new
	// message is sent before the previous response has finished).
	ctx       context.Context
	cancelCtx context.CancelFunc

	// UI components
	viewport      viewport.Model  // scrollable message history
	input         textarea.Model  // multi-line user input box
	apiKeyInput   textinput.Model // single-line input for the API key
	spinner       spinner.Model   // animated spinner shown while streaming
	viewportReady bool            // true after the first WindowSizeMsg

	// Feedback fields
	status   string // informational text shown in the header
	err      error  // last error; shown in red
	dataPath string // path to chat_history.json
}

// New constructs the chat Model. apiKey may be empty; in that case the panel
// will render a warning but won't crash.
func New(apiKey string) (Model, error) {
	// Resolve a platform-appropriate data directory (~/.kraken on all platforms).
	home, err := os.UserHomeDir()
	if err != nil {
		home = "." // fallback to current directory
	}
	dataDir := filepath.Join(home, ".kraken")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Model{}, fmt.Errorf("create data dir: %w", err)
	}
	dataPath := filepath.Join(dataDir, "chat_history.json")
	configPath := filepath.Join(dataDir, "config.json")

	// Try loading API key from local config if not provided
	if apiKey == "" {
		if data, err := os.ReadFile(configPath); err == nil {
			var cfg map[string]string
			if err := json.Unmarshal(data, &cfg); err == nil && cfg["api_key"] != "" {
				apiKey = cfg["api_key"]
			}
		}
	}

	// Configure the multi-line text input where the user types messages.
	ta := textarea.New()
	ta.Placeholder = "Ask Gemini anything… (Enter to send)"
	ta.CharLimit = 4096
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	// Input for API Key
	aki := textinput.New()
	aki.Placeholder = "Paste GEMINI_API_KEY here..."
	aki.EchoMode = textinput.EchoPassword
	aki.EchoCharacter = '•'
	aki.Width = 50

	if apiKey == "" {
		aki.Focus()
	} else {
		ta.Focus()
	}

	// Spinner shown next to "Generating…" while waiting for the first token.
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.ChatSpinner

	// A root context for all API calls; individual sends use derived contexts.
	ctx, cancel := context.WithCancel(context.Background())

	m := Model{
		apiKey:      apiKey,
		dataPath:    dataPath,
		input:       ta,
		apiKeyInput: aki,
		spinner:     sp,
		ctx:         ctx,
		cancelCtx:   cancel,
		tokenChan:   make(chan string, 512), // buffered to avoid blocking the goroutine
	}

	// Load persisted history; if none, start with a blank session.
	m.sessions = m.loadHistory()
	if len(m.sessions) == 0 {
		m.sessions = []Session{newSession()}
	}
	m.activeSession = len(m.sessions) - 1 // always open the most recent session

	// Initialize the Gemini client only if a key was provided.
	if apiKey != "" {
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			m.err = fmt.Errorf("gemini init: %w", err)
		} else {
			m.client = client
		}
	}

	return m, nil
}

// Init returns the startup commands: textarea cursor blink and spinner tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, textinput.Blink, m.spinner.Tick)
}

// SetSize is called by the root model every time the terminal is resized.
// It recalculates panel dimensions and re-flows the viewport content.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h

	// Reserve rows for session tabs (1), status line (1), and input area (4).
	inputH := 4
	headerH := 2
	vpH := h - inputH - headerH
	if vpH < 3 {
		vpH = 3
	}

	if !m.viewportReady {
		// First resize: create the viewport.
		m.viewport = viewport.New(w, vpH)
		m.viewportReady = true
	} else {
		// Subsequent resizes: just update dimensions.
		m.viewport.Width = w
		m.viewport.Height = vpH
	}

	m.input.SetWidth(w - 2)
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m
}

// ── Update ────────────────────────────────────────────────────────────────────

// Update handles all Bubble Tea messages routed to the chat panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// If the client isn't initialized, we only process API key input.
	if m.client == nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				key := strings.TrimSpace(m.apiKeyInput.Value())
				if key != "" {
					client, err := genai.NewClient(m.ctx, &genai.ClientConfig{
						APIKey:  key,
						Backend: genai.BackendGeminiAPI,
					})
					if err == nil {
						m.client = client
						m.apiKey = key
						m.err = nil
						// Save the key locally
						cfgPath := filepath.Join(filepath.Dir(m.dataPath), "config.json")
						data, _ := json.MarshalIndent(map[string]string{"api_key": key}, "", "  ")
						os.WriteFile(cfgPath, data, 0o600)

						m.apiKeyInput.Blur()
						m.input.Focus()
					} else {
						m.err = err
					}
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {

	// A new token arrived from the Gemini stream.
	case streamTokenMsg:
		if msg.token != "" {
			m.streamResponse += msg.token
		}
		// Show the accumulating response as a live preview below the history.
		m.viewport.SetContent(
			m.renderMessages() + "\n" +
				styles.ChatAIBubble.Width(m.width-2).Render("🐙 "+m.streamResponse),
		)
		m.viewport.GotoBottom()
		// Chain: schedule reading the next token from the channel.
		cmds = append(cmds, m.waitForToken())

	// The stream finished successfully.
	case streamDoneMsg:
		m.streaming = false
		if msg.fullResponse != "" {
			// Commit the completed response to the current session.
			sess := m.sessions[m.activeSession]
			sess.Messages = append(sess.Messages, Message{
				Role:    "model",
				Content: msg.fullResponse,
				Time:    time.Now(),
			})
			m.sessions[m.activeSession] = sess
		}
		m.streamResponse = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.saveHistory() // persist immediately so no data is lost on crash
		m.status = ""

	// An error terminated the stream early.
	case streamErrMsg:
		m.streaming = false
		m.err = msg.err
		m.streamResponse = ""

	// Keep the spinner animated while streaming.
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	// Keyboard input handling.
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Send the message only when not already streaming.
			if !m.streaming {
				cmds = append(cmds, m.sendMessage())
			}
		case "ctrl+k":
			// Disconnect the client to trigger the API key setup screen.
			m.client = nil
			m.apiKeyInput.SetValue(m.apiKey)
			m.apiKeyInput.Focus()
			m.err = nil
		case "alt+n":
			// Open a brand-new session.
			m = m.newSession()
		case "alt+x":
			m = m.deleteActiveSession()
		case "alt+left":
			// Switch to the previous session.
			if m.activeSession > 0 {
				m.activeSession--
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
			}
		case "alt+right":
			// Switch to the next session.
			if m.activeSession < len(m.sessions)-1 {
				m.activeSession++
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
			}
		case "pgup":
			m.viewport.HalfViewUp()
		case "pgdown":
			m.viewport.HalfViewDown()
		default:
			// Forward all other keys to the textarea while not streaming.
			if !m.streaming {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	default:
		// Bubble Tea internal messages (cursor blink, etc.) go to the textarea.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the chat panel: session tabs → status line → message viewport → input box.
func (m Model) View() string {
	if !m.viewportReady {
		return styles.Dim.Render("  Initialising…")
	}

	// ── Setup View (if no API Key) ────────────────────────────────────────────
	if m.client == nil {
		header := styles.ChatSystemMsg.Render("Welcome to Gemini 2.0 Flash!")
		sub := styles.Dim.Render("An API key is required to start chatting.")
		errStr := ""
		if m.err != nil {
			errStr = "\n" + styles.StatusErr.Render("Error: "+m.err.Error())
		}
		if m.apiKey != "" && m.err == nil {
			errStr = "\n" + styles.Dim.Render("(Press Enter to save your new key)")
		}

		box := lipgloss.JoinVertical(lipgloss.Center,
			header,
			sub,
			"",
			m.apiKeyInput.View(),
			errStr,
		)

		// Center the box vertically and horizontally
		padTop := (m.height - lipgloss.Height(box)) / 2
		if padTop < 0 {
			padTop = 0
		}

		return strings.Repeat("\n", padTop) + lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box)
	}

	// ── Session tab bar ───────────────────────────────────────────────────────
	var tabs strings.Builder
	for i, s := range m.sessions {
		label := fmt.Sprintf(" %d:%s ", i+1, truncate(s.Title, 12))
		if i == m.activeSession {
			tabs.WriteString(styles.ChatSessionTabActive.Render(label))
		} else {
			tabs.WriteString(styles.ChatSessionTab.Render(label))
		}
	}

	// ── Status / error line ───────────────────────────────────────────────────
	var statusLine string
	switch {
	case m.err != nil:
		statusLine = styles.StatusErr.Render("Error: " + m.err.Error())
	case m.streaming:
		statusLine = m.spinner.View() + styles.ChatSpinner.Render(" Generating…")
	default:
		statusLine = styles.Dim.Render(fmt.Sprintf(
			" Session %d/%d · %d messages",
			m.activeSession+1, len(m.sessions),
			len(m.sessions[m.activeSession].Messages),
		))
	}

	inputBox := styles.ChatInput.Width(m.width).Render(m.input.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		tabs.String(),
		statusLine,
		m.viewport.View(),
		inputBox,
	)
}

// ── Rendering helpers ─────────────────────────────────────────────────────────

// renderMessages converts the active session's message list into a styled
// string suitable for the viewport.
func (m Model) renderMessages() string {
	sess := m.sessions[m.activeSession]
	if len(sess.Messages) == 0 {
		return styles.ChatSystemMsg.Render("\n  👋 New session — say something!")
	}

	// Keep bubble width comfortable and away from panel borders.
	w := m.width - 4
	if w < 20 {
		w = 20
	}

	var b strings.Builder
	for _, msg := range sess.Messages {
		ts := msg.Time.Format("15:04")
		switch msg.Role {
		case "user":
			header := styles.ChatUserMsg.Render("You") + styles.Dim.Render(" "+ts)
			bubble := styles.ChatUserBubble.Width(w).Render(msg.Content)
			b.WriteString(header + "\n" + bubble + "\n")
		case "model":
			header := styles.ChatAIMsg.Render("🐙 Gemini") + styles.Dim.Render(" "+ts)
			bubble := styles.ChatAIBubble.Width(w).Render(msg.Content)
			b.WriteString(header + "\n" + bubble + "\n")
		}
	}
	return b.String()
}

// ── Streaming ─────────────────────────────────────────────────────────────────

// sendMessage is a Bubble Tea Cmd that:
//  1. Cancels any in-flight stream.
//  2. Adds the user's text to the session immediately (for instant display).
//  3. Launches a goroutine that calls the Gemini API and pushes tokens into
//     a buffered channel.
//  4. Waits for the first token and returns it as a streamTokenMsg, which
//     triggers the recursive waitForToken chain.
func (m Model) sendMessage() tea.Cmd {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return nil // nothing to send
	}

	// Cancel any previous in-flight request to avoid duplicate responses.
	if m.cancelCtx != nil {
		m.cancelCtx()
	}
	// Create a fresh context for this request.
	m.ctx, m.cancelCtx = context.WithCancel(context.Background())
	// Fresh buffered channel; old one is abandoned after the previous cancel.
	m.tokenChan = make(chan string, 512)
	m.streaming = true
	m.err = nil

	// Add user message to the session immediately so the UI updates at once.
	sess := m.sessions[m.activeSession]
	sess.Messages = append(sess.Messages, Message{
		Role:    "user",
		Content: text,
		Time:    time.Now(),
	})
	// Auto-title the session from the first message.
	if sess.Title == "New Session" && len(sess.Messages) == 1 {
		sess.Title = truncate(text, 20)
	}
	m.sessions[m.activeSession] = sess
	m.input.Reset()
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	// Capture values by copying into local variables so the closures below
	// don't race against future model updates (model is a value type in Bubbletea).
	client := m.client
	ctx := m.ctx
	ch := m.tokenChan
	history := m.buildHistory() // all prior turns, excluding the one we just added
	userText := text

	return func() tea.Msg {
		// Handle the case where no API key was configured.
		if client == nil {
			ch <- "[Error: Gemini client not initialized — set GEMINI_API_KEY]"
			close(ch)
			// Return the error token so it appears in the chat.
			return streamTokenMsg{token: <-ch}
		}

		// Launch the streaming goroutine.
		go func() {
			defer close(ch) // closing the channel signals streamDoneMsg

			// Create a new chat session from the saved history so the model
			// has full context about what was discussed before.
			chatSession, err := client.Chats.Create(ctx, modelName, nil, history)
			if err != nil {
				select {
				case ch <- "[Error creating session: " + err.Error() + "]":
				case <-ctx.Done():
				}
				return
			}

			// Call the streaming API. SendMessageStream returns an iterator
			// (iter.Seq2) that yields response chunks as they arrive.
			var full strings.Builder
			for chunk, err := range chatSession.SendMessageStream(ctx, genai.Part{Text: userText}) {
				if err != nil {
					// Non-nil error from the iterator means the stream broke.
					select {
					case ch <- "\n[Error: " + err.Error() + "]":
					case <-ctx.Done():
					}
					return
				}
				// chunk.Text() concatenates all text Parts in this response chunk.
				if t := chunk.Text(); t != "" {
					full.WriteString(t)
					select {
					case ch <- t: // push token to the UI
					case <-ctx.Done():
						return // request was cancelled (user sent new message)
					}
				}
			}
			// Push the complete response as one final value so streamDoneMsg
			// has the text available to save to history. We reuse the channel
			// by convention: close signals done, and the full text was already
			// accumulated in waitForToken via streamTokenMsg updates.
			// (ch is closed by defer above; the full text is reconstructed in
			//  streamDoneMsg by the Update handler from m.streamResponse.)
			_ = full // full text is accumulated in m.streamResponse via tokens
		}()

		// Block until the goroutine produces the first token, then return it
		// so Bubble Tea can render the first partial result immediately.
		token, ok := <-ch
		if !ok {
			return streamDoneMsg{fullResponse: ""}
		}
		return streamTokenMsg{token: token}
	}
}

// waitForToken is the recursive Bubble Tea Cmd that drains the token channel
// one message at a time. Each call blocks until the next token arrives, then
// returns either a streamTokenMsg (which triggers another waitForToken) or a
// streamDoneMsg when the channel is closed by the goroutine.
func (m Model) waitForToken() tea.Cmd {
	// Capture the channel so the closure doesn't need the whole model.
	ch := m.tokenChan
	// Capture the accumulated response so streamDoneMsg has the full text.
	accumulated := m.streamResponse
	return func() tea.Msg {
		token, ok := <-ch
		if !ok {
			// Channel closed → streaming is complete.
			return streamDoneMsg{fullResponse: accumulated}
		}
		return streamTokenMsg{token: token}
	}
}

// ── Session management ────────────────────────────────────────────────────────

// newSession creates a fresh chat session. If we already have maxSessions,
// the oldest one is dropped to make room.
func (m Model) newSession() Model {
	if len(m.sessions) >= maxSessions {
		m.sessions = m.sessions[1:] // evict the oldest session
	}
	m.sessions = append(m.sessions, newSession())
	m.activeSession = len(m.sessions) - 1
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	m.saveHistory()
	return m
}

// deleteActiveSession removes the current session.
func (m Model) deleteActiveSession() Model {
	if len(m.sessions) <= 1 {
		// If it's the last session, just reset it rather than deleting the slice element.
		m.sessions[0] = newSession()
	} else {
		m.sessions = append(m.sessions[:m.activeSession], m.sessions[m.activeSession+1:]...)
		if m.activeSession >= len(m.sessions) {
			m.activeSession = len(m.sessions) - 1
		}
	}
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	m.saveHistory()
	return m
}

// newSession (package-level) returns a blank Session with a unique ID.
func newSession() Session {
	return Session{
		ID:        fmt.Sprintf("s%d", time.Now().UnixNano()),
		Title:     "New Session",
		CreatedAt: time.Now(),
		Messages:  []Message{},
	}
}

// buildHistory converts the saved messages of the active session into the
// []*genai.Content slice expected by the Gemini Chats API. The last message
// is excluded because it is the user message being sent right now.
func (m Model) buildHistory() []*genai.Content {
	sess := m.sessions[m.activeSession]
	msgs := sess.Messages
	// Slice off the last entry — it's the user message we're about to send.
	if len(msgs) > 0 {
		msgs = msgs[:len(msgs)-1]
	}
	history := make([]*genai.Content, 0, len(msgs))
	for _, msg := range msgs {
		history = append(history, &genai.Content{
			Role:  msg.Role, // "user" or "model"
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}
	return history
}

// ── Persistence ───────────────────────────────────────────────────────────────

// loadHistory reads chat_history.json and returns up to maxSessions sessions.
// Returns nil (not an error) if the file doesn't exist yet.
func (m Model) loadHistory() []Session {
	data, err := os.ReadFile(m.dataPath)
	if err != nil {
		return nil // first run: no file yet
	}
	var h history
	if err := json.Unmarshal(data, &h); err != nil {
		return nil // corrupt file; start fresh
	}
	sessions := h.Sessions
	// Enforce the session cap in case the file was edited manually.
	if len(sessions) > maxSessions {
		sessions = sessions[len(sessions)-maxSessions:]
	}
	return sessions
}

// saveHistory writes the current sessions to disk as pretty-printed JSON.
// Errors are silently ignored to avoid disrupting the UI.
func (m Model) saveHistory() {
	h := history{Sessions: m.sessions}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.dataPath, data, 0o644)
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// truncate shortens s to max runes, appending an ellipsis if needed.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
