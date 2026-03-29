package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buildoak/agent-mux/internal/store"
)

// ---------------------------------------------------------------------------
// inspect subcommand tests
// ---------------------------------------------------------------------------

func TestInspectCommandOutputsHumanSummary(t *testing.T) {
	isolateHome(t)

	record := testStoreRecord("01KMT4E7BBNN1KQEC8MYJRW5H5", "completed")
	writeStoreRecord(t, record, "inspect test response", true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"inspect", record.ID}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Dispatch ID:",
		record.ID,
		"Status:",
		record.Status,
		"Engine:",
		record.Engine,
		"Model:",
		record.Model,
		"Role:",
		record.Role,
		"Duration:",
		"Started:",
		record.StartedAt,
		"Cwd:",
		"--- Response ---",
		"inspect test response",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want substring %q", out, want)
		}
	}
}

func TestInspectCommandJSONOutput(t *testing.T) {
	isolateHome(t)

	record := testStoreRecord("01KMT4E7BBNN1KQEC8MYJRW5H5", "completed")
	writeStoreRecord(t, record, "json inspect response", true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"inspect", "--json", record.ID}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}

	if result["dispatch_id"] != record.ID {
		t.Fatalf("dispatch_id = %v, want %q", result["dispatch_id"], record.ID)
	}
	if result["response"] != "json inspect response" {
		t.Fatalf("response = %v, want %q", result["response"], "json inspect response")
	}
	rec, ok := result["record"].(map[string]any)
	if !ok {
		t.Fatalf("record = %#v, want object", result["record"])
	}
	if rec["status"] != "completed" {
		t.Fatalf("record.status = %v, want completed", rec["status"])
	}
}

