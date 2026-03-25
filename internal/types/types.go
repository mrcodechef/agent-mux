// Package types defines shared type definitions for agent-mux v2.
// All JSON tags match SPEC-V2.md exactly.
package types

import (
	"context"
	"time"
)

// ── Dispatch Status ──────────────────────────────────────────

type DispatchStatus string

const (
	StatusCompleted DispatchStatus = "completed"
	StatusTimedOut  DispatchStatus = "timed_out"
	StatusFailed    DispatchStatus = "failed"
)

// ── Dispatch Result ──────────────────────────────────────────

type DispatchResult struct {
	SchemaVersion     int               `json:"schema_version"`
	Status            DispatchStatus    `json:"status"`
	DispatchID        string            `json:"dispatch_id"`
	DispatchSalt      string            `json:"dispatch_salt"`
	Response          string            `json:"response"`
	ResponseTruncated bool              `json:"response_truncated"`
	FullOutput        *string           `json:"full_output"`
	HandoffSummary    string            `json:"handoff_summary"`
	Artifacts         []string          `json:"artifacts"`
	Partial           bool              `json:"partial,omitempty"`
	Recoverable       bool              `json:"recoverable,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	Error             *DispatchError    `json:"error,omitempty"`
	Activity          *DispatchActivity `json:"activity"`
	Metadata          *DispatchMetadata `json:"metadata"`
	DurationMS        int64             `json:"duration_ms"`
}

// ── Dispatch Error ───────────────────────────────────────────

type DispatchError struct {
	Code             string   `json:"code"`
	Message          string   `json:"message"`
	Suggestion       string   `json:"suggestion"`
	Retryable        bool     `json:"retryable"`
	PartialArtifacts []string `json:"partial_artifacts,omitempty"`
}

// ── Dispatch Activity ────────────────────────────────────────

type DispatchActivity struct {
	FilesChanged []string `json:"files_changed"`
	FilesRead    []string `json:"files_read"`
	CommandsRun  []string `json:"commands_run"`
	ToolCalls    []string `json:"tool_calls"`
}

// ── Dispatch Metadata ────────────────────────────────────────

type DispatchMetadata struct {
	Engine           string      `json:"engine"`
	Model            string      `json:"model"`
	Role             string      `json:"role,omitempty"`
	Tokens           *TokenUsage `json:"tokens"`
	Turns            int         `json:"turns"`
	CostUSD          float64     `json:"cost_usd"`
	SessionID        string      `json:"session_id,omitempty"`
	PipelineID       string      `json:"pipeline_id,omitempty"`
	ParentDispatchID string      `json:"parent_dispatch_id,omitempty"`
}

// ── Token Usage ──────────────────────────────────────────────

type TokenUsage struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning,omitempty"`
}

// ── Dispatch Spec ────────────────────────────────────────────

type DispatchSpec struct {
	// Identity
	DispatchID string `json:"dispatch_id"`
	Salt       string `json:"salt,omitempty"`

	// Core dispatch parameters
	Engine       string `json:"engine"`
	Model        string `json:"model,omitempty"`
	Effort       string `json:"effort"`
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	Cwd          string `json:"cwd"`

	// Context & skills
	Skills      []string `json:"skills,omitempty"`
	Coordinator string   `json:"coordinator,omitempty"`
	ContextFile string   `json:"context_file,omitempty"`
	ArtifactDir string   `json:"artifact_dir"`

	// Timeout
	TimeoutSec int `json:"timeout_sec,omitempty"`
	GraceSec   int `json:"grace_sec,omitempty"`

	// Role
	Role string `json:"role,omitempty"`

	// Recursive dispatch controls
	MaxDepth         int  `json:"max_depth"`
	AllowSubdispatch bool `json:"allow_subdispatch"`
	Depth            int  `json:"depth"`

	// Lineage
	ParentDispatchID    string `json:"parent_dispatch_id,omitempty"`
	PipelineID          string `json:"pipeline_id,omitempty"`
	PipelineStep        int    `json:"pipeline_step"`
	ContinuesDispatchID string `json:"continues_dispatch_id,omitempty"`

	// Pipeline step data flow
	Receives     string `json:"receives,omitempty"`
	PassOutputAs string `json:"pass_output_as,omitempty"`
	Parallel     int    `json:"parallel,omitempty"`
	HandoffMode  string `json:"handoff_mode,omitempty"`

	// Response control
	ResponseMaxChars int `json:"response_max_chars,omitempty"`

	// Engine-specific passthrough
	EngineOpts map[string]any `json:"engine_opts,omitempty"`

	// Access mode
	FullAccess bool `json:"full_access"`
}

