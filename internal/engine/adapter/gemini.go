package adapter

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buildoak/agent-mux/internal/types"
)

type GeminiAdapter struct {
	mu           sync.Mutex
	pendingFiles map[string]string
	toolNames    map[string]string
	deltaBuffer  strings.Builder
}

type geminiEvent struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id"`
	Model     string          `json:"model"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolID    string          `json:"tool_id"`
	// New field names from Gemini CLI v0.34.0
	ToolName   string          `json:"tool_name"`
	Parameters json.RawMessage `json:"parameters"`
	Status     string          `json:"status"` // "success" | "error"
	Delta      bool            `json:"delta"`
	// Legacy field names — kept for backward compatibility with older Gemini CLI versions
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	IsError bool            `json:"is_error"`
	// Shared fields
	Output     string       `json:"output"`
	DurationMS int64        `json:"duration_ms"`
	Code       string       `json:"code"`
	Message    string       `json:"message"`
	Result     string       `json:"result"` // kept for backward compat only
	Stats      *geminiStats `json:"stats"`
}

// resolvedName returns the tool name, preferring the new field over the legacy one.
func (e *geminiEvent) resolvedName() string {
	if e.ToolName != "" {
		return e.ToolName
	}
	return e.Name
}

// resolvedParams returns the parameters payload, preferring the new field over the legacy one.
func (e *geminiEvent) resolvedParams() json.RawMessage {
	if len(e.Parameters) > 0 {
		return e.Parameters
	}
	return e.Input
}

// isError returns whether this event represents an error condition.
func (e *geminiEvent) isError() bool {
	if e.Status != "" {
		return e.Status == "error"
	}
	return e.IsError
}

type geminiStats struct {
	TotalTokens  int                        `json:"total_tokens"`
	InputTokens  int                        `json:"input_tokens"`
	OutputTokens int                        `json:"output_tokens"`
	Cached       int                        `json:"cached"`
	Input        int                        `json:"input"`
	DurationMS   int64                      `json:"duration_ms"`
	ToolCalls    int                        `json:"tool_calls"`
	Turns        int                        `json:"turns"`
	Models       map[string]geminiModelStats `json:"models"`
}

type geminiModelStats struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Cached       int `json:"cached"`
	Input        int `json:"input"`
}

var validGeminiApprovalModes = map[string]bool{
	"default":   true,
	"auto_edit": true,
	"yolo":      true,
	"plan":      true,
}

// ValidateGeminiApprovalMode checks that the approval mode is one of the values
// accepted by the Gemini CLI. Call before BuildArgs to catch invalid values
// early with a structured error instead of a silent Gemini crash.
func ValidateGeminiApprovalMode(mode string) error {
	if validGeminiApprovalModes[mode] {
		return nil
	}
	return fmt.Errorf("invalid Gemini approval mode %q: valid values are default, auto_edit, yolo, plan", mode)
}

// uuidPattern matches UUID-formatted strings (e.g. "550e8400-e29b-41d4-a716-446655440000").
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (a *GeminiAdapter) Binary() string {
	return "gemini"
}

func (a *GeminiAdapter) BuildArgs(spec *types.DispatchSpec) []string {
	if spec.EngineOpts == nil {
		spec.EngineOpts = map[string]any{}
	}

	if opts, ok := spec.EngineOpts["reasoning"]; ok {
		if r, ok := opts.(string); ok && r != "" {
			log.Printf("[gemini] Gemini CLI does not support effort flag; ignoring effort=%s — use model selection for thinking depth control", r)
		}
	}

	args := []string{"-p", spec.Prompt, "-o", "stream-json"}
	if spec.Model != "" {
		args = append(args, "-m", spec.Model)
	}
	approvalMode := "yolo"
	if mode, ok := spec.EngineOpts["permission-mode"].(string); ok && mode != "" {
		approvalMode = mode
	}
	args = append(args, "--approval-mode", approvalMode)
	// Always include $HOME and /tmp so Gemini can read context files and
	// artifacts outside --cwd (its workspace sandbox restricts reads otherwise).
	dirs := addDirs(spec)
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, home)
	}
	dirs = append(dirs, "/tmp")
	args = append(args, "--include-directories", strings.Join(dirs, ","))
	return args
}

func (a *GeminiAdapter) EnvVars(spec *types.DispatchSpec) ([]string, error) {
	if spec == nil || spec.SystemPrompt == "" || spec.ArtifactDir == "" {
		return nil, nil
	}
	path := filepath.Join(spec.ArtifactDir, "system_prompt.md")
	if err := os.WriteFile(path, []byte(spec.SystemPrompt), 0644); err != nil {
		return nil, fmt.Errorf("write Gemini system prompt %q: %w", path, err)
	}
	return []string{"GEMINI_SYSTEM_MD=" + path}, nil
}

func (a *GeminiAdapter) ParseEvent(line string) (*types.HarnessEvent, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	if !strings.HasPrefix(line, "{") {
		// Non-JSON lines with actual content (warnings, errors, progress from
		// the Gemini CLI) are surfaced as EventRawPassthrough so diagnostic
		// output is visible. Mirrors the Codex adapter pattern.
		return &types.HarnessEvent{
			Kind:      types.EventRawPassthrough,
			Timestamp: time.Now(),
			Raw:       []byte(line),
		}, nil
	}

	var raw geminiEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}

	evt := &types.HarnessEvent{
		Timestamp: time.Now(),
		Raw:       []byte(line),
	}

	// Parse the tool parameters from whichever field is populated.
	paramData := raw.resolvedParams()
	var inputFields struct {
		Path        string `json:"path"`
		FilePath    string `json:"file_path"`
		Command     string `json:"command"`
		Content     string `json:"content"`
		DirPath     string `json:"dir_path"`
		OldString   string `json:"old_string"`
		NewString   string `json:"new_string"`
		Description string `json:"description"`
	}
	if len(paramData) > 0 {
		if err := json.Unmarshal(paramData, &inputFields); err != nil {
			return nil, err
		}
	}

	switch raw.Type {
	case "init":
		evt.Kind = types.EventSessionStart
		evt.SessionID = raw.SessionID
	case "message":
		if raw.Role != "assistant" {
			return nil, nil
		}
		if raw.Delta {
			// Accumulate delta fragments into the buffer.
			a.mu.Lock()
			a.deltaBuffer.WriteString(raw.Content)
			a.mu.Unlock()
			// Emit a progress event with the fragment for streaming display.
			evt.Kind = types.EventProgress
			evt.Text = raw.Content
		} else {
			// Non-delta assistant message (e.g. single complete message).
			evt.Kind = types.EventProgress
			evt.Text = raw.Content
		}
	case "tool_use":
		return a.parseToolUse(&raw, &inputFields, evt), nil
	case "tool_result":
		return a.parseToolResult(&raw, evt), nil
	case "error":
		evt.Kind = types.EventError
		evt.ErrorCode = raw.Code
		evt.Text = raw.Message
	case "result":
		if raw.isError() {
			evt.Kind = types.EventError
			evt.ErrorCode = "result_error"
			evt.Text = raw.Message
		} else {
			evt.Kind = types.EventResponse
			evt.SessionID = raw.SessionID
			// Flush the accumulated delta buffer as the response text.
			a.mu.Lock()
			accumulated := a.deltaBuffer.String()
			a.deltaBuffer.Reset()
			a.mu.Unlock()
			if accumulated != "" {
				evt.Text = accumulated
			} else {
				// Backward compat: if no deltas were accumulated, fall back to raw.Result.
				evt.Text = raw.Result
			}
			tokens, actualModel := geminiStatsToTokens(raw.Stats)
			evt.Tokens = tokens
			evt.ActualModel = actualModel
		}
	default:
		evt.Kind = types.EventRawPassthrough
	}

	return evt, nil
}

func (a *GeminiAdapter) parseToolUse(raw *geminiEvent, inputFields *struct {
	Path        string `json:"path"`
	FilePath    string `json:"file_path"`
	Command     string `json:"command"`
	Content     string `json:"content"`
	DirPath     string `json:"dir_path"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	Description string `json:"description"`
}, evt *types.HarnessEvent) *types.HarnessEvent {
	toolName := raw.resolvedName()

	// Helper to resolve file path from parameters (new field name takes priority).
	filePath := inputFields.FilePath
	if filePath == "" {
		filePath = inputFields.Path
	}

	a.mu.Lock()
	if a.toolNames == nil {
		a.toolNames = make(map[string]string)
	}
	a.toolNames[raw.ToolID] = toolName
	a.mu.Unlock()

	switch toolName {
	case "read_file":
		evt.Kind = types.EventFileRead
		evt.FilePath = filePath
	case "write_file":
		evt.Kind = types.EventToolStart
		evt.Tool = toolName
		a.mu.Lock()
		if a.pendingFiles == nil {
			a.pendingFiles = make(map[string]string)
		}
		a.pendingFiles[raw.ToolID] = filePath
		a.mu.Unlock()
	case "replace":
		evt.Kind = types.EventFileWrite
		evt.Tool = toolName
		a.mu.Lock()
		if a.pendingFiles == nil {
			a.pendingFiles = make(map[string]string)
		}
		a.pendingFiles[raw.ToolID] = filePath
		a.mu.Unlock()
	case "shell", "run_shell_command":
		evt.Kind = types.EventCommandRun
		evt.Tool = toolName
		evt.Command = inputFields.Command
	case "list_directory":
		evt.Kind = types.EventToolStart
		evt.Tool = toolName
		// dir_path stored as FilePath for display purposes
		evt.FilePath = inputFields.DirPath
	default:
		evt.Kind = types.EventToolStart
		evt.Tool = toolName
	}
	return evt
}

