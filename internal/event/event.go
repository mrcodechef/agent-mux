// Package event handles NDJSON event streaming on stderr.
package event

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// SchemaVersion is included in every event.
const SchemaVersion = 1

// Event represents a single NDJSON event emitted on stderr.
type Event struct {
	SchemaVersion int    `json:"schema_version"`
	Type          string `json:"type"`
	DispatchID    string `json:"dispatch_id,omitempty"`
	Salt          string `json:"salt,omitempty"`
	Timestamp     string `json:"ts"`

	// Type-specific fields (only populated for relevant event types)
	Engine         string `json:"engine,omitempty"`
	Model          string `json:"model,omitempty"`
	ElapsedS       int    `json:"elapsed_s,omitempty"`
	IntervalS      int    `json:"interval_s,omitempty"`
	LastActivity   string `json:"last_activity,omitempty"`
	Tool           string `json:"tool,omitempty"`
	Args           string `json:"args,omitempty"`
	DurationMS     int64  `json:"duration_ms,omitempty"`
	Path           string `json:"path,omitempty"`
	Command        string `json:"command,omitempty"`
	Message        string `json:"message,omitempty"`
	Status         string `json:"status,omitempty"`
	SilenceSeconds int    `json:"silence_seconds,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
}

// Emitter writes NDJSON events to stderr.
type Emitter struct {
	mu           sync.Mutex
	dispatchID   string
	salt         string
	verbose      bool
	eventLog     *os.File // append-only events.jsonl
}

// NewEmitter creates an event emitter for the given dispatch.
func NewEmitter(dispatchID, salt string, verbose bool, eventLogPath string) (*Emitter, error) {
	e := &Emitter{
		dispatchID: dispatchID,
		salt:       salt,
		verbose:    verbose,
	}

	if eventLogPath != "" {
		f, err := os.OpenFile(eventLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open event log: %w", err)
		}
		e.eventLog = f
	}

	return e, nil
}

// Close closes the event log file.
func (e *Emitter) Close() error {
	if e.eventLog != nil {
		return e.eventLog.Close()
	}
	return nil
}

// Emit writes an event to stderr and optionally the event log.
func (e *Emitter) Emit(evt Event) {
	evt.SchemaVersion = SchemaVersion
	evt.DispatchID = e.dispatchID
	evt.Salt = e.salt
	evt.Timestamp = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(evt)
	if err != nil {
		return // silently drop malformed events
	}

	line := append(data, '\n')

	e.mu.Lock()
	defer e.mu.Unlock()

	os.Stderr.Write(line)

	if e.eventLog != nil {
		e.eventLog.Write(line)
	}
}

// EmitDispatchStart emits the dispatch_start event.
func (e *Emitter) EmitDispatchStart(engine, model string) {
	e.Emit(Event{
		Type:   "dispatch_start",
		Engine: engine,
		Model:  model,
	})
}

// EmitDispatchEnd emits the dispatch_end event.
func (e *Emitter) EmitDispatchEnd(status string, durationMS int64) {
	e.Emit(Event{
		Type:       "dispatch_end",
		Status:     status,
		DurationMS: durationMS,
	})
}

// EmitHeartbeat emits a heartbeat event.
func (e *Emitter) EmitHeartbeat(elapsedS, intervalS int, lastActivity string) {
	e.Emit(Event{
		Type:         "heartbeat",
		ElapsedS:     elapsedS,
		IntervalS:    intervalS,
		LastActivity: lastActivity,
	})
}

// EmitToolStart emits a tool_start event.
func (e *Emitter) EmitToolStart(tool, args string) {
	e.Emit(Event{
		Type: "tool_start",
		Tool: tool,
		Args: args,
	})
}

// EmitToolEnd emits a tool_end event.
func (e *Emitter) EmitToolEnd(tool string, durationMS int64) {
	e.Emit(Event{
		Type:       "tool_end",
		Tool:       tool,
		DurationMS: durationMS,
	})
}

// EmitFileWrite emits a file_write event.
func (e *Emitter) EmitFileWrite(path string) {
	e.Emit(Event{
		Type: "file_write",
		Path: path,
	})
}

// EmitFileRead emits a file_read event.
func (e *Emitter) EmitFileRead(path string) {
	e.Emit(Event{
		Type: "file_read",
		Path: path,
	})
}

// EmitCommandRun emits a command_run event.
func (e *Emitter) EmitCommandRun(command string) {
	e.Emit(Event{
		Type:    "command_run",
		Command: command,
	})
}

// EmitProgress emits a progress event.
func (e *Emitter) EmitProgress(message string) {
	e.Emit(Event{
		Type:    "progress",
		Message: message,
	})
}

// EmitTimeoutWarning emits a timeout_warning event.
func (e *Emitter) EmitTimeoutWarning(message string) {
	e.Emit(Event{
		Type:    "timeout_warning",
		Message: message,
	})
}

// EmitFrozenWarning emits a frozen_warning event.
func (e *Emitter) EmitFrozenWarning(silenceSeconds int, message string) {
	e.Emit(Event{
		Type:           "frozen_warning",
		SilenceSeconds: silenceSeconds,
		Message:        message,
	})
}

// EmitCoordinatorInject emits a coordinator_inject event.
func (e *Emitter) EmitCoordinatorInject(message string) {
	e.Emit(Event{
		Type:    "coordinator_inject",
		Message: message,
	})
}

// EmitError emits an error event.
func (e *Emitter) EmitError(code, message string) {
	e.Emit(Event{
		Type:      "error",
		ErrorCode: code,
		Message:   message,
	})
}

// HeartbeatTicker runs a goroutine that emits heartbeats at the given interval.
// Returns a stop function and an activity update function.
func (e *Emitter) HeartbeatTicker(intervalSec int) (stop func(), updateActivity func(string)) {
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	done := make(chan struct{})
	startTime := time.Now()

	var activityMu sync.Mutex
	lastActivity := "initializing"

	go func() {
		for {
			select {
			case <-ticker.C:
				activityMu.Lock()
				act := lastActivity
				activityMu.Unlock()
				elapsed := int(time.Since(startTime).Seconds())
				e.EmitHeartbeat(elapsed, intervalSec, act)
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	var stopOnce sync.Once
	stopFn := func() {
		stopOnce.Do(func() {
			close(done)
		})
	}

	updateFn := func(activity string) {
		activityMu.Lock()
		lastActivity = activity
		activityMu.Unlock()
	}

	return stopFn, updateFn
}