// ── Event Types ──────────────────────────────────────────────

type EventKind int

const (
	EventUnknown        EventKind = iota
	EventToolStart                // Harness began a tool call
	EventToolEnd                  // Harness finished a tool call
	EventFileWrite                // Harness wrote a file
	EventFileRead                 // Harness read a file
	EventCommandRun               // Harness ran a shell command
	EventProgress                 // Free-form progress
	EventResponse                 // Final or partial response text
	EventError                    // Harness-reported error
	EventSessionStart             // Session initialized (carries session ID)
	EventTurnComplete             // Turn finished (carries token counts)
	EventTurnFailed               // Turn failed
	EventRawPassthrough           // Unclassifiable line
)

// ── Harness Event ────────────────────────────────────────────

type HarnessEvent struct {
	Kind       EventKind
	Timestamp  time.Time
	Tool       string       // Set for ToolStart/ToolEnd
	FilePath   string       // Set for FileWrite/FileRead
	Command    string       // Set for CommandRun
	Text       string       // Set for Progress/Response/Error
	SessionID  string       // Set for SessionStart
	DurationMS int64        // Set for ToolEnd/TurnComplete
	Tokens     *TokenUsage  // Set for TurnComplete
	ErrorCode  string       // Set for Error
	Raw        []byte       // Always set (original harness line)
}

// ── Inbox Mode ───────────────────────────────────────────────

type InboxMode int

const (
	InboxDeterministic InboxMode = iota // harness supports resume
	InboxNone                           // engine does not support inbox
)

// ── Engine Interface ─────────────────────────────────────────

type Engine interface {
	Name() string
	ValidModels() []string
	Dispatch(ctx context.Context, spec *DispatchSpec) (*DispatchResult, error)
	InboxMode() InboxMode
}

// ── Harness Adapter Interface ────────────────────────────────

type HarnessAdapter interface {
	Binary() string
	BuildArgs(spec *DispatchSpec) []string
	ParseEvent(line string) (*HarnessEvent, error)
	SupportsResume() bool
	ResumeArgs(sessionID string, message string) []string
}

// ── Pipeline Types (Phase 2, defined now for completeness) ───

type WorkerStatus string

const (
	WorkerCompleted WorkerStatus = "completed"
	WorkerTimedOut  WorkerStatus = "timed_out"
	WorkerFailed    WorkerStatus = "failed"
)

type WorkerResult struct {
	WorkerIndex int          `json:"worker_index"`
	DispatchID  string       `json:"dispatch_id"`
	Status      WorkerStatus `json:"status"`
	Summary     string       `json:"summary"`
	ArtifactDir string       `json:"artifact_dir"`
	OutputFile  string       `json:"output_file,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	ErrorMsg    string       `json:"error_msg,omitempty"`
	DurationMS  int64        `json:"duration_ms"`
}

type StepOutput struct {
	StepName    string         `json:"step_name"`
	StepIndex   int            `json:"step_index"`
	PipelineID  string         `json:"pipeline_id"`
	HandoffMode HandoffMode    `json:"handoff_mode"`
	Workers     []WorkerResult `json:"workers"`
	HandoffText string         `json:"handoff_text"`
	Succeeded   int            `json:"succeeded"`
	Failed      int            `json:"failed"`
	TotalMS     int64          `json:"total_ms"`
}

type HandoffMode string

const (
	HandoffSummaryAndRefs HandoffMode = "summary_and_refs"
	HandoffFullConcat     HandoffMode = "full_concat"
	HandoffRefsOnly       HandoffMode = "refs_only"
)