func (a *GeminiAdapter) parseToolResult(raw *geminiEvent, evt *types.HarnessEvent) *types.HarnessEvent {
	// Look up the tool name from the tool_use event via tool_id.
	a.mu.Lock()
	toolName := a.toolNames[raw.ToolID]
	a.mu.Unlock()
	// Fall back to the name on the event itself (legacy schema / older CLI versions).
	if toolName == "" {
		toolName = raw.resolvedName()
	}

	errored := raw.isError()

	switch {
	case (toolName == "write_file") && !errored:
		evt.Kind = types.EventFileWrite
		evt.Tool = toolName
		evt.DurationMS = raw.DurationMS
		evt.FilePath = a.takePendingFile(raw.ToolID)
	case (toolName == "replace") && !errored:
		evt.Kind = types.EventFileWrite
		evt.Tool = toolName
		evt.DurationMS = raw.DurationMS
		evt.FilePath = a.takePendingFile(raw.ToolID)
	case errored:
		evt.Kind = types.EventError
		evt.Tool = toolName
		evt.DurationMS = raw.DurationMS
		evt.ErrorCode = "tool_error"
		evt.Text = raw.Output
		// Clean up pending file entry if applicable.
		if toolName == "write_file" || toolName == "replace" {
			a.takePendingFile(raw.ToolID)
		}
	default:
		evt.Kind = types.EventToolEnd
		if toolName == "shell" || toolName == "run_shell_command" {
			evt.SecondaryKind = types.EventCommandRun
		}
		evt.Tool = toolName
		evt.DurationMS = raw.DurationMS
	}

	// Clean up tool name mapping.
	a.mu.Lock()
	if a.toolNames != nil {
		delete(a.toolNames, raw.ToolID)
	}
	a.mu.Unlock()

	return evt
}

