// Package dispatch handles DispatchResult construction, error catalog,
// response truncation, and artifact directory lifecycle.
package dispatch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buildoak/agent-mux/internal/types"
	"github.com/oklog/ulid/v2"
)

// ── Salt Generation ──────────────────────────────────────────

var (
	adjectives = []string{
		"amber", "blue", "coral", "dark", "eager", "fair", "gold",
		"hot", "icy", "jade", "keen", "lime", "mint", "navy",
		"oak", "pale", "quick", "red", "sage", "teal",
	}
	nouns = []string{
		"ant", "bear", "cat", "deer", "elk", "fox", "goat",
		"hawk", "ibis", "jay", "koi", "lark", "moth", "newt",
		"owl", "pike", "quail", "ray", "swan", "toad",
	}
	digits = []string{
		"one", "two", "three", "four", "five",
		"six", "seven", "eight", "nine", "zero",
	}
)

// GenerateSalt returns a human-greppable three-word phrase.
func GenerateSalt() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%s-%s-%s",
		adjectives[r.Intn(len(adjectives))],
		nouns[r.Intn(len(nouns))],
		digits[r.Intn(len(digits))],
	)
}

// GenerateDispatchID returns a new ULID string.
func GenerateDispatchID() string {
	return ulid.Make().String()
}

// ── Artifact Directory ───────────────────────────────────────

// EnsureArtifactDir creates the artifact directory with mode 0755.
func EnsureArtifactDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// ── Dispatch Metadata File ───────────────────────────────────

// DispatchMeta is the _dispatch_meta.json written at dispatch start.
type DispatchMeta struct {
	DispatchID          string  `json:"dispatch_id"`
	DispatchSalt        string  `json:"dispatch_salt"`
	StartedAt           string  `json:"started_at"`
	Engine              string  `json:"engine"`
	Model               string  `json:"model"`
	Role                string  `json:"role,omitempty"`
	PromptHash          string  `json:"prompt_hash"`
	Cwd                 string  `json:"cwd"`
	ContinuesDispatchID *string `json:"continues_dispatch_id"`
	EndedAt             string  `json:"ended_at,omitempty"`
	Status              string  `json:"status,omitempty"`
	Artifacts           []string `json:"artifacts,omitempty"`
}

// WriteDispatchMeta writes the _dispatch_meta.json to the artifact dir.
func WriteDispatchMeta(artifactDir string, spec *types.DispatchSpec) error {
	hash := sha256.Sum256([]byte(spec.Prompt))
	meta := DispatchMeta{
		DispatchID:   spec.DispatchID,
		DispatchSalt: spec.Salt,
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
		Engine:       spec.Engine,
		Model:        spec.Model,
		Role:         spec.Role,
		PromptHash:   fmt.Sprintf("sha256:%x", hash[:8]),
		Cwd:          spec.Cwd,
	}
	if spec.ContinuesDispatchID != "" {
		meta.ContinuesDispatchID = &spec.ContinuesDispatchID
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	path := filepath.Join(artifactDir, "_dispatch_meta.json")
	return os.WriteFile(path, data, 0644)
}

// UpdateDispatchMeta updates the metadata file with completion info.
func UpdateDispatchMeta(artifactDir string, status string, artifacts []string) error {
	path := filepath.Join(artifactDir, "_dispatch_meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read meta: %w", err)
	}

	var meta DispatchMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("unmarshal meta: %w", err)
	}

	meta.EndedAt = time.Now().UTC().Format(time.RFC3339)
	meta.Status = status
	meta.Artifacts = artifacts

	data, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// ── Response Truncation ──────────────────────────────────────

// TruncateResponse truncates a response at a sentence boundary.
// Returns (truncated, wasTruncated).
func TruncateResponse(response string, maxChars int) (string, bool) {
	if maxChars <= 0 || len(response) <= maxChars {
		return response, false
	}

	// Find last sentence boundary before maxChars
	truncated := response[:maxChars]
	lastPeriod := strings.LastIndexAny(truncated, ".!?\n")
	if lastPeriod > maxChars/2 {
		truncated = truncated[:lastPeriod+1]
	}

	return truncated, true
}

