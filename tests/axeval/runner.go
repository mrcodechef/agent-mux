//go:build axeval

package axeval

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// dispatch runs agent-mux with the given TestCase and returns a parsed Result.
func dispatch(t *testing.T, binary string, tc TestCase) Result {
	t.Helper()

	artifactDir := t.TempDir()

	// Build the JSON dispatch spec for --stdin mode.
	spec := map[string]any{
		"engine":       tc.Engine,
		"model":        tc.Model,
		"effort":       tc.Effort,
		"prompt":       tc.Prompt,
		"cwd":          tc.CWD,
		"artifact_dir": artifactDir,
		"skip_skills":  tc.SkipSkills,
	}
	if tc.TimeoutSec > 0 {
		spec["timeout_sec"] = tc.TimeoutSec
	}
	if len(tc.EngineOpts) > 0 {
		opts := make(map[string]any, len(tc.EngineOpts))
		for k, v := range tc.EngineOpts {
			opts[k] = v
		}
		spec["engine_opts"] = opts
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal dispatch spec: %v", err)
	}

	// Set up context with wall-clock timeout.
	wallClock := tc.MaxWallClock
	if wallClock == 0 {
		wallClock = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), wallClock)
	defer cancel()

	cmdArgs := []string{"--stdin", "--yes"}
	cmdArgs = append(cmdArgs, tc.ExtraFlags...)
	cmd := exec.CommandContext(ctx, binary, cmdArgs...)
	cmd.Stdin = bytes.NewReader(specJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Context deadline or other error.
			exitCode = -1
		}
	}

	result := Result{
		ArtifactDir: artifactDir,
		Duration:    duration,
		ExitCode:    exitCode,
		RawStdout:   stdout.Bytes(),
		RawStderr:   stderr.Bytes(),
	}

	// Parse stdout as JSON to extract status, response, error fields.
	var raw map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Logf("stdout not valid JSON: %s", stdout.String())
		t.Logf("stderr: %s", stderr.String())
		result.Status = "parse_error"
		result.ErrorMessage = fmt.Sprintf("failed to parse stdout as JSON: %v", err)
		return result
	}

	if s, ok := raw["status"].(string); ok {
		result.Status = s
	}
	if r, ok := raw["response"].(string); ok {
		result.Response = r
	}
	if errObj, ok := raw["error"].(map[string]any); ok {
		if code, ok := errObj["code"].(string); ok {
			result.ErrorCode = code
		}
		if msg, ok := errObj["message"].(string); ok {
			result.ErrorMessage = msg
		}
	}

	// Parse events.jsonl from artifact dir.
	result.Events = parseEvents(artifactDir)

	return result
}

// dispatchAsync runs agent-mux with --async and returns two Results:
// 1. The async ack (from the initial --async dispatch stdout)
// 2. The collected result (from `ax result <id>`)
func dispatchAsync(t *testing.T, binary string, tc TestCase) (ack Result, collected Result) {
	t.Helper()

	// First dispatch with --async.
	ack = dispatch(t, binary, tc)

	// Parse the async_started ack to get the dispatch_id.
	var ackJSON map[string]any
	if err := json.Unmarshal(ack.RawStdout, &ackJSON); err != nil {
		t.Logf("async ack not valid JSON: %s", string(ack.RawStdout))
		return ack, Result{Status: "parse_error", ErrorMessage: "async ack not valid JSON"}
	}

	dispatchID, _ := ackJSON["dispatch_id"].(string)
	if dispatchID == "" {
		return ack, Result{Status: "parse_error", ErrorMessage: "no dispatch_id in async ack"}
	}

	// Run `ax result <id> --json` to collect the result.
	wallClock := tc.MaxWallClock
	if wallClock == 0 {
		wallClock = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), wallClock)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "result", dispatchID, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	collected = Result{
		ArtifactDir: ack.ArtifactDir,
		Duration:    duration,
		ExitCode:    exitCode,
		RawStdout:   stdout.Bytes(),
		RawStderr:   stderr.Bytes(),
	}

	// Parse result JSON.
	var raw map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &raw); err == nil {
		if s, ok := raw["status"].(string); ok {
			collected.Status = s
		}
		if r, ok := raw["response"].(string); ok {
			collected.Response = r
		}
	}

	collected.Events = parseEvents(ack.ArtifactDir)
	return ack, collected
}

