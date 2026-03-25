package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buildoak/agent-mux/internal/dispatch"
	"github.com/buildoak/agent-mux/internal/event"
	"github.com/buildoak/agent-mux/internal/liveness"
	"github.com/buildoak/agent-mux/internal/supervisor"
	"github.com/buildoak/agent-mux/internal/types"
)

// LoopEngine implements the unified engine lifecycle from spec §6.2.
// It starts a harness binary, reads its live event stream, and manages
// supervision (timeout, liveness, heartbeat).
type LoopEngine struct {
	adapter    types.HarnessAdapter
	registry   *Registry
	engineName string
}

// NewLoopEngine creates a LoopEngine for the given adapter.
func NewLoopEngine(engineName string, adapter types.HarnessAdapter, registry *Registry) *LoopEngine {
	return &LoopEngine{
		adapter:    adapter,
		registry:   registry,
		engineName: engineName,
	}
}

// Name returns the engine name.
func (e *LoopEngine) Name() string {
	return e.engineName
}

// ValidModels returns valid models from the registry.
func (e *LoopEngine) ValidModels() []string {
	return e.registry.ValidModels(e.engineName)
}

// InboxMode returns deterministic for Codex (supports resume).
func (e *LoopEngine) InboxMode() types.InboxMode {
	if e.adapter.SupportsResume() {
		return types.InboxDeterministic
	}
	return types.InboxNone
}