// WriteFullOutput writes the full response to artifact dir and returns the path.
func WriteFullOutput(artifactDir, response string) (string, error) {
	path := filepath.Join(artifactDir, "full_output.md")
	if err := os.WriteFile(path, []byte(response), 0644); err != nil {
		return "", fmt.Errorf("write full output: %w", err)
	}
	return path, nil
}

// ── Handoff Summary Extraction ───────────────────────────────

// ExtractHandoffSummary extracts a handoff summary from the response.
func ExtractHandoffSummary(response string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 2000
	}

	// Try to find ## Summary or ## Handoff section
	for _, header := range []string{"## Summary", "## Handoff"} {
		idx := strings.Index(response, header)
		if idx >= 0 {
			section := response[idx+len(header):]
			// Find the end of the section (next ## header or end of text)
			nextHeader := strings.Index(section, "\n## ")
			if nextHeader >= 0 {
				section = section[:nextHeader]
			}
			section = strings.TrimSpace(section)
			if len(section) > maxChars {
				section = section[:maxChars]
			}
			return section
		}
	}

	// Fallback: first maxChars chars, truncated at sentence boundary
	if len(response) <= maxChars {
		return response
	}

	truncated := response[:maxChars]
	lastPeriod := strings.LastIndexAny(truncated, ".!?\n")
	if lastPeriod > maxChars/2 {
		truncated = truncated[:lastPeriod+1]
	}

	return truncated
}

// ── Error Code Catalog ───────────────────────────────────────

// ErrorInfo holds metadata about an error code.
type ErrorInfo struct {
	Code       string
	Message    string
	Suggestion string
	Retryable  bool
}

// ErrorCatalog maps error codes to their templates.
var ErrorCatalog = map[string]ErrorInfo{
	"model_not_found": {
		Code:      "model_not_found",
		Retryable: true,
	},
	"engine_not_found": {
		Code:       "engine_not_found",
		Message:    "Unknown engine name.",
		Suggestion: "Valid engines: codex, claude, gemini",
		Retryable:  true,
	},
	"binary_not_found": {
		Code:      "binary_not_found",
		Retryable: false,
	},
	"api_key_missing": {
		Code:      "api_key_missing",
		Retryable: false,
	},
	"api_overloaded": {
		Code:       "api_overloaded",
		Message:    "Provider overloaded (429/529).",
		Suggestion: "Retry in 30s or try --engine with a different provider.",
		Retryable:  true,
	},
	"api_error": {
		Code:      "api_error",
		Retryable: true,
	},
	"frozen_tool_call": {
		Code:       "frozen_tool_call",
		Retryable:  true,
	},
	"invalid_args": {
		Code:      "invalid_args",
		Retryable: true,
	},
	"config_error": {
		Code:      "config_error",
		Retryable: true,
	},
	"process_killed": {
		Code:      "process_killed",
		Retryable: true,
	},
	"recovery_failed": {
		Code:       "recovery_failed",
		Message:    "No artifacts found for the given dispatch ID.",
		Suggestion: "Previous dispatch may not have written artifacts. Check the artifact directory.",
		Retryable:  false,
	},
	"output_parse_error": {
		Code:       "output_parse_error",
		Retryable:  false,
	},
	"skill_not_found": {
		Code:      "skill_not_found",
		Retryable: true,
	},
	"coordinator_not_found": {
		Code:      "coordinator_not_found",
		Retryable: true,
	},
	"prompt_file_missing": {
		Code:      "prompt_file_missing",
		Retryable: true,
	},
	"artifact_dir_unwritable": {
		Code:      "artifact_dir_unwritable",
		Retryable: false,
	},
	"interrupted": {
		Code:       "interrupted",
		Retryable:  false,
	},
}

// NewDispatchError creates a DispatchError from the catalog with custom message/suggestion.
func NewDispatchError(code string, message string, suggestion string) *types.DispatchError {
	info, ok := ErrorCatalog[code]
	if !ok {
		info = ErrorInfo{Code: code, Retryable: false}
	}

	if message == "" {
		message = info.Message
	}
	if suggestion == "" {
		suggestion = info.Suggestion
	}

	return &types.DispatchError{
		Code:       code,
		Message:    message,
		Suggestion: suggestion,
		Retryable:  info.Retryable,
	}
}

