// Package adapter implements harness-specific event parsing and CLI construction.
package adapter

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/buildoak/agent-mux/internal/types"
)

// CodexAdapter implements HarnessAdapter for Codex CLI.
type CodexAdapter struct{}

// NewCodexAdapter creates a new Codex adapter.
func NewCodexAdapter() *CodexAdapter {
	return &CodexAdapter{}
}

// Binary returns the Codex CLI binary name.
func (a *CodexAdapter) Binary() string {
	return "codex"
}

// BuildArgs constructs CLI flags from a DispatchSpec for Codex.
// Codex CLI uses: exec --json -m MODEL -s SANDBOX -C CWD -c key=value PROMPT
func (a *CodexAdapter) BuildArgs(spec *types.DispatchSpec) []string {
	args := []string{"exec", "--json"}

	if spec.Model != "" {
		args = append(args, "-m", spec.Model)
	}

	// Sandbox mode
	sandbox := "danger-full-access"
	if opts, ok := spec.EngineOpts["sandbox"]; ok {
		if s, ok := opts.(string); ok && s != "" {
			sandbox = s
		}
	}

	// Full access bypasses sandbox entirely
	if spec.FullAccess && sandbox == "danger-full-access" {
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	} else {
		args = append(args, "-s", sandbox)
	}

	// Working directory
	if spec.Cwd != "" {
		args = append(args, "-C", spec.Cwd)
	}

	// Reasoning effort via config override
	if opts, ok := spec.EngineOpts["reasoning"]; ok {
		if r, ok := opts.(string); ok && r != "" {
			args = append(args, "-c", "model_reasoning_effort="+r)
		}
	}

	// Additional writable directories
	if opts, ok := spec.EngineOpts["add_dirs"]; ok {
		if dirs, ok := opts.([]string); ok {
			for _, dir := range dirs {
				args = append(args, "--add-dir", dir)
			}
		}
	}
	// Also check "add-dir" key (CLI uses hyphenated form)
	if opts, ok := spec.EngineOpts["add-dir"]; ok {
		if dirs, ok := opts.([]string); ok {
			for _, dir := range dirs {
				args = append(args, "--add-dir", dir)
			}
		}
	}

	// System prompt prepended to the user prompt
	prompt := spec.Prompt
	if spec.SystemPrompt != "" {
		prompt = spec.SystemPrompt + "\n\n" + prompt
	}

	args = append(args, prompt)

	return args
}