// Dispatch executes the full LoopEngine lifecycle (spec §6.2).
func (e *LoopEngine) Dispatch(ctx context.Context, spec *types.DispatchSpec) (*types.DispatchResult, error) {
	startTime := time.Now()

	// Step 1: Ensure artifact dir and write _dispatch_meta.json
	if err := dispatch.EnsureArtifactDir(spec.ArtifactDir); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}
	if err := dispatch.WriteDispatchMeta(spec.ArtifactDir, spec); err != nil {
		return nil, fmt.Errorf("write dispatch meta: %w", err)
	}

	// Set up event emitter
	eventLogPath := filepath.Join(spec.ArtifactDir, "events.jsonl")
	emitter, err := event.NewEmitter(spec.DispatchID, spec.Salt, false, eventLogPath)
	if err != nil {
		return nil, fmt.Errorf("create emitter: %w", err)
	}
	defer emitter.Close()

	emitter.EmitDispatchStart(spec.Engine, spec.Model)

	// Step 2: Build command via HarnessAdapter.BuildArgs()
	args := e.adapter.BuildArgs(spec)

	// Check that the binary exists
	binary := e.adapter.Binary()
	if _, err := exec.LookPath(binary); err != nil {
		durationMS := time.Since(startTime).Milliseconds()
		emitter.EmitDispatchEnd("failed", durationMS)
		return dispatch.BuildFailedResult(
			spec,
			dispatch.NewDispatchError("binary_not_found",
				fmt.Sprintf("Binary %q not found on PATH.", binary),
				fmt.Sprintf("Install %s: see the engine documentation for installation instructions.", binary)),
			emptyActivity(),
			buildMetadata(spec, nil, 0),
			durationMS,
		), nil
	}

	// Step 3: Start harness binary with process group
	env := buildEnv(spec)
	proc := supervisor.NewProcess(ctx, binary, args, spec.Cwd, env)
	cmd := proc.Cmd()

	// Set up stdout pipe for event stream reading
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for error reporting
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := proc.Start(); err != nil {
		durationMS := time.Since(startTime).Milliseconds()
		emitter.EmitDispatchEnd("failed", durationMS)
		return dispatch.BuildFailedResult(
			spec,
			dispatch.NewDispatchError("process_killed",
				fmt.Sprintf("Failed to start %s: %v", binary, err),
				"Check that the binary is installed and accessible."),
			emptyActivity(),
			buildMetadata(spec, nil, 0),
			durationMS,
		), nil
	}

	// Step 4-5: Read event stream + liveness watchdog (concurrent goroutines)

	// Determine timeouts
	softTimeout := time.Duration(spec.TimeoutSec) * time.Second
	gracePeriod := time.Duration(spec.GraceSec) * time.Second

	// Activity tracker
	activity := &types.DispatchActivity{
		FilesChanged: []string{},
		FilesRead:    []string{},
		CommandsRun:  []string{},
		ToolCalls:    []string{},
	}

	// Collected data
	var (
		mu            sync.Mutex
		lastResponse  string
		sessionID     string
		totalTokens   *types.TokenUsage
		turnCount     int
		lastError     *types.HarnessEvent
		terminalState string // "completed", "timed_out", "failed"
		terminalOnce  sync.Once
	)

	setTerminal := func(state string) bool {
		set := false
		terminalOnce.Do(func() {
			mu.Lock()
			terminalState = state
			mu.Unlock()
			set = true
		})
		return set
	}

	// Liveness watchdog
	watchdog := liveness.NewWatchdog(90, 180) // defaults, could come from config

	// Heartbeat ticker
	stopHeartbeat, updateActivity := emitter.HeartbeatTicker(15)
	defer stopHeartbeat()

	// Event stream reader goroutine
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		scanner := bufio.NewScanner(stdout)
		// Increase buffer size for large event lines
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			evt, err := e.adapter.ParseEvent(line)
			if err != nil || evt == nil {
				continue
			}

			// Update liveness watchdog
			watchdog.Touch()

			mu.Lock()
			// Track activity and emit events
			switch evt.Kind {
			case types.EventSessionStart:
				sessionID = evt.SessionID
				updateActivity("session started")

			case types.EventToolStart:
				if evt.Tool != "" {
					activity.ToolCalls = append(activity.ToolCalls, evt.Tool)
				}
				if evt.Command != "" {
					emitter.EmitToolStart(evt.Tool, evt.Command)
					updateActivity(fmt.Sprintf("running: %s", truncate(evt.Command, 60)))
				} else {
					emitter.EmitToolStart(evt.Tool, "")
					updateActivity(fmt.Sprintf("tool: %s", evt.Tool))
				}

			case types.EventToolEnd:
				emitter.EmitToolEnd(evt.Tool, evt.DurationMS)

			case types.EventFileWrite:
				activity.FilesChanged = appendUnique(activity.FilesChanged, evt.FilePath)
				emitter.EmitFileWrite(evt.FilePath)
				updateActivity(fmt.Sprintf("wrote: %s", evt.FilePath))

			case types.EventFileRead:
				activity.FilesRead = appendUnique(activity.FilesRead, evt.FilePath)
				emitter.EmitFileRead(evt.FilePath)

			case types.EventCommandRun:
				activity.CommandsRun = appendUnique(activity.CommandsRun, evt.Command)
				emitter.EmitCommandRun(evt.Command)
				updateActivity(fmt.Sprintf("running: %s", truncate(evt.Command, 60)))

			case types.EventProgress:
				emitter.EmitProgress(truncate(evt.Text, 200))

			case types.EventResponse:
				lastResponse = evt.Text
				updateActivity("received response")

			case types.EventTurnComplete:
				turnCount++
				if evt.Tokens != nil {
					totalTokens = evt.Tokens
				}
				updateActivity("turn completed")

			case types.EventTurnFailed:
				lastError = evt
				updateActivity("turn failed")

			case types.EventError:
				lastError = evt
				emitter.EmitError(evt.ErrorCode, evt.Text)
				updateActivity(fmt.Sprintf("error: %s", evt.ErrorCode))

			case types.EventRawPassthrough:
				// Skip in non-verbose mode
			}
			mu.Unlock()
		}
	}()

	// Liveness watchdog goroutine
	watchdogActions, stopWatchdog := watchdog.RunTicker(5)
	defer stopWatchdog()

	// Timeout timer
	var softTimer, hardTimer <-chan time.Time
	if softTimeout > 0 {
		softTimer = time.After(softTimeout)
	}

	// Wait for completion
	procDone := make(chan error, 1)
	go func() {
		procDone <- proc.Wait()
	}()

	// Main select loop: wait for process exit, timeout, or liveness kill
	var procErr error
	for {
		select {
		case procErr = <-procDone:
			// Process exited
			<-streamDone // Wait for stream to finish
			goto buildResult

		case <-softTimer:
			if setTerminal("timed_out") {
				emitter.EmitTimeoutWarning(fmt.Sprintf("Soft timeout reached. Grace period: %ds.", spec.GraceSec))
				softTimer = nil
				hardTimer = time.After(gracePeriod)
			}

		case <-hardTimer:
			if setTerminal("timed_out") {
				// Already set, just proceed
			}
			watchdog.Terminate()
			proc.GracefulStop(5)
			<-streamDone
			goto buildResult

		case wEvt := <-watchdogActions:
			switch wEvt.Action {
			case liveness.ActionWarn:
				emitter.EmitFrozenWarning(wEvt.SilenceSeconds, fmt.Sprintf("No harness events for %ds.", wEvt.SilenceSeconds))

			case liveness.ActionKill:
				if setTerminal("failed") {
					emitter.EmitError("frozen_tool_call",
						fmt.Sprintf("No harness events for %ds. Likely frozen. Process terminated.", wEvt.SilenceSeconds))
					proc.GracefulStop(5)
					<-streamDone
					goto buildResult
				}
			}

		case <-ctx.Done():
			if setTerminal("failed") {
				proc.GracefulStop(5)
				<-streamDone
				goto buildResult
			}
		}
	}

