package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/buildoak/agent-mux/internal/config"
	"github.com/buildoak/agent-mux/internal/dispatch"
	"github.com/buildoak/agent-mux/internal/engine"
	"github.com/buildoak/agent-mux/internal/engine/adapter"
	"github.com/buildoak/agent-mux/internal/types"
	"github.com/oklog/ulid/v2"
)

const version = "agent-mux v2.0.0-dev"

type stringSlice []string

func (s *stringSlice) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type cliFlags struct {
	engine           string
	role             string
	cwd              string
	model            string
	effort           string
	timeout          int
	systemPrompt     string
	systemPromptFile string
	coordinator      string
	skills           stringSlice
	pipeline         string
	recover          string
	contextFile      string
	artifactDir      string
	salt             string
	config           string
	output           string
	full             bool
	noFull           bool
	promptFile       string
	maxDepth         int
	noSubdispatch    bool
	signals          stringSlice
	permissionMode   string
	stdin            bool
	responseMaxChars int
	verbose          bool
	version          bool
	help             bool
	sandbox          string
	reasoning        string
	maxTurns         int
	addDirs          stringSlice
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags, positional, fs, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}

	if flags.help {
		fs.Usage()
		return 0
	}

	if flags.version {
		fmt.Fprintln(stdout, version)
		return 0
	}

	var spec *types.DispatchSpec
	if flags.stdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "read stdin: %v\n", err)
			return 1
		}
		var parsed types.DispatchSpec
		if err := json.Unmarshal(data, &parsed); err != nil {
			fmt.Fprintf(stderr, "parse stdin JSON: %v\n", err)
			return 1
		}
		spec = &parsed
	} else {
		spec, err = buildDispatchSpecE(flags, positional)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	// Load config
	cfg, err := config.LoadConfig(flags.config, spec.Cwd)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	// Resolve role if specified
	if flags.role != "" {
		role, err := config.ResolveRole(cfg, flags.role)
		if err != nil {
			result := dispatch.BuildFailedResult(spec,
				dispatch.NewDispatchError("config_error", err.Error(), ""),
				&types.DispatchActivity{FilesChanged: []string{}, FilesRead: []string{}, CommandsRun: []string{}, ToolCalls: []string{}},
				&types.DispatchMetadata{Engine: spec.Engine, Tokens: &types.TokenUsage{}}, 0)
			writeResult(stdout, result)
			return 1
		}
		// Role fills in missing values (CLI flags override)
		if spec.Engine == "" && role.Engine != "" {
			spec.Engine = role.Engine
		}
		if spec.Model == "" && role.Model != "" {
			spec.Model = role.Model
		}
		if spec.Effort == "high" && role.Effort != "" {
			// Only override if effort wasn't explicitly set
			spec.Effort = role.Effort
		}
	}

	// Apply config defaults for missing values
	if spec.Engine == "" {
		spec.Engine = cfg.Defaults.Engine
	}
	if spec.Model == "" {
		spec.Model = cfg.Defaults.Model
	}

	// Resolve timeout
	if spec.TimeoutSec == 0 {
		spec.TimeoutSec = config.TimeoutForEffort(cfg, spec.Effort)
	}
	if spec.GraceSec == 0 {
		spec.GraceSec = cfg.Timeout.Grace
	}

	// Generate salt if not provided
	if spec.Salt == "" {
		spec.Salt = dispatch.GenerateSalt()
	}

	// Validate engine
	if spec.Engine == "" {
		result := dispatch.BuildFailedResult(spec,
			dispatch.NewDispatchError("invalid_args", "No engine specified.", "Use --engine codex, --engine claude, or --engine gemini."),
			&types.DispatchActivity{FilesChanged: []string{}, FilesRead: []string{}, CommandsRun: []string{}, ToolCalls: []string{}},
			&types.DispatchMetadata{Tokens: &types.TokenUsage{}}, 0)
		writeResult(stdout, result)
		return 1
	}

	// Build engine registry
	registry := engine.NewRegistry()

	// Register Codex adapter
	codexModels := cfg.Models["codex"]
	if len(codexModels) == 0 {
		codexModels = []string{"gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex-spark", "gpt-5.2-codex"}
	}
	registry.Register("codex", adapter.NewCodexAdapter(), codexModels)

	// Get adapter for requested engine
	adp, err := registry.GetAdapter(spec.Engine)
	if err != nil {
		validEngines := registry.EngineNames()
		result := dispatch.BuildFailedResult(spec,
			dispatch.NewDispatchError("engine_not_found",
				fmt.Sprintf("Engine %q not found.", spec.Engine),
				fmt.Sprintf("Valid engines: %v", validEngines)),
			&types.DispatchActivity{FilesChanged: []string{}, FilesRead: []string{}, CommandsRun: []string{}, ToolCalls: []string{}},
			&types.DispatchMetadata{Engine: spec.Engine, Tokens: &types.TokenUsage{}}, 0)
		writeResult(stdout, result)
		return 1
	}

	// Validate model if specified
	if spec.Model != "" {
		validModels := registry.ValidModels(spec.Engine)
		if len(validModels) > 0 {
			found := false
			for _, m := range validModels {
				if m == spec.Model {
					found = true
					break
				}
			}
			if !found {
				suggestion := dispatch.FuzzyMatchModel(spec.Model, validModels)
				suggestionText := fmt.Sprintf("Valid models for %s: %v", spec.Engine, validModels)
				if suggestion != "" {
					suggestionText = fmt.Sprintf("Did you mean %q? %s", suggestion, suggestionText)
				}
				result := dispatch.BuildFailedResult(spec,
					dispatch.NewDispatchError("model_not_found",
						fmt.Sprintf("Model %q not found for engine %s.", spec.Model, spec.Engine),
						suggestionText),
					&types.DispatchActivity{FilesChanged: []string{}, FilesRead: []string{}, CommandsRun: []string{}, ToolCalls: []string{}},
					&types.DispatchMetadata{Engine: spec.Engine, Model: spec.Model, Tokens: &types.TokenUsage{}}, 0)
				writeResult(stdout, result)
				return 1
			}
		}
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create LoopEngine and dispatch
	eng := engine.NewLoopEngine(spec.Engine, adp, registry)
	result, err := eng.Dispatch(ctx, spec)
	if err != nil {
		fmt.Fprintf(stderr, "dispatch error: %v\n", err)
		return 1
	}

	writeResult(stdout, result)
	return 0
}

func writeResult(w io.Writer, result *types.DispatchResult) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

func newFlagSet(stderr io.Writer) (*flag.FlagSet, *cliFlags) {
	flags := &cliFlags{
		effort:    "high",
		output:    "json",
		full:      true,
		maxDepth:  2,
		sandbox:   "danger-full-access",
		reasoning: "medium",
	}

	fs := flag.NewFlagSet("agent-mux", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: agent-mux [options] <prompt>")
		fs.PrintDefaults()
	}

	fs.StringVar(&flags.engine, "engine", "", "Engine name")
	fs.StringVar(&flags.engine, "E", "", "Engine name")
	fs.StringVar(&flags.role, "role", "", "Role")
	fs.StringVar(&flags.role, "R", "", "Role")
	fs.StringVar(&flags.cwd, "cwd", "", "Working directory")
	fs.StringVar(&flags.cwd, "C", "", "Working directory")
	fs.StringVar(&flags.model, "model", "", "Model")
	fs.StringVar(&flags.model, "m", "", "Model")
	fs.StringVar(&flags.effort, "effort", "high", "Effort")
	fs.StringVar(&flags.effort, "e", "high", "Effort")
	fs.IntVar(&flags.timeout, "timeout", 0, "Timeout seconds")
	fs.IntVar(&flags.timeout, "t", 0, "Timeout seconds")
	fs.StringVar(&flags.systemPrompt, "system-prompt", "", "System prompt")
	fs.StringVar(&flags.systemPrompt, "s", "", "System prompt")
	fs.StringVar(&flags.systemPromptFile, "system-prompt-file", "", "System prompt file")
	fs.StringVar(&flags.coordinator, "coordinator", "", "Coordinator")
	fs.Var(&flags.skills, "skill", "Skill name")
	fs.StringVar(&flags.pipeline, "pipeline", "", "Pipeline ID")
	fs.StringVar(&flags.pipeline, "P", "", "Pipeline ID")
	fs.StringVar(&flags.recover, "recover", "", "Recover dispatch ID")
	fs.StringVar(&flags.contextFile, "context-file", "", "Context file")
	fs.StringVar(&flags.artifactDir, "artifact-dir", "", "Artifact directory")
	fs.StringVar(&flags.salt, "salt", "", "Dispatch salt")
	fs.StringVar(&flags.config, "config", "", "Config path")
	fs.StringVar(&flags.output, "output", "json", "Output mode")
	fs.StringVar(&flags.output, "o", "json", "Output mode")
	fs.BoolVar(&flags.full, "full", true, "Full access mode")
	fs.BoolVar(&flags.full, "f", true, "Full access mode")
	fs.BoolVar(&flags.noFull, "no-full", false, "Disable full access mode")
	fs.StringVar(&flags.promptFile, "prompt-file", "", "Prompt file")
	fs.IntVar(&flags.maxDepth, "max-depth", 2, "Maximum recursive depth")
	fs.BoolVar(&flags.noSubdispatch, "no-subdispatch", false, "Disable recursive dispatch")
	fs.Var(&flags.signals, "signal", "Signal payload")
	fs.StringVar(&flags.permissionMode, "permission-mode", "", "Permission mode")
	fs.BoolVar(&flags.stdin, "stdin", false, "Read DispatchSpec JSON from stdin")
	fs.IntVar(&flags.responseMaxChars, "response-max-chars", 0, "Maximum response characters")
	fs.BoolVar(&flags.verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&flags.verbose, "v", false, "Verbose logging")
	fs.BoolVar(&flags.version, "version", false, "Show version")
	fs.BoolVar(&flags.version, "V", false, "Show version")
	fs.BoolVar(&flags.help, "help", false, "Show help")
	fs.BoolVar(&flags.help, "h", false, "Show help")

	fs.StringVar(&flags.sandbox, "sandbox", "danger-full-access", "Sandbox mode")
	fs.StringVar(&flags.reasoning, "reasoning", "medium", "Reasoning effort")
	fs.StringVar(&flags.reasoning, "r", "medium", "Reasoning effort")
	fs.IntVar(&flags.maxTurns, "max-turns", 0, "Maximum turns")
	fs.Var(&flags.addDirs, "add-dir", "Additional writable directory")

	return fs, flags
}

func parseFlags(args []string, stderr io.Writer) (cliFlags, []string, *flag.FlagSet, error) {
	fs, flags := newFlagSet(stderr)
	if err := fs.Parse(args); err != nil {
		return cliFlags{}, nil, fs, err
	}
	return *flags, fs.Args(), fs, nil
}

func buildDispatchSpec(flags cliFlags, args []string) *types.DispatchSpec {
	spec, err := buildDispatchSpecE(flags, args)
	if err != nil {
		return nil
	}
	return spec
}

func buildDispatchSpecE(flags cliFlags, args []string) (*types.DispatchSpec, error) {
	if flags.promptFile != "" && len(args) > 0 {
		return nil, errors.New("prompt must come from either the first positional arg or --prompt-file, not both")
	}

	prompt, err := resolvePrompt(flags.promptFile, args)
	if err != nil {
		return nil, err
	}
	if prompt == "" {
		return nil, errors.New("missing prompt: provide the first positional arg or --prompt-file")
	}

	systemPrompt, err := resolveSystemPrompt(flags.systemPrompt, flags.systemPromptFile)
	if err != nil {
		return nil, err
	}

	dispatchID := ulid.Make().String()
	cwd := flags.cwd
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	artifactDir := flags.artifactDir
	if artifactDir == "" {
		artifactDir = filepath.ToSlash(filepath.Join("/tmp/agent-mux", dispatchID)) + "/"
	}

	fullAccess := flags.full
	if flags.noFull {
		fullAccess = false
	}

	allowSubdispatch := true
	if flags.noSubdispatch {
		allowSubdispatch = false
	}

	engineOpts := map[string]any{
		"sandbox":         flags.sandbox,
		"reasoning":       flags.reasoning,
		"max-turns":       flags.maxTurns,
		"add-dir":         []string(flags.addDirs),
		"permission-mode": flags.permissionMode,
	}

	spec := &types.DispatchSpec{
		DispatchID:          dispatchID,
		Salt:                flags.salt,
		Engine:              flags.engine,
		Model:               flags.model,
		Effort:              flags.effort,
		Prompt:              prompt,
		SystemPrompt:        systemPrompt,
		Cwd:                 cwd,
		Skills:              append([]string(nil), flags.skills...),
		Coordinator:         flags.coordinator,
		ContextFile:         flags.contextFile,
		ArtifactDir:         artifactDir,
		TimeoutSec:          flags.timeout,
		GraceSec:            60,
		Role:                flags.role,
		MaxDepth:            flags.maxDepth,
		AllowSubdispatch:    allowSubdispatch,
		PipelineID:          flags.pipeline,
		PipelineStep:        -1,
		ContinuesDispatchID: flags.recover,
		HandoffMode:         string(types.HandoffSummaryAndRefs),
		ResponseMaxChars:    flags.responseMaxChars,
		EngineOpts:          engineOpts,
		FullAccess:          fullAccess,
	}

	return spec, nil
}

func resolvePrompt(promptFile string, args []string) (string, error) {
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file %q: %w", promptFile, err)
		}
		return string(data), nil
	}
	if len(args) == 0 {
		return "", nil
	}
	return args[0], nil
}

func resolveSystemPrompt(inline, file string) (string, error) {
	parts := make([]string, 0, 2)
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read system prompt file %q: %w", file, err)
		}
		if len(data) > 0 {
			parts = append(parts, string(data))
		}
	}
	if inline != "" {
		parts = append(parts, inline)
	}
	return strings.Join(parts, "\n\n"), nil
}
