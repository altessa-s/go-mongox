# AGENTS.md

> Project map for AI agents. Keep this file up-to-date as the project evolves.

## Project Overview

go-mongox provides shared BSON, pagination and index primitives for MongoDB consumer adapters.

## Tech Stack

- **Language:** Go 1.27+
- **Type:** Library
- **Dependencies:** go-aip root, go-atlas-core, mongo-driver v2 and shopspring/decimal. Tests also use testify.

## Project Structure

```
go-mongox/
├── bsonbound/     # Bounds BSON encoding by byte size, nesting depth and evaluator budget
├── bsoncodec/     # Registers BSON codecs for optional values, exact decimals and civil dates
├── bsonfilter/    # Validates filters and prepares ordered BSON for execution
├── bsonraw/       # Walks raw BSON and checks element boundaries
├── indexes/       # Resolves index names and creates indexes
├── keyset/        # Reads signed keyset pages from MongoDB collections
└── devtools/       # Pinned development tools
```

## Library rules

- Use the published go-aip root module, go-atlas-core, mongo-driver v2 and shopspring/decimal. Consumer adapters belong in their respective repositories.
- Validate Go inputs before canonicalization and bounded encoding. Bind and execute the checked bytes.
- Preserve BSON types, ordered document keys and historical signer bindings. Keep filter values and tokens out of error messages.
- Index creation is non-destructive. Consumers own reconciliation and collection lifecycle.
- Keep exported APIs stable across consumers. Document the package scope of any proposed dependency.

### Reuse from go-atlas-core

| Reuse from go-atlas-core | Used for                                           |
|--------------------------|----------------------------------------------------|
| `collections/maps`       | Immutable filter operator sets             |
| `time/civil`             | Calendar date representation                       |
| `types/optional`         | Presence-aware BSON encoding and filter validation |
| `types/nilcheck`         | Nil checks in keyset pagination                    |

## Package Layout

Every Go package in this repo MUST ship the following files. Missing any of them is a review blocker.

