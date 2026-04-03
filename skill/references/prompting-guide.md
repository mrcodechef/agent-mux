# Prompting Guide

Per-engine prompt patterns, context loading, and dispatch/wait/result workflow.

---

## General Rules

Every dispatch prompt should provide:

1. **One job** — a single deliverable, not a compound task
2. **Explicit scope** — files, directories, boundaries
3. **A verification gate** — how the worker proves it is done
4. **A clear output shape** — what to report back

Default prompt frame:

```text
Goal:
Scope:
Constraints:
Verification gate:
Expected output:
```

Do not spend tokens explaining agent-mux to the worker. The worker already
has the harness and artifact directory. Prompt the task, not the tool.

---

## Codex Prompting

Codex executes precision. Give it surgical scope and it delivers clean work.
Give it ambiguity and it hallucinates scope or stalls.

### What works

- Exact file paths (never "explore src/")
- Concrete edits with clear boundaries
- Specific commands to run after the edit
- A hard verification gate
- One deliverable per dispatch

### What fails

- "Explore the repo and tell me what you find"
- Multi-goal prompts with fuzzy priorities
- Hidden verification expectations
- Prompts that require discovering scope

### Recommended shape

```text
Implement X in these files:
- src/parser.go
- internal/parser/types.go

Constraints:
- do not change public API names
- keep changes under 150 LOC

Verification gate:
- go test ./internal/parser ./cmd/tool passes
- malformed input now returns ErrInvalidToken

Report: what changed, which tests ran, any follow-up risk
```

---

## Claude Prompting

Claude handles ambiguity, planning, synthesis, and review.

### Recommended shape

```text
Task: Pressure-test the migration plan for auth.
Scope: read the plan, identify top 3 failure modes, propose safer rollout.
Output: verdict first, then revised sequence, then open questions.
```

For read-only work, use `--permission-mode=plan` or the appropriate role.
If the job is implementation at its core, plan in Claude and hand coding
to Codex via a second dispatch.

---

## Gemini Prompting

Best as a fast alternate take, not primary implementation.

### Recommended shape

```text
Review this patch for hidden risks.
Focus on: backwards compatibility, deployment hazards, missing tests.
Keep the answer to 5 bullets max.
```

Keep Gemini prompts narrower than Claude prompts. Treat as a contrast probe.

---

## Context-Loading Tools

### --skill / "skills"

Reusable runbooks from named skill directories. Keeps prompts composable.
```json
{"role":"lifter","skills":["react","test-writer"],"prompt":"...","cwd":"/repo"}
```

### --context-file / "context_file"

Large briefs already written to disk. Injects preamble telling worker to
read `$AGENT_MUX_CONTEXT` before starting.

### system_prompt / "system_prompt"

Run-level framing, not giant manuals:
```json
{"role":"auditor","system_prompt":"Prioritize regressions over style","prompt":"..."}
```

### --profile / "profile"

Persistent personas with default engine/model. Put stable persona in the
body, dynamic task in the prompt.

---

## Dispatch -> Wait -> Result Pattern

The canonical multi-step workflow using explicit dispatch/wait/result
sequences.

### Sequential handoff (2 workers)

```bash
# Step 1: Plan
ID1=$(agent-mux -R=architect --async -C=/repo "Design the auth migration" 2>/dev/null | jq -r .dispatch_id)

# Wait for plan
agent-mux wait --poll 30s $ID1 2>/dev/null

# Read plan output
PLAN=$(agent-mux result $ID1 --json 2>/dev/null | jq -r .response)

# Step 2: Implement (using plan as context)
printf '{"role":"lifter","prompt":"Implement this plan:\n%s","cwd":"/repo"}' "$PLAN" \
  | agent-mux --stdin --async 2>/dev/null
```

### Sequential handoff via context file

```bash
# Step 1: produce output
ID1=$(agent-mux -R=architect --async -C=/repo "Design the auth migration" 2>/dev/null | jq -r .dispatch_id)
agent-mux wait --poll 30s $ID1 2>/dev/null

# Write plan to file
agent-mux result $ID1 2>/dev/null > /tmp/plan.md

# Step 2: consume via context_file
agent-mux -R=lifter --async --context-file=/tmp/plan.md -C=/repo "Implement the plan at \$AGENT_MUX_CONTEXT" 2>/dev/null
```

### Parallel fan-out

```bash
# Launch parallel workers
ID1=$(agent-mux -R=scout --async -C=/repo "Scan auth module" 2>/dev/null | jq -r .dispatch_id)
ID2=$(agent-mux -R=scout --async -C=/repo "Scan API layer" 2>/dev/null | jq -r .dispatch_id)
ID3=$(agent-mux -R=scout --async -C=/repo "Scan DB layer" 2>/dev/null | jq -r .dispatch_id)

# Wait for all
agent-mux wait $ID1 2>/dev/null
agent-mux wait $ID2 2>/dev/null
agent-mux wait $ID3 2>/dev/null

# Collect results
R1=$(agent-mux result $ID1 --json 2>/dev/null | jq -r .response)
R2=$(agent-mux result $ID2 --json 2>/dev/null | jq -r .response)
R3=$(agent-mux result $ID3 --json 2>/dev/null | jq -r .response)
```

---

## Recovery Phrasing

`--recover` already builds a continuation prompt listing old artifacts.
Your prompt should be the delta.

- Good: "Continue by finishing test coverage for the parser"
- Bad: "Re-explain the entire project from scratch"

### Signal phrasing

Signals become a short resumed turn. Keep them crisp.

- Good: "Focus on auth paths only; skip docs."
- Good: "Stop after green tests and summarize remaining edge cases."
- Bad: Multi-paragraph redesigns or contradictory instructions.

---

## Flag Hygiene

1. **Use `=` for all string-value flags.** `--context-file=/path`, `-C=/repo`.
   Space-separated form can cause Go's flag parser to consume the next flag as the value.
2. **Flags before positional args.** `agent-mux wait --poll 30s <id>`, not
   `agent-mux wait <id> --poll 30s`. Go's flag parser stops at the first
   positional argument.
3. **Escape `$AGENT_MUX_ARTIFACT_DIR` in prompts.** This env var is set in
   the worker's environment, not the coordinator's. Use `\$` or single quotes.
4. **Always `2>/dev/null`.** stderr carries diagnostic events, not output.
   Parse stdout as JSON.