// ── Fuzzy Model Matching ─────────────────────────────────────

// FuzzyMatchModel suggests the closest valid model.
func FuzzyMatchModel(input string, validModels []string) string {
	if len(validModels) == 0 {
		return ""
	}

	bestMatch := ""
	bestDist := len(input) + 10

	for _, model := range validModels {
		d := levenshtein(strings.ToLower(input), strings.ToLower(model))
		if d < bestDist {
			bestDist = d
			bestMatch = model
		}
	}

	// Only suggest if within reasonable distance
	if bestDist <= 3 {
		return bestMatch
	}
	return ""
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Result Builders ──────────────────────────────────────────

// BuildCompletedResult builds a DispatchResult for a completed dispatch.
func BuildCompletedResult(spec *types.DispatchSpec, response string, activity *types.DispatchActivity, metadata *types.DispatchMetadata, durationMS int64, responseMaxChars int) *types.DispatchResult {
	result := &types.DispatchResult{
		SchemaVersion:  1,
		Status:         types.StatusCompleted,
		DispatchID:     spec.DispatchID,
		DispatchSalt:   spec.Salt,
		Response:       response,
		HandoffSummary: ExtractHandoffSummary(response, 2000),
		Activity:       activity,
		Metadata:       metadata,
		DurationMS:     durationMS,
	}

	if responseMaxChars > 0 {
		truncated, wasTruncated := TruncateResponse(response, responseMaxChars)
		if wasTruncated {
			result.Response = truncated
			result.ResponseTruncated = true
			fullPath, err := WriteFullOutput(spec.ArtifactDir, response)
			if err == nil {
				result.FullOutput = &fullPath
			}
		}
	}

	// Scan artifact directory
	result.Artifacts = scanArtifacts(spec.ArtifactDir)

	return result
}

// BuildTimedOutResult builds a DispatchResult for a timed-out dispatch.
func BuildTimedOutResult(spec *types.DispatchSpec, response, reason string, activity *types.DispatchActivity, metadata *types.DispatchMetadata, durationMS int64) *types.DispatchResult {
	result := &types.DispatchResult{
		SchemaVersion:  1,
		Status:         types.StatusTimedOut,
		DispatchID:     spec.DispatchID,
		DispatchSalt:   spec.Salt,
		Response:       response,
		HandoffSummary: ExtractHandoffSummary(response, 2000),
		Partial:        true,
		Recoverable:    true,
		Reason:         reason,
		Activity:       activity,
		Metadata:       metadata,
		DurationMS:     durationMS,
	}

	result.Artifacts = scanArtifacts(spec.ArtifactDir)

	return result
}

// BuildFailedResult builds a DispatchResult for a failed dispatch.
func BuildFailedResult(spec *types.DispatchSpec, dispatchErr *types.DispatchError, activity *types.DispatchActivity, metadata *types.DispatchMetadata, durationMS int64) *types.DispatchResult {
	result := &types.DispatchResult{
		SchemaVersion:  1,
		Status:         types.StatusFailed,
		DispatchID:     spec.DispatchID,
		DispatchSalt:   spec.Salt,
		Response:       "",
		HandoffSummary: dispatchErr.Message,
		Error:          dispatchErr,
		Activity:       activity,
		Metadata:       metadata,
		DurationMS:     durationMS,
	}

	result.Artifacts = scanArtifacts(spec.ArtifactDir)

	return result
}

// scanArtifacts lists files in the artifact directory (non-recursive, excludes meta files).
func scanArtifacts(dir string) []string {
	if dir == "" {
		return []string{}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}

	var artifacts []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "_dispatch_meta.json" || name == "events.jsonl" || name == "inbox.md" {
			continue
		}
		artifacts = append(artifacts, filepath.Join(dir, name))
	}

	if artifacts == nil {
		return []string{}
	}
	return artifacts
}
