# Prompting Guide by Engine

Engine-specific prompting tips, model variants, and the golden rules for each engine.

---

## Codex (GPT-5.4)

**The golden rule:** Tell Codex WHAT to read, WHAT to check, and WHERE to write. Never say "explore" or "audit everything."

**What works:**
- One goal per invocation
- Explicit file targets
- Concrete deliverables (patches, tests)
- LOC limits and style constraints
- Bias toward action

**What fails:**
- "Audit the entire codebase."
- Multi-goal prompts
- Upfront planning announcements (causes premature stopping)
- Open-ended exploration

**Model variants:**

| Model | Speed | Context | Best for |
| --- | --- | --- | --- |
| `gpt-5.4` (default) | ~65-70 tok/s | Standard | Primary worker. Thorough, pedantic, complex multi-step tasks. SWE-Bench Pro: 57.7%, Terminal-Bench: 75.1% |
| `gpt-5.4-mini` | 2x+ faster than 5.4 | 272K | Cost-efficient subagent tasks, high-volume work. SWE-Bench Pro: 54.4%, Terminal-Bench: 60.0% |
| `gpt-5.3-codex-spark` | 1000+ tok/s | 128K | Fast grunt work, parallel workers |
| `gpt-5.3-codex` (previous) | ~65-70 tok/s | Standard | Still allowed, previous generation. Terminal-Bench leader at 77.3% |
| `gpt-5.2-codex` (older) | ~65-70 tok/s | Standard | Still allowed, older generation |

**Reasoning levels:**

| Level | Use case | Notes |
| --- | --- | --- |
| `minimal` | Not recommended | Incompatible with MCP tools. Only supported on original `gpt-5` |
| `low` | Trivial fixes | Minimal reasoning overhead |
| `medium` | Routine tasks | Default level |
| `high` | Implementation | Sweet spot for most work |
| `xhigh` | Deep audits only | Overthinks routine work |

---

## Codex Spark

Same prompting discipline as Codex, tighter scope.

**Use for:**
- Parallel workers
- Filesystem scanning
- Docstring generation
- Fast iteration cycles

**Avoid for:**
- Complex multi-file refactors
- Deep reasoning
- Context beyond 128K

Invoke with: `--engine codex --model gpt-5.3-codex-spark`

---

## Codex Config Knobs

agent-mux's `--reasoning` / `-r` flag maps directly to Codex's `model_reasoning_effort` config key. Additional Codex config keys can be forwarded via `-c`:

| Config key | Values | What it does |
| --- | --- | --- |
| `model_reasoning_effort` | `minimal`, `low`, `medium`, `high`, `xhigh` | Controls reasoning depth. Same as `--reasoning` / `-r` flag |
| `model_reasoning_summary` | `none`, `auto`, `concise`, `detailed` | Controls reasoning summary verbosity in output |
| `model_verbosity` | `low`, `medium`, `high` | Controls overall response verbosity |
| `plan_mode_reasoning_effort` | `minimal`, `low`, `medium`, `high`, `xhigh` | Plan-mode-specific reasoning effort override |

**Examples:**
```bash
# Override reasoning effort via -c (equivalent to --reasoning xhigh)
agent-mux --engine codex --cwd /repo -c model_reasoning_effort=xhigh "Deep audit of auth module"

# Combine reasoning with verbosity control
agent-mux --engine codex --cwd /repo --reasoning high -c model_verbosity=low "Fix the bug in parser.ts"

# Detailed reasoning summary for debugging agent behavior
agent-mux --engine codex --cwd /repo --reasoning high -c model_reasoning_summary=detailed "Refactor error handling"
```

---

## Claude (Opus 4.6)

**What works:**
- Open-ended exploration
- Multi-goal when needed
- Writing and documentation
- Prompt crafting for other engines
- Architecture with tradeoff reasoning

**Permission modes:**

| Mode | Use case |
| --- | --- |
| `default` | Interactive use, prompts for permissions |
| `acceptEdits` | Allows file edits without prompts |
| `bypassPermissions` | Full autonomy, default for agent-mux |
| `plan` | Read-only analysis, no mutations |

**Turn scaling:** Use `--max-turns` for effort control. Higher turns = more thorough exploration. Effort level auto-derives turns when unset.

---

## OpenCode

**What works:**
- End-to-end deliverable framing
- Structured output requests
- Cross-checking other engines

**Key presets:**

| Preset | Context | Cost | Strength |
| --- | --- | --- | --- |
| `kimi` | 262K | Paid | Multimodal, largest context |
| `glm-5` | Standard | Paid | Agentic engineering, tool-heavy |
| `opencode-minimax` | Standard | Free | 80% SWE-bench |
| `deepseek-r1` | Standard | Free | Code reasoning |
| `free` | Standard | Free | Zero-cost smoke tests |

---

## Engine Comparison Table

| Aspect | Codex (5.4) | Codex Mini (5.4-mini) | Codex Spark | Claude (Opus 4.6) | OpenCode (varies) |
| --- | --- | --- | --- | --- | --- |
| Speed | ~65-70 tok/s | 2x+ faster than 5.4 | 1000+ tok/s | ~65-70 tok/s | Varies |
| Context | Standard | 272K | 128K | 1M (beta) | 200-262K |
| Prompting | One goal, explicit files | Same as 5.4 | Same, simpler tasks | Open-ended, multi-goal OK | End-to-end deliverables |
| Best for | Implementation, review | High-volume subagent work | Fast grunt work | Architecture, writing | Third opinion, diversity |
| Fails on | Open-ended exploration | Frontier-difficulty tasks | Complex multi-step | Drift without constraints | Vague prompts |

---

## Codex Model Decision Matrix

| Signal | Use Spark | Use Mini (`gpt-5.4-mini`) | Use Regular (`gpt-5.4`) |
| --- | --- | --- | --- |
| Task complexity | Simple, well-scoped | Medium, well-scoped | Multi-step, complex logic |
| Parallelism needed | Yes -- many small workers | Yes -- high-volume subagents | No -- one thorough pass |
| Speed priority | Extreme latency-sensitive | Fast, 2x+ over regular | Quality over speed |
| Context size | Under 128K | Under 272K | Larger context needed |
| Benchmark quality | Adequate for grunt work | Near-frontier (54.4% SWE-Bench Pro) | Frontier (57.7% SWE-Bench Pro) |
| Examples | Docstrings, rename, scan | Batch edits, parallel reviews, cost-efficient subagents | Refactor, debug, implement |
