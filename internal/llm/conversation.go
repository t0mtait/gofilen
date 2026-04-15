package llm

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/t0mtait/gofilen/internal/filer"
)

const maxToolRounds = 10

// RunConversation drives a full chat turn: it streams the LLM response, handles
// any tool calls (executing them and re-prompting), and sends all events to ch.
// ch is closed when the turn is complete.
func RunConversation(ctx context.Context, client *Client, initialMessages []Message, tools []Tool, f filer.Filer, ch chan<- StreamEvent) {
	defer close(ch)

	messages := make([]Message, len(initialMessages))
	copy(messages, initialMessages)

	for round := 0; round < maxToolRounds; round++ {
		innerCh := make(chan StreamEvent, 64)
		go client.chat(ctx, messages, tools, innerCh)

		var contentBuf strings.Builder
		var rawToolCalls []struct {
			name string
			args json.RawMessage
		}

		for e := range innerCh {
			switch e.Type {
			case "chunk":
				contentBuf.WriteString(e.Content)
				ch <- e // forward streaming text to TUI

			case "tool_call_raw":
				rawToolCalls = append(rawToolCalls, struct {
					name string
					args json.RawMessage
				}{name: e.ToolName, args: json.RawMessage(e.Content)})

			case "error":
				ch <- e
				return
			}
		}

		// Flush accumulated streaming content as an assistant message.
		if contentBuf.Len() > 0 {
			content := contentBuf.String()
			messages = append(messages, Message{Role: RoleAssistant, Content: content})
		}

		if len(rawToolCalls) == 0 {
			// No tool calls — conversation turn complete.
			ch <- StreamEvent{Type: "done", UpdatedHistory: messages}
			return
		}

		// Build the assistant's tool-call message for the history.
		toolCallStructs := make([]ToolCall, len(rawToolCalls))
		for i, tc := range rawToolCalls {
			toolCallStructs[i].Function.Name = tc.name
			toolCallStructs[i].Function.Arguments = tc.args
		}
		messages = append(messages, Message{
			Role:      RoleAssistant,
			Content:   "",
			ToolCalls: toolCallStructs,
		})

		// Execute each tool and collect results.
		for _, tc := range rawToolCalls {
			argsStr := string(tc.args)

			if IsDestructive(tc.name) {
				// Ask the TUI for confirmation before executing.
				confirmCh := make(chan bool, 1)
				ch <- StreamEvent{
					Type:      "tool_call",
					ToolName:  tc.name,
					ToolArgs:  argsStr,
					ConfirmCh: confirmCh,
				}
				confirmed := <-confirmCh
				if !confirmed {
					ch <- StreamEvent{Type: "tool_cancelled", ToolName: tc.name}
					messages = append(messages, Message{Role: RoleTool, Content: "User cancelled this operation."})
					continue
				}
			} else {
				ch <- StreamEvent{Type: "tool_call", ToolName: tc.name, ToolArgs: argsStr}
			}

			result, err := ExecuteTool(tc.name, tc.args, f)
			if err != nil {
				result = "Error: " + err.Error()
			}

			ch <- StreamEvent{Type: "tool_result", ToolName: tc.name, ToolResult: result}
			messages = append(messages, Message{Role: RoleTool, Content: result})
		}
		// Loop: re-prompt the LLM with tool results.
	}

	// Exceeded max rounds — bail gracefully.
	ch <- StreamEvent{Type: "done", UpdatedHistory: messages}
}
