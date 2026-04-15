package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/t0mtait/gofilen/internal/config"
	"github.com/t0mtait/gofilen/internal/filer"
	"github.com/t0mtait/gofilen/internal/llm"
)

// ── Display message types ──────────────────────────────────────────────────────

type msgKind int

const (
	msgSystem msgKind = iota
	msgUser
	msgAssistant
	msgToolCall
	msgToolResult
	msgToolCancelled
	msgError
)

type displayMsg struct {
	kind    msgKind
	label   string
	content string
}

// ── Pending confirmation state ─────────────────────────────────────────────────

type pendingConfirm struct {
	toolName  string
	toolArgs  string
	confirmCh chan bool
	streamCh  chan llm.StreamEvent
}

// ── Bubble Tea message types ───────────────────────────────────────────────────

type streamEventMsg struct {
	event llm.StreamEvent
	ch    chan llm.StreamEvent
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	cfg       config.Config
	filer     filer.Filer
	llmClient *llm.Client
	tools     []llm.Tool

	// LLM conversation history (includes system prompt)
	history []llm.Message

	// What's rendered in the chat viewport
	display []displayMsg

	// Current streamed-but-not-yet-committed assistant text
	streaming string
	thinking  bool

	// Non-nil when waiting for user to confirm a destructive tool call
	confirm *pendingConfirm

	// TUI components
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	width  int
	height int
	ready  bool
}

func newModel(cfg config.Config, f filer.Filer) Model {
	ta := textarea.New()
	ta.Placeholder = "Message the AI…  (Enter to send, Alt+Enter for newline)"
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.CharLimit = 8192
	ta.SetHeight(3)

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = headerSpinnerStyle

	tree := f.Tree(3)
	systemPrompt := fmt.Sprintf(`You are an intelligent assistant that manages the user's Filen cloud drive.

CAPABILITIES — when asked "what can you do?" or similar, list these:
- List files and folders (with sizes and modification dates)
- Read the contents of any text file
- Create new files with specified content
- Create new directories (including nested paths)
- Delete files or directories
- Move or rename files and directories
- Copy files
- Show action history (all file operations performed this session)
- Answer questions about the files and their contents

TOOLS AVAILABLE: list_files, read_file, write_file, create_dir, delete, move, copy, get_action_history

RULES:
1. Only call a tool when the user explicitly asks you to do something with their files.
2. For greetings or general questions unrelated to files, respond conversationally — do NOT call any tool.
3. The tools write_file, create_dir, delete, move, and copy will automatically prompt the user for confirmation before executing — you do not need to ask yourself.
4. Do NOT re-fetch the file tree unless the user asks — use the snapshot below as context.
5. When asked about your history or what you've done, call get_action_history.
6. When asked what tools are available, list: list_files, read_file, write_file, create_dir, delete, move, copy, get_action_history.

Current file tree of the Filen drive (up to 3 levels):
%s`, tree)

	history := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
	}

	display := []displayMsg{
		{
			kind:    msgSystem,
			content: fmt.Sprintf("Filen drive: %s  •  model: %s  •  Enter to send, Alt+Enter for newline, Ctrl+L to clear, Esc to quit", cfg.Dir, cfg.Model),
		},
	}

	return Model{
		cfg:       cfg,
		filer:     f,
		llmClient: llm.NewClient(cfg.OllamaURL, cfg.Model),
		tools:     llm.FileTools(),
		history:   history,
		display:   display,
		spinner:   sp,
		textarea:  ta,
	}
}

// Run starts the Bubble Tea program.
func Run(cfg config.Config) error {
	f, err := filer.NewLocal(cfg.Dir)
	if err != nil {
		return fmt.Errorf("cannot access Filen mount at %s: %w", cfg.Dir, err)
	}
	m := newModel(cfg, f)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.recalcLayout()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if msg.Alt {
				// Alt+Enter → let textarea handle (inserts newline)
				break
			}
			// Confirmation mode takes priority over normal submission.
			if m.confirm != nil {
				input := strings.ToLower(strings.TrimSpace(m.textarea.Value()))
				m.textarea.Reset()
				confirmed := input == "y" || input == "yes"
				m.confirm.confirmCh <- confirmed
				ch := m.confirm.streamCh
				m.confirm = nil
				m.textarea.Placeholder = "Message the AI…  (Enter to send, Alt+Enter for newline)"
				return m, waitForStream(ch)
			}
			if m.thinking {
				return m, nil
			}
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}
			m.textarea.Reset()
			return m.submit(input)

		case tea.KeyCtrlL:
			m.display = []displayMsg{{kind: msgSystem, content: "Chat cleared."}}
			m.history = m.history[:1] // keep system message
			m.streaming = ""
			m.viewport.SetContent(m.renderChat())
			return m, nil
		}

	case streamEventMsg:
		return m.handleStreamEvent(msg)

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	var vpCmd, taCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, vpCmd, taCmd)

	return m, tea.Batch(cmds...)
}

