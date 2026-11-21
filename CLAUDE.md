# CLAUDE.md

> Entry point for [Claude Code](https://claude.com/claude-code) working on
> this repository. Project map, conventions, and agent rules live in
> [`AGENTS.md`](AGENTS.md) — this file imports it and layers Claude-specific
> notes on top.

@AGENTS.md

## Working with this repo as Claude

Follow the conventions in `AGENTS.md` without exception. The rules below are specific to how Claude should operate in this project.

### At the start of every session

**Activate the core Go expertise first, in a single turn, before anything else.** This is a hard precondition for the project — do
it silently, before reading files, before asking clarifying questions, before planning:

> The `SessionStart` hook in `.claude/settings.json` injects this activation directive into every new session automatically —
> honor it as a standing instruction. This section remains the human-readable source of truth: when the skill list changes, update
> **both** the list below and the hook's `additionalContext` so they stay in sync.

#### Essential Skills (always activate):
- `/ecc:golang-patterns` — Go patterns, best practices, idiomatic code
- `/ecc:golang-testing` — Testing strategies, table-driven tests, benchmarks
- `/modern-go-guidelines:use-modern-go` — modern Go features and idioms
- `/antigravity-awesome-skills:golang-pro` — Advanced Go expertise and optimization
- `/go-google-best-practices` — Google's Go style and best practices guide
- `/go-senior-developer` — Senior-level Go architecture, concurrency, error handling, and tooling
- `/go-google-style-guide` — Google's Go code style and formatting rules
- `/ecc:security-scan` — Security analysis for production code

#### Behavioral guidelines (always activate):
- `/andrej-karpathy-skills:karpathy-guidelines` — Karpathy-inspired guardrails against common LLM coding mistakes:
  *think before coding* (surface assumptions and tradeoffs instead of silently picking one),
  *simplicity first* (minimal code that solves only what was asked — no speculative features),
  *surgical changes* (touch only what the task requires — no "while I'm here" cleanups),
  *goal-driven execution* (turn imperative tasks into verifiable success criteria).
  Apply on every non-trivial task; relax for trivial fixes (typos, copyright bumps).

#### Specialized skills (activate as needed):

**For performance optimization:**
- `/antigravity-awesome-skills:performance-optimizer` — Performance analysis and optimization

**For code review and refactoring:**
- `/ecc:go-review` — Code review best practices
- `code-simplifier:agents/code-simplifier` — Code simplification and refactoring

**For concurrency:**
- `/antigravity-awesome-skills:go-concurrency-patterns` — Concurrent patterns (channels, goroutines)

Skills and agents persist for the whole session — do not re-invoke them on later turns. Respect an explicit opt-out (`skip the skills`,
`just do it`). Do not mention the activation in user-facing replies.

### Before you start a task

1. **Scan `AGENTS.md` first** — it has the package map, build commands, library rules and conventions.
2. **Check your memory** at `~/.claude/projects/-Users-antonio-go-src-github-com-altessa-s-go-mongox/memory/` — feedback accumulated across
   sessions lives there and may already answer the user's question or record a constraint you must respect.
3. **Read before editing.** Never propose changes to a file you have not read in the current session.

### While coding

- **Use dedicated tools**, not `bash`: `Read` for files, `Grep` for code search, `Glob` for path patterns, `Edit`/`Write` for edits. Reserve
  `Bash` for things only a shell can do (`go build`, `go test`, `git`).
- **One shell command per `Bash` call.** The repo-wide rule is in `AGENTS.md` — do not chain with `&&`, `||`, or `;`, even when a plan or
  skill provides a combined command. This is enforced, not advisory.
- **Stay in scope.** Do the task the user asked for — no incidental refactors, no unrequested comments, no "while I'm here" cleanups. The
  user prefers minimal, reviewable diffs.
- **Match the surrounding style.** When editing a file, follow the package's existing conventions (error wrapping, receiver names,
  assertion library, logging). Introduce a new style only when the user asks for it.

### Spelling

Use **US English** in all godoc, comments, README, and user-facing strings. The repo's `golangci-lint` config enables the `misspell` linter
and treats British forms as misspellings — defaulting to American variants on first write avoids a full rename pass later.

| Avoid (British) | Use (American)  |
|-----------------|-----------------|
| materialise     | materialize     |
| behaviour       | behavior        |
| honour, honours | honor, honors   |
| organisation    | organization    |
| optimise        | optimize        |
| analyse         | analyze         |

The linter is the source of truth — this applies regardless of the language used in chat.

### Writing Go code

- Target **Go 1.27+** language features freely. Prefer modern built-ins (`new(expr)`, `errors.AsType`, `slices`, `maps`, `cmp`,
  `sync.WaitGroup.Go`, `for b.Loop()`, `t.Context()`, `strings.SplitSeq`, `strings.CutLast`, `url.URL.Clone`, `omitzero`) over legacy
  idioms.
- Bind and execute the prepared BSON bytes. Preserve ordered keys, BSON types and historical signer bindings.
- **Every package ships** `doc.go`, `README.md`, `*_test.go`, and `*_bench_test.go` (see the exemptions in `AGENTS.md`).
- **Configuration is always generated** by `optgen`. Declare a private `options` struct in `options.go`, add the `//go:generate`
  directive, then run `make generate`. Never hand-write `WithXxx` functions and never expose a public `Config` struct.
- **Never import go-atlas** (`AGENTS.md → Library rules`): go-atlas depends on this module.

### Communication

- **Be terse.** Lead with the answer or the diff, not the reasoning. Skip preamble and trailing summaries when the diff speaks for itself.
- **Speak Russian when the user does.** Code, comments, logs and docs stay in US English.
- **Confirm risky actions.** Never run destructive git commands (reset, force-push, clean), never delete untracked files, never commit
  without explicit instruction. Ask before touching shared state.
- **Don't invent tools or APIs.** If you suggest a function or flag from memory, verify it still exists in the current tree before telling the user to use it.

### When you finish

- Run `make vet`, `make test-all` and `make lint`; run `make generate` when an `options.go` changed (it must leave the tree clean).
- Report preexisting failures explicitly and separate them from regressions introduced by the current change. When a test is flaky or an
  unrelated package is broken, say so and show the evidence rather than papering over it.
- Do **not** create a git commit unless the user asks for one.