// codexEvent is the raw JSON structure from Codex CLI --json output.
type codexEvent struct {
	Type         string          `json:"type"`
	ThreadID     string          `json:"thread_id,omitempty"`
	TurnIndex    int             `json:"turn_index,omitempty"`
	ItemID       string          `json:"item_id,omitempty"`
	ItemType     string          `json:"item_type,omitempty"`
	Command      string          `json:"command,omitempty"`
	Content      string          `json:"content,omitempty"`
	ContentDelta string          `json:"content_delta,omitempty"`
	FilePath     string          `json:"file_path,omitempty"`
	ChangeType   string          `json:"change_type,omitempty"`
	ExitCode     int             `json:"exit_code,omitempty"`
	DurationMS   int64           `json:"duration_ms,omitempty"`
	Usage        *codexUsage     `json:"usage,omitempty"`
	Error        *codexError     `json:"error,omitempty"`
	Code         string          `json:"code,omitempty"`
	Message      string          `json:"message,omitempty"`
	Model        string          `json:"model,omitempty"`
	CreatedAt    string          `json:"created_at,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

type codexUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type codexError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ParseEvent parses one line from the Codex event stream.
func (a *CodexAdapter) ParseEvent(line string) (*types.HarnessEvent, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	var raw codexEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		// Non-JSON line -> passthrough
		return &types.HarnessEvent{
			Kind:      types.EventRawPassthrough,
			Timestamp: time.Now(),
			Raw:       []byte(line),
		}, nil
	}

	evt := &types.HarnessEvent{
		Timestamp: time.Now(),
		Raw:       []byte(line),
	}

	switch raw.Type {
	case "thread.started":
		evt.Kind = types.EventSessionStart
		evt.SessionID = raw.ThreadID

	case "turn.started":
		// Internal bookkeeping, no direct mapping
		return nil, nil

	case "item.started":
		evt = a.parseItemStarted(&raw, evt)

	case "item.updated":
		if raw.ItemType == "agent_message" && raw.ContentDelta != "" {
			evt.Kind = types.EventProgress
			evt.Text = raw.ContentDelta
		} else {
			return nil, nil // skip other updates
		}

	case "item.completed":
		evt = a.parseItemCompleted(&raw, evt)

	case "turn.completed":
		evt.Kind = types.EventTurnComplete
		evt.DurationMS = raw.DurationMS
		if raw.Usage != nil {
			evt.Tokens = &types.TokenUsage{
				Input:     raw.Usage.InputTokens,
				Output:    raw.Usage.OutputTokens,
				Reasoning: raw.Usage.ReasoningTokens,
			}
		}

	case "turn.failed":
		evt.Kind = types.EventTurnFailed
		if raw.Error != nil {
			evt.ErrorCode = raw.Error.Code
			evt.Text = raw.Error.Message
		}

	case "error":
		evt.Kind = types.EventError
		evt.ErrorCode = raw.Code
		evt.Text = raw.Message

	default:
		evt.Kind = types.EventRawPassthrough
	}

	return evt, nil
}

func (a *CodexAdapter) parseItemStarted(raw *codexEvent, evt *types.HarnessEvent) *types.HarnessEvent {
	switch raw.ItemType {
	case "command_execution":
		evt.Kind = types.EventToolStart
		evt.Tool = "command_execution"
		evt.Command = raw.Command
	case "file_change":
		evt.Kind = types.EventToolStart
		evt.Tool = "file_change"
	case "web_search", "mcp_tool_call", "collab_tool_call":
		evt.Kind = types.EventToolStart
		evt.Tool = raw.ItemType
	case "agent_message":
		// Wait for completed
		return nil
	default:
		evt.Kind = types.EventToolStart
		evt.Tool = raw.ItemType
	}
	return evt
}

func (a *CodexAdapter) parseItemCompleted(raw *codexEvent, evt *types.HarnessEvent) *types.HarnessEvent {
	switch raw.ItemType {
	case "agent_message":
		evt.Kind = types.EventResponse
		evt.Text = raw.Content
	case "command_execution":
		evt.Kind = types.EventToolEnd
		evt.Tool = "command_execution"
		evt.Command = raw.Command
		evt.DurationMS = raw.DurationMS
	case "file_change":
		evt.Kind = types.EventFileWrite
		evt.FilePath = raw.FilePath
		evt.DurationMS = raw.DurationMS
	case "reasoning":
		evt.Kind = types.EventProgress
		evt.Text = raw.Content
	default:
		evt.Kind = types.EventToolEnd
		evt.Tool = raw.ItemType
		evt.DurationMS = raw.DurationMS
	}
	return evt
}

// SupportsResume returns whether Codex supports session resume.
func (a *CodexAdapter) SupportsResume() bool {
	return true
}

// ResumeArgs builds CLI args for resuming a Codex session with an injected message.
func (a *CodexAdapter) ResumeArgs(sessionID string, message string) []string {
	return []string{"exec", "resume", "--id", sessionID, "--json", message}
}

// IsCompletionEvent returns true if the event signals dispatch completion.
func IsCompletionEvent(evt *types.HarnessEvent) bool {
	return evt.Kind == types.EventTurnComplete || evt.Kind == types.EventTurnFailed
}

// IsErrorEvent returns true if the event represents a terminal error.
func IsErrorEvent(evt *types.HarnessEvent) bool {
	return evt.Kind == types.EventError || evt.Kind == types.EventTurnFailed
}