| File | Purpose |
|------|---------|
| `doc.go` | Package-level godoc. Starts with the 3-line copyright header, then `// Package <name> …` opening sentence, `// # Section` sub-headings, a tab-indented usage snippet, and the bare `package <name>` clause. |
| `README.md` | Human-facing quick reference. H1 is the lowercase package name, followed by a fenced ` ```go ` import block, a 1–2 sentence summary, then `##` sections with aligned markdown tables (`## Options`, `## Key types`, `## Usage`, etc.). Cross-reference siblings with relative links. |
| `options.go` + `options_gen.go` | Required when the package has tunables. `options.go` declares the private `options` struct with `optgen:"default=…"` tags and the `//go:generate optgen generate` directive (`optgen` is installed from `devtools` by `make generate`). `options_gen.go` is produced by `make generate` / `go generate ./…` and carries the `DO NOT EDIT` header. |
| `<name>.go` | Implementation. |
| `<name>_test.go` | Unit tests (see Testing). Required where the package has behavior worth asserting — see [Test-file exemptions](#test-file-exemptions). |
| `<name>_bench_test.go` | Dedicated benchmark file — do not mix benchmarks into the main `_test.go`. At minimum one `BenchmarkXxx` per hot-path function. Use `for b.Loop() { … }`. Required only when the package has a hot path. |

### Test-file exemptions

| Shape | `*_test.go` | `*_bench_test.go` | Why |
|-------|-------------|-------------------|-----|
| Generated options (`options_gen.go`) | not for the generated file | no | `optgen` output is exercised through the code that consumes it. |
| Primitive/pure helpers with no branching, sentinel-only or constant-only packages | only if there is edge-case behavior worth pinning | no | Trivial pass-throughs are covered by their callers. |

The through-line: **write a test when there is behavior a future change could break, and a benchmark only for a function on a hot path.**

### Options pattern

- **Never expose a public `Config` struct** with exported fields. Callers configure packages through generated `Option` values only. The
  `.optgen.yaml` at the repo root sets `all-fields: true`, so every field of the private `options` struct automatically gets a `WithXxx`.
- Declare defaults as exported `Default*` constants referenced from `optgen:"default=DefaultXxx"` tags so godoc users can discover them.
- Mandatory positional arguments (a directory path, a required dependency) stay OUT of the `options` struct — pass them as regular
  constructor parameters (`bsoncodec.RegisterDateType[T](reg, opts...)`). Do not abuse `opt:"-"` for this.
- **Skipping fields from generation:** `opt:"-"` fully skips the field (no `WithXxx`, not initialized in `defaultOptions()`);
  `optgen:"manual"` skips the `WithXxx` but keeps the field's `optgen:"default=…"` in `defaultOptions()`.
- **Hand-written options are allowed only for `opt:"-"` / `optgen:"manual"` fields** whose option is composite: it takes several arguments,
  merges into a map, or sets several fields at once. Keep them in
  `options.go` next to the struct.
- The date codec uses `date_options.go` and generates `DateOption` in `date_options_gen.go`.
- After editing options, run `make generate` and include the regenerated file in the same change.

## Testing

### Conventions

- **Fuzz the parsers.** Code that parses untrusted input (raw BSON, filter values, keyset positions) gets a fuzz target with real assertions.
- **Reuse `internal/testhelpers`** for helpers used across two or more top-level packages; it depends only on the standard library.
- **Assertion library:** `github.com/stretchr/testify/require`, as in go-atlas.

- Gate live MongoDB tests with the `integration` build tag and `MONGO_URI`. Clean up only the collection created by the test.

### Modern Go test style (no exceptions)

- `t.Context()` — never `context.WithCancel(context.Background())`.
- `t.TempDir()` — never `os.MkdirTemp`.
- `t.Setenv(k, v)` — never manual save/restore of env vars.
- `t.Parallel()` by default; serial tests must document the shared global state that forces serialization.
- Table-driven tests with `t.Run(tc.name, func(t *testing.T) { t.Parallel(); … })` for subtests.
- Benchmarks: `for b.Loop() { … }` — never `for i := 0; i < b.N; i++`.
- Error checks: `errors.Is` / `errors.As`. Never string-match error messages.
- Helpers accept `testing.TB` and call `t.Helper()` as their first statement.
- Time goes through the signer’s injected clock in historical continuation tests.
- Prefer **black-box** `package <name>_test`; use white-box `package <name>` only when a test genuinely needs a private helper, and define
  that helper **in the test file**, not in the production source.

## Documentation
| Document | Path | Description |
|----------|------|-------------|
| README | README.md | Project landing page and package listing |
| Contributing | CONTRIBUTING.md | Contribution guidelines |
| Security | SECURITY.md | Vulnerability reporting |

## AI Context Files
| File | Purpose |
|------|---------|
| AGENTS.md | This file — generic project map and conventions for any AI agent |
| CLAUDE.md | Claude Code entry point; imports AGENTS.md and lists Claude-specific notes |
| .ai-factory/DESCRIPTION.md | Project specification and tech stack |
| .ai-factory/ARCHITECTURE.md | Architecture decisions and guidelines |

## Build Commands
| Command | Description |
|---------|-------------|
| `make test` | Run all tests |
| `make test-integration` | Run tagged tests against `MONGO_URI` |
| `make test-all` | Tests with race detector, shuffle, coverage |
| `make vet` | go vet |
| `make lint` | golangci-lint with the repo config (`.golangci.yml`) |
| `make fmt` | Format + sort imports (gci) |
| `make bench` | Run all benchmarks |
| `make fuzz` | Fuzz every target for `FUZZTIME` |
| `make generate` | Run go generate (optgen) |
| `make security-scan` | govulncheck + gosec |
| `make ci` | lint, test-all, security-scan, dupl-check |

## Agent Rules

### Shell

- Never combine shell commands with `&&`, `||`, or `;` — execute each command as a separate Bash tool call. This applies even when a skill,
  plan, or instruction provides a combined command — always decompose it into individual calls.
  - Wrong: `git checkout main && git pull`
  - Right: Two separate Bash tool calls — first `git checkout main`, then `git pull`

### Go source changes

- Target Go 1.27+ language features freely. Prefer built-ins and stdlib helpers (`new(expr)`, `errors.AsType`, `slices`, `maps`, `cmp`,
  `sync.WaitGroup.Go`, `for b.Loop()`, `t.Context()`, `strings.SplitSeq`, `strings.CutLast`, `url.URL.Clone`, `omitzero`) over legacy
  patterns — see the `modern-go-guidelines/use-modern-go` skill for the full list. Tests that wait on time use `testing/synctest` (`synctest.Test`,
  `synctest.Sleep`, `httptest.NewTestServer`) where every blocking point is a channel, timer or in-memory conn.
- Every configurable package goes through `optgen`. Do not hand-write `WithXxx` functions or introduce a public `Config` struct.
- Every package must have `doc.go` and `README.md`, plus `*_test.go` and `*_bench_test.go` unless the package qualifies for a
  [Test-file exemption](#test-file-exemptions). Treat a rename/move as incomplete until these exist in the destination.
- US English in all code, comments, godoc, logs and docs (`misspell` with `locale: US`).

### Refactors and moves

- When renaming an API, search the whole repo for the old identifier and migrate every call site in the same change. Leave no `// deprecated`
  shims unless the user explicitly asks for them.

### Documentation

- **Line length: 152 characters.** Wrap prose in `README.md` and any other Markdown documentation to 152 characters per line —
  don't fold shorter (no sentence-per-line, no 80-column wrap) and don't go longer. Tables, fenced code blocks, headings, and link reference
  lines are exempt.

### Don't do

- Don't propose changes to files you haven't read.
- Don't add features, refactors, or "while I'm here" cleanups beyond what was asked.
- Don't add comments, docstrings, or type annotations to code you didn't change.
- Don't introduce backwards-compatibility shims (`// removed: X`, re-exports, deprecated aliases) when the user hasn't asked for them.
- Don't run `git reset --hard`, `git push --force`, `git clean -f`, or any destructive git operation without explicit instruction.
- Don't skip hooks (`--no-verify`, `--no-gpg-sign`) unless the user explicitly asks.
