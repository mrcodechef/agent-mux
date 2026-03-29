//go:build axeval

package axeval

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var dispatchSaltPattern = regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+$`)

var AllCasesV2 = func() []TestCase {
	cwd := fixtureDir()
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}

	return []TestCase{
		{
			Name:         "output-contract-schema",
			Category:     CatCorrectness,
			Engine:       "codex",
			Model:        "gpt-5.4-mini",
			Effort:       "high",
			Prompt:       "What is 2+2? Answer with just the number.",
			CWD:          cwd,
			TimeoutSec:   120,
			MaxWallClock: 3 * time.Minute,
			SkipSkills:   true,
			Evaluate: compose(
				statusIs("completed"),
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					if schemaVersion, ok := raw["schema_version"].(float64); !ok || schemaVersion != 1 {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("schema_version=%v, want 1", raw["schema_version"])}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "schema_version=1"}
				},
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					if err := requireNonEmptyStringField(raw, "dispatch_id"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "dispatch_id present"}
				},
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					salt, ok := jsonStringField(raw, "dispatch_salt")
					if !ok || !dispatchSaltPattern.MatchString(salt) {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("dispatch_salt=%q does not match word-word-word", salt)}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "dispatch_salt matches pattern"}
				},
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					traceToken, ok := jsonStringField(raw, "trace_token")
					if !ok || !strings.HasPrefix(traceToken, "AGENT_MUX_GO_") {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("trace_token=%q missing AGENT_MUX_GO_ prefix", traceToken)}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "trace_token prefix ok"}
				},
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					activity, err := jsonObjectField(raw, "activity")
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					if err := requirePresentKeys(activity, "files_read", "files_changed", "commands_run", "tool_calls"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("activity: %v", err)}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "activity fields present"}
				},
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					metadata, err := jsonObjectField(raw, "metadata")
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					if err := requireExactStringField(metadata, "engine", "codex"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("metadata: %v", err)}
					}
					if err := requireExactStringField(metadata, "model", "gpt-5.4-mini"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("metadata: %v", err)}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "metadata engine/model match"}
				},
				func(r Result) Verdict {
					raw, err := stdoutJSONObject(r)
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					if err := requirePositiveNumberField(raw, "duration_ms"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "duration_ms > 0"}
				},
			),
		},
		{
			Name:         "artifact-dir-metadata",
			Category:     CatCorrectness,
			Engine:       "codex",
			Model:        "gpt-5.4-mini",
			Effort:       "high",
			Prompt:       "Create a file called proof.txt containing exactly the word exists",
			CWD:          cwd,
			TimeoutSec:   120,
			MaxWallClock: 3 * time.Minute,
			SkipSkills:   true,
			Evaluate: compose(
				statusIs("completed"),
				func(r Result) Verdict {
					meta, err := artifactJSONObject(r, "_dispatch_meta.json")
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					for _, key := range []string{"dispatch_id", "engine", "model", "started_at", "ended_at"} {
						if err := requireNonEmptyStringField(meta, key); err != nil {
							return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("_dispatch_meta.json: %v", err)}
						}
					}
					if err := requireExactStringField(meta, "status", "completed"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("_dispatch_meta.json: %v", err)}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "_dispatch_meta.json fields valid"}
				},
				artifactExists("events.jsonl"),
				func(r Result) Verdict {
					status, err := artifactJSONObject(r, "status.json")
					if err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: err.Error()}
					}
					if err := requireExactStringField(status, "state", "completed"); err != nil {
						return Verdict{Pass: false, Score: 0.0, Reason: fmt.Sprintf("status.json: %v", err)}
					}
					return Verdict{Pass: true, Score: 1.0, Reason: "status.json state=completed"}
				},
			),
		},
	}
}()

func init() {
	AllCases = append(AllCases, AllCasesV2...)
}
