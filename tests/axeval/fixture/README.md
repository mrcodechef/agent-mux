# ax-eval fixture repo

Hermetic test repository for the ax-eval behavioral testing framework.

- `main.go` — Go program with a deliberate off-by-one bug in `processNames`.
- `helpers.py` — Python utility for text table formatting.
- `scripts/` — Shell scripts for liveness and error testing.

## Worker Isolation

This directory has its own `.git` repository. This creates a git boundary so
that workers dispatched with `--cwd fixture/` cannot escape via `../` and read
agent-mux source code. Workers running `git log`, `git diff`, etc. see only
fixture history.

The sandbox mode `workspace` (Codex `engine_opts.sandbox`) further restricts
file access to the CWD subtree. When dispatching to this fixture, set
`engine_opts.sandbox = "workspace"` to enforce read/write boundaries at the
Codex level.

**Maintaining the boundary:** After modifying fixture files, commit inside this
repo (`cd tests/axeval/fixture && git add -A && git commit -m "update fixture"`).
The outer agent-mux repo tracks `fixture/.git` as an untracked directory — this
is intentional.