func (a *GeminiAdapter) takePendingFile(toolID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pendingFiles == nil {
		return ""
	}
	path := a.pendingFiles[toolID]
	delete(a.pendingFiles, toolID)
	return path
}

// geminiStatsToTokens converts Gemini stats into a TokenUsage and extracts the
// actual model name from the per-model breakdown (relevant for auto-routing
// profiles like "auto-gemini-3" where the init event model differs from the
// model that actually served the request).
func geminiStatsToTokens(stats *geminiStats) (*types.TokenUsage, string) {
	if stats == nil {
		return nil, ""
	}
	tokens := &types.TokenUsage{
		Input:     stats.InputTokens,
		Output:    stats.OutputTokens,
		CacheRead: stats.Cached,
	}
	return tokens, resolveActualModel(stats.Models)
}

// resolveActualModel picks the primary model from the per-model stats map.
// Single entry → that model. Multiple entries → the one with the most
// output_tokens (the model that did the heavy generation work).
func resolveActualModel(models map[string]geminiModelStats) string {
	if len(models) == 0 {
		return ""
	}
	bestModel := ""
	bestOutput := -1
	for name, m := range models {
		if m.OutputTokens > bestOutput {
			bestOutput = m.OutputTokens
			bestModel = name
		}
	}
	return bestModel
}

func (a *GeminiAdapter) SupportsResume() bool {
	return true
}

func (a *GeminiAdapter) ResumeArgs(_ *types.DispatchSpec, sessionID string, message string) []string {
	// The Gemini CLI --resume flag accepts "latest" or a numeric index, not a
	// UUID session ID. If the session ID looks like a UUID (from the init event),
	// fall back to "latest" since we don't have the numeric index mapping.
	resumeID := sessionID
	if uuidPattern.MatchString(sessionID) || strings.Contains(sessionID, "-") && len(sessionID) >= 36 {
		log.Printf("[gemini] session ID %q looks like a UUID; using \"latest\" for --resume (Gemini CLI does not accept UUID session IDs)", sessionID)
		resumeID = "latest"
	}
	return []string{"--resume", resumeID, "-p", message}
}
