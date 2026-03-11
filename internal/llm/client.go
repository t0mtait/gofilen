package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Role is an LLM conversation participant.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in the conversation history.
type Message struct {
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a function call made by the LLM.
type ToolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// Tool defines a function the LLM can call.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a tool's name, purpose, and parameter schema.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

// ToolParameters is the JSON Schema for a tool's arguments.
type ToolParameters struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]PropertySchema `json:"properties"`
}

// PropertySchema describes a single tool argument.
type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// StreamEvent is an event emitted by the conversation runner.
type StreamEvent struct {
	Type           string    // "chunk" | "tool_call" | "tool_result" | "tool_cancelled" | "done" | "error"
	Content        string    // chunk text
	ToolName       string    // tool_call / tool_result / tool_cancelled
	ToolArgs       string    // tool_call — JSON string
	ToolResult     string    // tool_result
	Err            error     // error
	UpdatedHistory []Message // set on "done"
	// ConfirmCh is non-nil for destructive tool calls. The TUI must send
	// true (confirm) or false (cancel) to proceed.
	ConfirmCh chan bool
}

// Client is a minimal Ollama HTTP client.
type Client struct {
	BaseURL string
	Model   string
	http    *http.Client
}

// NewClient creates a new Ollama client.
func NewClient(baseURL, model string) *Client {
	return &Client{
		BaseURL: baseURL,
		Model:   model,
		http:    &http.Client{},
	}
}

type chatResponse struct {
	Message struct {
		Role      Role       `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done bool `json:"done"`
}

// chat sends a single request and streams events to ch, then closes ch.
func (c *Client) chat(ctx context.Context, messages []Message, tools []Tool, ch chan<- StreamEvent) {
	defer close(ch)

	body, err := json.Marshal(map[string]interface{}{
		"model":    c.Model,
		"messages": messages,
		"tools":    tools,
		"stream":   true,
	})
	if err != nil {
		ch <- StreamEvent{Type: "error", Err: err}
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		ch <- StreamEvent{Type: "error", Err: err}
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		ch <- StreamEvent{Type: "error", Err: fmt.Errorf("ollama unreachable: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ch <- StreamEvent{Type: "error", Err: fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)}
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var r chatResponse
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		if len(r.Message.ToolCalls) > 0 {
			for i := range r.Message.ToolCalls {
				ch <- StreamEvent{Type: "tool_call_raw", ToolName: r.Message.ToolCalls[i].Function.Name, Content: string(r.Message.ToolCalls[i].Function.Arguments)}
			}
			// tool calls imply done
			return
		}
		if r.Message.Content != "" {
			ch <- StreamEvent{Type: "chunk", Content: r.Message.Content}
		}
		if r.Done {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Type: "error", Err: err}
	}
}