// dispatchAsyncSteer dispatches with --async, waits, runs a steer command, then collects the result.
func dispatchAsyncSteer(t *testing.T, binary string, tc TestCase) (ack Result, steerResult Result, collected Result) {
	t.Helper()

	if tc.SteerSpec == nil {
		t.Fatal("dispatchAsyncSteer called without SteerSpec")
	}

	// Step 1: Dispatch with --async.
	ack = dispatch(t, binary, tc)

	// Parse dispatch_id from ack.
	var ackJSON map[string]any
	if err := json.Unmarshal(ack.RawStdout, &ackJSON); err != nil {
		t.Logf("async ack not valid JSON: %s", string(ack.RawStdout))
		return ack, Result{Status: "parse_error", ErrorMessage: "async ack not valid JSON"}, Result{}
	}
	dispatchID, _ := ackJSON["dispatch_id"].(string)
	if dispatchID == "" {
		return ack, Result{Status: "parse_error", ErrorMessage: "no dispatch_id in async ack"}, Result{}
	}

	// Step 2: Wait before steering.
	time.Sleep(tc.SteerSpec.DelayBeforeSteer)

	// Step 3: Run steer command.
	wallClock := tc.MaxWallClock
	if wallClock == 0 {
		wallClock = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), wallClock)
	defer cancel()

	steerArgs := []string{"steer", dispatchID, tc.SteerSpec.Action}
	if tc.SteerSpec.Message != "" {
		steerArgs = append(steerArgs, tc.SteerSpec.Message)
	}

	steerCmd := exec.CommandContext(ctx, binary, steerArgs...)
	var steerStdout, steerStderr bytes.Buffer
	steerCmd.Stdout = &steerStdout
	steerCmd.Stderr = &steerStderr

	steerStart := time.Now()
	steerRunErr := steerCmd.Run()
	steerDuration := time.Since(steerStart)

	steerExit := 0
	if steerRunErr != nil {
		if exitErr, ok := steerRunErr.(*exec.ExitError); ok {
			steerExit = exitErr.ExitCode()
		} else {
			steerExit = -1
		}
	}

	steerResult = Result{
		Duration:  steerDuration,
		ExitCode:  steerExit,
		RawStdout: steerStdout.Bytes(),
		RawStderr: steerStderr.Bytes(),
	}
	// Parse steer JSON output.
	var steerJSON map[string]any
	if err := json.Unmarshal(steerStdout.Bytes(), &steerJSON); err == nil {
		if s, ok := steerJSON["action"].(string); ok {
			steerResult.Status = s
		}
	}

	t.Logf("steer %s result: exit=%d stdout=%s", tc.SteerSpec.Action, steerExit, steerStdout.String())

	// Step 4: Collect result. For abort, use --no-wait since the process may be dead.
	var collectArgs []string
	if tc.SteerSpec.Action == "abort" {
		// Give the process a moment to terminate after SIGTERM.
		time.Sleep(3 * time.Second)
		collectArgs = []string{"status", dispatchID, "--json"}
	} else {
		collectArgs = []string{"result", dispatchID, "--json"}
	}

	collectCtx, collectCancel := context.WithTimeout(context.Background(), wallClock)
	defer collectCancel()

	collectCmd := exec.CommandContext(collectCtx, binary, collectArgs...)
	var collectStdout, collectStderr bytes.Buffer
	collectCmd.Stdout = &collectStdout
	collectCmd.Stderr = &collectStderr

	collectStart := time.Now()
	collectRunErr := collectCmd.Run()
	collectDuration := time.Since(collectStart)

	collectExit := 0
	if collectRunErr != nil {
		if exitErr, ok := collectRunErr.(*exec.ExitError); ok {
			collectExit = exitErr.ExitCode()
		} else {
			collectExit = -1
		}
	}

	collected = Result{
		ArtifactDir: ack.ArtifactDir,
		Duration:    collectDuration,
		ExitCode:    collectExit,
		RawStdout:   collectStdout.Bytes(),
		RawStderr:   collectStderr.Bytes(),
	}

	var raw map[string]any
	if err := json.Unmarshal(collectStdout.Bytes(), &raw); err == nil {
		if s, ok := raw["status"].(string); ok {
			collected.Status = s
		}
		if s, ok := raw["state"].(string); ok {
			collected.Status = s
		}
		if r, ok := raw["response"].(string); ok {
			collected.Response = r
		}
	}

	// Parse artifact dir from ack if available.
	if artDir, ok := ackJSON["artifact_dir"].(string); ok && artDir != "" {
		collected.ArtifactDir = artDir
		collected.Events = parseEvents(artDir)
	}

	return ack, steerResult, collected
}

// dispatchWithFlags runs agent-mux with explicit CLI flags (not --stdin) and returns a Result.
func dispatchWithFlags(t *testing.T, binary string, args []string, wallClock time.Duration) Result {
	t.Helper()

	if wallClock == 0 {
		wallClock = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), wallClock)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := Result{
		Duration:  duration,
		ExitCode:  exitCode,
		RawStdout: stdout.Bytes(),
		RawStderr: stderr.Bytes(),
	}

	var raw map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &raw); err == nil {
		if s, ok := raw["status"].(string); ok {
			result.Status = s
		}
		if s, ok := raw["state"].(string); ok {
			result.Status = s
		}
		if r, ok := raw["response"].(string); ok {
			result.Response = r
		}
	}

	return result
}

// parseEvents reads events.jsonl from the artifact dir and returns parsed events.
func parseEvents(artifactDir string) []Event {
	eventsPath := filepath.Join(artifactDir, "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		return nil // No events file is fine for error cases.
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt Event
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		events = append(events, evt)
	}
	return events
}