func (m Model) recalcLayout() Model {
	const headerH = 3
	const footerH = 2
	const inputH = 5

	vpH := m.height - headerH - footerH - inputH
	if vpH < 5 {
		vpH = 5
	}
	vpW := m.width - 4 // account for border

	if !m.ready {
		m.viewport = viewport.New(vpW, vpH)
		m.viewport.SetContent(m.renderChat())
		m.ready = true
	} else {
		m.viewport.Width = vpW
		m.viewport.Height = vpH
	}
	m.textarea.SetWidth(m.width - 6)
	return m
}

// ── Submit ────────────────────────────────────────────────────────────────────

func (m Model) submit(text string) (Model, tea.Cmd) {
	m.display = append(m.display, displayMsg{kind: msgUser, label: "You", content: text})
	m.history = append(m.history, llm.Message{Role: llm.RoleUser, Content: text})
	m.thinking = true
	m.streaming = ""
	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()

	ch := make(chan llm.StreamEvent, 64)
	go llm.RunConversation(context.Background(), m.llmClient, m.history, m.tools, m.filer, ch)

	return m, tea.Batch(waitForStream(ch), m.spinner.Tick)
}

func waitForStream(ch chan llm.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return streamEventMsg{event: llm.StreamEvent{Type: "done"}, ch: ch}
		}
		return streamEventMsg{event: e, ch: ch}
	}
}

// ── Stream event handler ──────────────────────────────────────────────────────

func (m Model) handleStreamEvent(msg streamEventMsg) (Model, tea.Cmd) {
	e := msg.event

	switch e.Type {
	case "chunk":
		m.streaming += e.Content
		m.viewport.SetContent(m.renderChatWithStreaming())
		m.viewport.GotoBottom()
		return m, tea.Batch(waitForStream(msg.ch), m.spinner.Tick)

	case "tool_call":
		// Commit any partial streamed text first.
		if m.streaming != "" {
			m.display = append(m.display, displayMsg{kind: msgAssistant, label: "AI", content: m.streaming})
			m.streaming = ""
		}
		if e.ConfirmCh != nil {
			// Destructive tool — pause stream and ask user.
			m.display = append(m.display, displayMsg{
				kind:    msgToolCall,
				label:   e.ToolName,
				content: e.ToolArgs,
			})
			m.confirm = &pendingConfirm{
				toolName:  e.ToolName,
				toolArgs:  e.ToolArgs,
				confirmCh: e.ConfirmCh,
				streamCh:  msg.ch,
			}
			m.textarea.Placeholder = "Confirm? type y to proceed, n to cancel, then Enter"
			m.viewport.SetContent(m.renderChat())
			m.viewport.GotoBottom()
			return m, waitForStream(msg.ch)
		}
		m.display = append(m.display, displayMsg{kind: msgToolCall, label: e.ToolName, content: e.ToolArgs})
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return m, tea.Batch(waitForStream(msg.ch), m.spinner.Tick)

	case "tool_cancelled":
		m.display = append(m.display, displayMsg{
			kind:    msgToolCancelled,
			label:   e.ToolName,
			content: "cancelled by user",
		})
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return m, tea.Batch(waitForStream(msg.ch), m.spinner.Tick)

	case "tool_result":
		m.display = append(m.display, displayMsg{kind: msgToolResult, label: e.ToolName, content: e.ToolResult})
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return m, tea.Batch(waitForStream(msg.ch), m.spinner.Tick)

	case "done":
		if m.streaming != "" {
			m.display = append(m.display, displayMsg{kind: msgAssistant, label: "AI", content: m.streaming})
			m.streaming = ""
		}
		if e.UpdatedHistory != nil {
			m.history = e.UpdatedHistory
		}
		m.thinking = false
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return m, nil

	case "error":
		m.display = append(m.display, displayMsg{kind: msgError, content: e.Err.Error()})
		m.thinking = false
		m.streaming = ""
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, waitForStream(msg.ch)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return "\n  Loading…"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		viewportBorderStyle.
			Width(m.width-2).
			Height(m.viewport.Height+2).
			Render(m.viewport.View()),
		m.renderInputArea(),
		m.renderFooter(),
	)
}

