package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/buildoak/agent-mux/internal/dispatch"
	"github.com/buildoak/agent-mux/internal/recovery"
	"github.com/buildoak/agent-mux/internal/sanitize"
	"github.com/buildoak/agent-mux/internal/store"
)

const listDefaultLimit = 20

func runListCommand(args []string, stdout io.Writer) int {
	var flagOutput bytes.Buffer
	fs := flag.NewFlagSet("agent-mux list", flag.ContinueOnError)
	fs.SetOutput(&flagOutput)

	limit := listDefaultLimit
	statusFilter := ""
	jsonOutput := false

	fs.IntVar(&limit, "limit", limit, "Maximum records to print (0 = all)")
	fs.StringVar(&statusFilter, "status", "", "Filter by status: completed, failed, timed_out")
	fs.BoolVar(&jsonOutput, "json", false, "Emit NDJSON")

	if err := fs.Parse(normalizeArgs(args)); err != nil {
		return handleLifecycleParseError(stdout, &flagOutput, err)
	}
	if len(fs.Args()) != 0 {
		return emitLifecycleError(stdout, 2, "invalid_args", "list does not accept positional arguments", "Use --limit, --status, or --json flags only.")
	}
	if limit < 0 {
		return emitLifecycleError(stdout, 1, "invalid_input", fmt.Sprintf("invalid limit %d: must be >= 0", limit), "")
	}

	statusFilter = strings.TrimSpace(statusFilter)
	if statusFilter != "" && !isValidDispatchStatus(statusFilter) {
		return emitLifecycleError(stdout, 1, "invalid_input", fmt.Sprintf("invalid status %q: want completed, failed, or timed_out", statusFilter), "")
	}

	records, err := store.ListRecords("", 0)
	if err != nil {
		return emitLifecycleError(stdout, 1, "store_error", fmt.Sprintf("list dispatch records: %v", err), "")
	}

	filtered := filterRecordsByStatus(records, statusFilter)
	filtered = tailRecords(filtered, limit)

	if jsonOutput {
		for _, record := range filtered {
			writeCompactJSON(stdout, record)
		}
		return 0
	}

	writeRecordTable(stdout, filtered)
	return 0
}

func runStatusCommand(args []string, stdout io.Writer) int {
	var flagOutput bytes.Buffer
	fs := flag.NewFlagSet("agent-mux status", flag.ContinueOnError)
	fs.SetOutput(&flagOutput)

	jsonOutput := false
	fs.BoolVar(&jsonOutput, "json", false, "Emit JSON")

	if err := fs.Parse(normalizeArgs(args)); err != nil {
		return handleLifecycleParseError(stdout, &flagOutput, err)
	}
	if len(fs.Args()) != 1 {
		return emitLifecycleError(stdout, 2, "invalid_args", "status requires exactly one dispatch_id argument", "Pass a full dispatch ID or unique prefix.")
	}

	idPrefix := strings.TrimSpace(fs.Args()[0])
	if err := sanitize.ValidateDispatchID(idPrefix); err != nil {
		return emitLifecycleError(stdout, 1, "invalid_input", fmt.Sprintf("invalid dispatch_id %q: %v", idPrefix, err), "")
	}

	record, err := store.FindRecord("", idPrefix)
	if err != nil {
		return emitLifecycleError(stdout, 1, "lookup_failed", fmt.Sprintf("find dispatch %q: %v", idPrefix, err), "")
	}
	if record == nil {
		return emitLifecycleError(stdout, 1, "not_found", fmt.Sprintf("no dispatch found for prefix %q", idPrefix), "")
	}

	if jsonOutput {
		writeCompactJSON(stdout, record)
		return 0
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Status:\t%s\n", dashIfEmpty(record.Status))
	fmt.Fprintf(tw, "Engine/Model:\t%s\n", formatEngineModel(record.Engine, record.Model))
	fmt.Fprintf(tw, "Duration:\t%s\n", formatDuration(record.DurationMs))
	fmt.Fprintf(tw, "Started:\t%s\n", dashIfEmpty(record.StartedAt))
	fmt.Fprintf(tw, "Truncated:\t%t\n", record.Truncated)
	fmt.Fprintf(tw, "Salt:\t%s\n", dashIfEmpty(record.Salt))
	fmt.Fprintf(tw, "ArtifactDir:\t%s\n", dashIfEmpty(record.ArtifactDir))
	_ = tw.Flush()
	return 0
}

func runResultCommand(args []string, stdout io.Writer) int {
	var flagOutput bytes.Buffer
	fs := flag.NewFlagSet("agent-mux result", flag.ContinueOnError)
	fs.SetOutput(&flagOutput)

	jsonOutput := false
	fs.BoolVar(&jsonOutput, "json", false, "Emit JSON")

	if err := fs.Parse(normalizeArgs(args)); err != nil {
		return handleLifecycleParseError(stdout, &flagOutput, err)
	}
	if len(fs.Args()) != 1 {
		return emitLifecycleError(stdout, 2, "invalid_args", "result requires exactly one dispatch_id argument", "Pass a full dispatch ID or unique prefix.")
	}

	idPrefix := strings.TrimSpace(fs.Args()[0])
	if err := sanitize.ValidateDispatchID(idPrefix); err != nil {
		return emitLifecycleError(stdout, 1, "invalid_input", fmt.Sprintf("invalid dispatch_id %q: %v", idPrefix, err), "")
	}

	record, err := store.FindRecord("", idPrefix)
	if err != nil {
		return emitLifecycleError(stdout, 1, "lookup_failed", fmt.Sprintf("find dispatch %q: %v", idPrefix, err), "")
	}

	dispatchID := idPrefix
	if record != nil && strings.TrimSpace(record.ID) != "" {
		dispatchID = record.ID
	}

	response, err := store.ReadResult("", dispatchID)
	if err != nil {
		if !os.IsNotExist(err) {
			return emitLifecycleError(stdout, 1, "store_error", fmt.Sprintf("read result for dispatch %q: %v", dispatchID, err), "")
		}

		fallbackPath, fallbackErr := legacyFullOutputPath(dispatchID)
		if fallbackErr != nil {
			return emitLifecycleError(stdout, 1, "invalid_input", fmt.Sprintf("invalid dispatch_id %q: %v", dispatchID, fallbackErr), "")
		}
		data, fallbackErr := os.ReadFile(fallbackPath)
		if fallbackErr != nil {
			if os.IsNotExist(fallbackErr) && record == nil {
				return emitLifecycleError(stdout, 1, "not_found", fmt.Sprintf("no stored result found for prefix %q", idPrefix), "Use `agent-mux list` to find a durable dispatch ID.")
			}
			if os.IsNotExist(fallbackErr) {
				return emitLifecycleError(stdout, 1, "not_found", fmt.Sprintf("no stored result found for dispatch %q", dispatchID), "")
			}
			return emitLifecycleError(stdout, 1, "store_error", fmt.Sprintf("read fallback result %q: %v", fallbackPath, fallbackErr), "")
		}
		response = string(data)
	}

	if jsonOutput {
		writeCompactJSON(stdout, map[string]any{
			"dispatch_id": dispatchID,
			"response":    response,
		})
		return 0
	}

	_, _ = io.WriteString(stdout, response)
	return 0
}

func handleLifecycleParseError(stdout io.Writer, flagOutput *bytes.Buffer, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		emitResult(stdout, map[string]any{
			"kind":  "help",
			"usage": strings.TrimSpace(flagOutput.String()),
		})
		return 0
	}
	return emitLifecycleError(stdout, 2, "invalid_args", err.Error(), strings.TrimSpace(flagOutput.String()))
}