buildResult:
	stopHeartbeat()
	stopWatchdog()

	durationMS := time.Since(startTime).Milliseconds()

	mu.Lock()
	state := terminalState
	response := lastResponse
	errEvt := lastError
	tokens := totalTokens
	turns := turnCount
	sid := sessionID
	act := activity
	mu.Unlock()

	metadata := buildMetadata(spec, tokens, turns)
	metadata.SessionID = sid

	// Determine result based on terminal state
	switch state {
	case "timed_out":
		emitter.EmitDispatchEnd("timed_out", durationMS)
		dispatch.UpdateDispatchMeta(spec.ArtifactDir, "timed_out", act.FilesChanged)
		return dispatch.BuildTimedOutResult(
			spec,
			response,
			fmt.Sprintf("Soft timeout at %ds, hard kill after %ds grace.", spec.TimeoutSec, spec.GraceSec),
			act,
			metadata,
			durationMS,
		), nil

	case "failed":
		emitter.EmitDispatchEnd("failed", durationMS)
		dispatch.UpdateDispatchMeta(spec.ArtifactDir, "failed", act.FilesChanged)

		var dispErr *types.DispatchError
		if errEvt != nil {
			dispErr = dispatch.NewDispatchError(errEvt.ErrorCode, errEvt.Text, "")
		} else {
			errMsg := "Process failed."
			if stderrBuf.Len() > 0 {
				lines := strings.Split(stderrBuf.String(), "\n")
				tail := lines
				if len(tail) > 5 {
					tail = tail[len(tail)-5:]
				}
				errMsg = fmt.Sprintf("Exit code %d. stderr: %s", proc.ExitCode(), strings.Join(tail, "\n"))
			}
			dispErr = dispatch.NewDispatchError("process_killed", errMsg, "Check engine logs.")
		}
		return dispatch.BuildFailedResult(spec, dispErr, act, metadata, durationMS), nil

	default:
		// Normal completion
		if procErr != nil {
			// Process exited with error but no terminal state set
			emitter.EmitDispatchEnd("failed", durationMS)
			dispatch.UpdateDispatchMeta(spec.ArtifactDir, "failed", act.FilesChanged)

			errMsg := fmt.Sprintf("Exit code %d.", proc.ExitCode())
			if stderrBuf.Len() > 0 {
				lines := strings.Split(stderrBuf.String(), "\n")
				tail := lines
				if len(tail) > 5 {
					tail = tail[len(tail)-5:]
				}
				errMsg += " stderr: " + strings.Join(tail, "\n")
			}

			var dispErr *types.DispatchError
			if errEvt != nil {
				dispErr = dispatch.NewDispatchError(errEvt.ErrorCode, errEvt.Text, "")
			} else {
				dispErr = dispatch.NewDispatchError("process_killed", errMsg, "Check engine logs.")
			}
			return dispatch.BuildFailedResult(spec, dispErr, act, metadata, durationMS), nil
		}

		emitter.EmitDispatchEnd("completed", durationMS)
		dispatch.UpdateDispatchMeta(spec.ArtifactDir, "completed", act.FilesChanged)
		responseMaxChars := spec.ResponseMaxChars
		return dispatch.BuildCompletedResult(spec, response, act, metadata, durationMS, responseMaxChars), nil
	}
}

// ── Helpers ──────────────────────────────────────────────────

func buildEnv(spec *types.DispatchSpec) []string {
	env := os.Environ()

	// Inject agent-mux environment variables
	env = append(env,
		fmt.Sprintf("AGENT_MUX_DISPATCH_ID=%s", spec.DispatchID),
		fmt.Sprintf("AGENT_MUX_ARTIFACT_DIR=%s", spec.ArtifactDir),
		fmt.Sprintf("AGENT_MUX_DEPTH=%d", spec.Depth),
	)

	if spec.ContextFile != "" {
		env = append(env, fmt.Sprintf("AGENT_MUX_CONTEXT=%s", spec.ContextFile))
	}

	return env
}

func buildMetadata(spec *types.DispatchSpec, tokens *types.TokenUsage, turns int) *types.DispatchMetadata {
	if tokens == nil {
		tokens = &types.TokenUsage{}
	}
	return &types.DispatchMetadata{
		Engine: spec.Engine,
		Model:  spec.Model,
		Role:   spec.Role,
		Tokens: tokens,
		Turns:  turns,
	}
}

func emptyActivity() *types.DispatchActivity {
	return &types.DispatchActivity{
		FilesChanged: []string{},
		FilesRead:    []string{},
		CommandsRun:  []string{},
		ToolCalls:    []string{},
	}
}

func appendUnique(slice []string, item string) []string {
	if item == "" {
		return slice
	}
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