func (m Model) renderHeader() string {
	title := headerTitleStyle.Render("⬡ gofilen")
	dir := headerDimStyle.Render("  " + m.cfg.Dir)
	model := headerDimStyle.Render("  " + m.cfg.Model)

	var status string
	if m.thinking {
		status = "  " + m.spinner.View() + headerDimStyle.Render(" thinking…")
	}

	content := lipgloss.JoinHorizontal(lipgloss.Center, title, dir, model, status)
	return headerStyle.Width(m.width).Render(content)
}

func (m Model) renderInputArea() string {
	var style lipgloss.Style
	switch {
	case m.confirm != nil:
		style = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cYellow).
			Padding(0, 1)
	case m.thinking:
		style = inputIdleStyle
	default:
		style = inputBorderStyle
	}
	return style.Width(m.width - 4).Render(m.textarea.View())
}

func (m Model) renderFooter() string {
	if m.confirm != nil {
		hint := keyStyle.Render("y") + " confirm  ·  " + keyStyle.Render("n") + " cancel  ·  then " + keyStyle.Render("Enter")
		return footerStyle.Width(m.width).Render("⚠  Confirm " + m.confirm.toolName + ":  " + hint)
	}
	keys := []string{
		keyStyle.Render("Enter") + " send",
		keyStyle.Render("Alt+Enter") + " newline",
		keyStyle.Render("Ctrl+L") + " clear",
		keyStyle.Render("Esc") + " quit",
	}
	return footerStyle.Width(m.width).Render(strings.Join(keys, "  ·  "))
}

// ── Chat rendering ────────────────────────────────────────────────────────────

func (m Model) renderChat() string {
	return m.buildChatContent(m.display, "")
}

func (m Model) renderChatWithStreaming() string {
	return m.buildChatContent(m.display, m.streaming)
}

func (m Model) buildChatContent(msgs []displayMsg, streaming string) string {
	w := m.viewport.Width - 2
	if w < 20 {
		w = 80
	}

	var parts []string
	for _, msg := range msgs {
		parts = append(parts, m.renderMsg(msg, w))
	}

	if streaming != "" {
		parts = append(parts, m.renderStreamingBubble(streaming, w))
	} else if m.thinking && len(parts) > 0 {
		parts = append(parts, systemStyle.Render("  "+m.spinner.View()))
	}

	return strings.Join(parts, "\n\n")
}

func (m Model) renderMsg(msg displayMsg, w int) string {
	switch msg.kind {

	case msgUser:
		label := userLabelStyle.Render("▶ " + msg.label)
		body := userBodyStyle.Width(w).Render(msg.content)
		return lipgloss.JoinVertical(lipgloss.Left, label, body)

	case msgAssistant:
		label := aiLabelStyle.Render("◆ AI")
		body := aiBodyStyle.Width(w).Render(msg.content)
		return lipgloss.JoinVertical(lipgloss.Left, label, body)

	case msgToolCall:
		args := prettyJSON(msg.content)
		inner := fmt.Sprintf("⚙  %s\n%s", msg.label, indent(args, "   "))
		return toolCallBoxStyle.Width(w).Render(inner)

	case msgToolResult:
		trimmed := strings.TrimRight(msg.content, "\n")
		lines := strings.Split(trimmed, "\n")
		if len(lines) > 20 {
			lines = append(lines[:20], fmt.Sprintf("… (%d more lines)", len(lines)-20))
		}
		return toolResultBarStyle.Width(w - 2).Render(strings.Join(lines, "\n"))

	case msgToolCancelled:
		return lipgloss.NewStyle().Foreground(cYellow).Render("⊘ " + msg.label + " — cancelled")

	case msgSystem:
		return systemStyle.Width(w).Render("— " + msg.content + " —")

	case msgError:
		return errorStyle.Render("✗ " + msg.content)
	}
	return ""
}

func (m Model) renderStreamingBubble(content string, w int) string {
	label := aiLabelStyle.Render("◆ AI")
	body := aiBodyStyle.Width(w).Render(content + "▌")
	return lipgloss.JoinVertical(lipgloss.Left, label, body)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func prettyJSON(s string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(b)
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