func emitLifecycleError(stdout io.Writer, exitCode int, code, message, suggestion string) int {
	emitResult(stdout, map[string]any{
		"kind":  "error",
		"error": dispatch.NewDispatchError(code, message, suggestion),
	})
	return exitCode
}

func filterRecordsByStatus(records []store.DispatchRecord, status string) []store.DispatchRecord {
	if strings.TrimSpace(status) == "" {
		return append([]store.DispatchRecord(nil), records...)
	}

	filtered := make([]store.DispatchRecord, 0, len(records))
	for _, record := range records {
		if record.Status == status {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func tailRecords(records []store.DispatchRecord, limit int) []store.DispatchRecord {
	if limit <= 0 || limit >= len(records) {
		return records
	}
	return records[len(records)-limit:]
}

func isValidDispatchStatus(status string) bool {
	switch status {
	case "completed", "failed", "timed_out":
		return true
	default:
		return false
	}
}

func writeRecordTable(w io.Writer, records []store.DispatchRecord) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSALT\tSTATUS\tENGINE\tMODEL\tDURATION\tCWD")
	for _, record := range records {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			shortDispatchID(record.ID),
			dashIfEmpty(record.Salt),
			dashIfEmpty(record.Status),
			dashIfEmpty(record.Engine),
			dashIfEmpty(record.Model),
			formatDuration(record.DurationMs),
			truncateMiddle(record.Cwd, 48),
		)
	}
	_ = tw.Flush()
}

func shortDispatchID(id string) string {
	runes := []rune(strings.TrimSpace(id))
	if len(runes) <= 12 {
		return string(runes)
	}
	return string(runes[:12])
}

func truncateMiddle(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return "-"
	}
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}

	headLen := (maxRunes - 3) / 2
	tailLen := maxRunes - 3 - headLen
	if headLen == 0 {
		headLen = 1
		tailLen = maxRunes - 4
	}
	return string(runes[:headLen]) + "..." + string(runes[len(runes)-tailLen:])
}

func formatDuration(durationMS int64) string {
	switch {
	case durationMS <= 0:
		return "-"
	case durationMS < 1000:
		return fmt.Sprintf("%dms", durationMS)
	default:
		return fmt.Sprintf("%ds", durationMS/1000)
	}
}

func formatEngineModel(engine, model string) string {
	engine = strings.TrimSpace(engine)
	model = strings.TrimSpace(model)
	switch {
	case engine == "" && model == "":
		return "-"
	case model == "":
		return engine
	case engine == "":
		return model
	default:
		return engine + " / " + model
	}
}

func dashIfEmpty(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func legacyFullOutputPath(dispatchID string) (string, error) {
	artifactDir, err := recovery.DefaultArtifactDir(dispatchID)
	if err != nil {
		return "", err
	}
	return filepath.Join(artifactDir, "full_output.md"), nil
}