func TestInspectCommandMissingIDReturnsError(t *testing.T) {
	isolateHome(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"inspect"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["kind"] != "error" {
		t.Fatalf("kind = %v, want error", result["kind"])
	}
	errObj, _ := result["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("error.code = %v, want invalid_args", errObj["code"])
	}
}

func TestInspectCommandInvalidIDReturnsError(t *testing.T) {
	isolateHome(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"inspect", "../bad-id"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["kind"] != "error" {
		t.Fatalf("kind = %v, want error", result["kind"])
	}
	errObj, _ := result["error"].(map[string]any)
	if errObj["code"] != "invalid_input" {
		t.Fatalf("error.code = %v, want invalid_input", errObj["code"])
	}
}

func TestInspectCommandNotFoundReturnsError(t *testing.T) {
	isolateHome(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"inspect", "NONEXISTENT1234"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["kind"] != "error" {
		t.Fatalf("kind = %v, want error", result["kind"])
	}
	errObj, _ := result["error"].(map[string]any)
	if errObj["code"] != "not_found" {
		t.Fatalf("error.code = %v, want not_found", errObj["code"])
	}
}

func TestInspectCommandPrefixMatch(t *testing.T) {
	isolateHome(t)

	record := testStoreRecord("01KMT4E7BBNN1KQEC8MYJRW5H5", "completed")
	writeStoreRecord(t, record, "prefix match response", true)

	// Use a 12-character prefix.
	prefix := record.ID[:12]

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"inspect", prefix}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	out := stdout.String()
	// The full ID should appear in the output, resolved from the prefix.
	if !strings.Contains(out, record.ID) {
		t.Fatalf("stdout = %q, want full dispatch ID %q", out, record.ID)
	}
	if !strings.Contains(out, "prefix match response") {
		t.Fatalf("stdout = %q, want response text", out)
	}
}

// ---------------------------------------------------------------------------
// gc subcommand tests
// ---------------------------------------------------------------------------

func TestGCOlderThanDryRunListsRecords(t *testing.T) {
	isolateHome(t)

	// Create old and new records. Old record is 10 days ago.
	oldRecord := testStoreRecordWithTime("01GC0000AAAA1111BBBB2222CC", "completed", time.Now().UTC().Add(-10*24*time.Hour))
	newRecord := testStoreRecordWithTime("01GC0000DDDD3333EEEE4444FF", "completed", time.Now().UTC().Add(-1*time.Hour))

	writeStoreRecord(t, oldRecord, "old result", true)
	writeStoreRecord(t, newRecord, "new result", true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"gc", "--older-than", "7d", "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}
	if result["kind"] != "gc_dry_run" {
		t.Fatalf("kind = %v, want gc_dry_run", result["kind"])
	}
	wouldRemove := int(result["would_remove"].(float64))
	if wouldRemove != 1 {
		t.Fatalf("would_remove = %d, want 1", wouldRemove)
	}
}

func TestGCDeletesAllOldRecordsAndCleanup(t *testing.T) {
	isolateHome(t)

	// Create two records both timestamped 48 hours ago.
	old1 := testStoreRecordWithTime("01GC0000AAAA1111BBBB2222CC", "completed", time.Now().UTC().Add(-48*time.Hour))
	old2 := testStoreRecordWithTime("01GC0000DDDD3333EEEE4444FF", "failed", time.Now().UTC().Add(-48*time.Hour))

	writeStoreRecord(t, old1, "old1 result", true)
	writeStoreRecord(t, old2, "old2 result", true)

	// Create artifact directories for cleanup verification.
	if old1.ArtifactDir != "" {
		if err := os.MkdirAll(old1.ArtifactDir, 0o755); err != nil {
			t.Fatalf("MkdirAll artifact: %v", err)
		}
		if err := os.WriteFile(filepath.Join(old1.ArtifactDir, "meta.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("WriteFile artifact: %v", err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"gc", "--older-than", "1h"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}
	if result["kind"] != "gc" {
		t.Fatalf("kind = %v, want gc", result["kind"])
	}
	removed := int(result["removed"].(float64))
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	kept := int(result["kept"].(float64))
	if kept != 0 {
		t.Fatalf("kept = %d, want 0", kept)
	}

	// Verify records are gone from the store.
	records, err := store.ListRecords("", 0)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records remaining = %d, want 0", len(records))
	}

	// Verify result files were removed.
	storePath := store.DefaultStorePath()
	for _, id := range []string{old1.ID, old2.ID} {
		resultPath := filepath.Join(storePath, "results", id+".md")
		if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
			t.Fatalf("result file %s should be deleted, stat err = %v", resultPath, err)
		}
	}

	// Verify artifact directory was cleaned up.
	if old1.ArtifactDir != "" {
		if _, err := os.Stat(old1.ArtifactDir); !os.IsNotExist(err) {
			t.Fatalf("artifact dir %s should be deleted, stat err = %v", old1.ArtifactDir, err)
		}
	}
}

func TestGCWithoutOlderThanReturnsUsageError(t *testing.T) {
	isolateHome(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"gc"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["kind"] != "error" {
		t.Fatalf("kind = %v, want error", result["kind"])
	}
	errObj, _ := result["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("error.code = %v, want invalid_args", errObj["code"])
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "--older-than is required") {
		t.Fatalf("error.message = %v, want '--older-than is required'", errObj["message"])
	}
}

func TestGCOlderThanFlagParsing(t *testing.T) {
	isolateHome(t)

	// This is the exact scenario from the bug report: gc --older-than 7d --dry-run.
	// Before the fix, normalizeArgs treated "7d" as a positional argument.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"gc", "--older-than", "7d", "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}

	// Should be a gc result (dry_run or gc with 0 removed), not an error.
	kind, _ := result["kind"].(string)
	if kind != "gc_dry_run" && kind != "gc" {
		t.Fatalf("kind = %v, want gc_dry_run or gc (got error?)", result["kind"])
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func testStoreRecordWithTime(id, status string, started time.Time) store.DispatchRecord {
	record := testStoreRecord(id, status)
	record.StartedAt = started.Format(time.RFC3339)
	record.EndedAt = started.Add(5 * time.Minute).Format(time.RFC3339)
	return record
}
