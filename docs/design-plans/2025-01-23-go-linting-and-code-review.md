# Go Linting, Pre-commit Hooks, and Code Review Design

## Summary

The cc_session_mon project currently uses golangci-lint for code quality checks but lacks explicit configuration or pre-commit enforcement. This design adds two layers of automated quality assurance to the development workflow: a strict `.golangci.yml` configuration that enforces correctness, security, performance, and style standards across the codebase, and lightweight pre-commit hooks that run fast linters on staged files before commits are allowed. These tools catch issues early in the development cycle while maintaining developer productivity.

A one-time comprehensive code review document complements these automated checks by analyzing the entire codebase for algorithm errors, logical bugs, and efficiency problems that static analysis tools cannot detect.

## Definition of Done

1. **`.golangci.yml`** - Strict golangci-lint configuration with linters for code quality, security, performance, and style
2. **`.pre-commit-config.yaml`** - Lightweight pre-commit hooks running go fmt, go vet, and a fast golangci-lint subset
3. **Makefile updates** - Any needed changes to support the new tooling
4. **Comprehensive code review document** - Analysis of the existing codebase identifying:
   - Linting violations found by the new strict config
   - Algorithm errors or logical bugs
   - Computation inefficiencies (unnecessary work, O(n^2) when O(n) possible, etc.)
   - Memory inefficiencies (unnecessary allocations, leaks, large copies, etc.)

## Glossary

- **golangci-lint**: A fast, parallel-running Go linter aggregator that combines multiple linters (govet, staticcheck, gosec, etc.) into a single tool with unified configuration and output
- **Pre-commit hooks**: Git hooks that run automatically before a commit is created, allowing validation of staged changes; can prevent commits that fail checks
- **Linter**: Automated tool that analyzes code without executing it to find style violations, potential bugs, security issues, and other problems
- **Static analysis**: Code analysis performed without running the program, used to detect defects and code quality issues
- **--fast flag**: golangci-lint option that runs only fast linters on specific files, skipping slow checks and full repository analysis

## Architecture

This design adds static analysis tooling and a one-time comprehensive code review to the cc_session_mon project.

**Components:**

1. **golangci-lint configuration** (`.golangci.yml`) — Strict linter configuration enabling correctness, security, and performance linters. Serves as the source of truth for code quality standards.

2. **Pre-commit hooks** (`.pre-commit-config.yaml`) — Lightweight git hooks using the official golangci-lint pre-commit integration. Runs fast linters only on staged files.

3. **Code review document** (`docs/code-review-2025-01-23.md`) — One-time comprehensive analysis combining automated linting results with AI-assisted review for algorithm, logic, and efficiency issues.

**Data flow:**

```
Developer commits code
    → pre-commit triggers
    → golangci-lint runs with --fast flag (staged files only)
    → Pass: commit proceeds
    → Fail: commit blocked, developer fixes issues
```

Full linting (without --fast) runs via `make lint` for comprehensive checks.

## Existing Patterns

Investigation found an existing `make lint` target in the Makefile that calls `golangci-lint run`. No `.golangci.yml` configuration exists — the project uses golangci-lint defaults.

This design:
- Preserves the existing `make lint` workflow
- Adds explicit configuration via `.golangci.yml`
- Adds pre-commit hooks as a new pattern (none exist currently)

## Implementation Phases

### Phase 1: golangci-lint Configuration

**Goal:** Create strict linter configuration optimized for code quality, security, and performance.

**Components:**
- `.golangci.yml` at project root with:
  - Correctness linters: govet, staticcheck, errcheck, ineffassign, exhaustive
  - Security linters: gosec
  - Performance linters: bodyclose, prealloc, noctx, copyloopvar
  - Quality linters: gocyclo, funlen, gocritic, unconvert, unparam, misspell
  - Relaxed rules for `*_test.go` files
  - 3-minute timeout for full runs

**Dependencies:** None

**Done when:** `golangci-lint run` executes with the new configuration and reports findings (or passes cleanly).

### Phase 2: Pre-commit Hooks

**Goal:** Add lightweight pre-commit hooks that run fast linters on staged files.

**Components:**
- `.pre-commit-config.yaml` at project root using:
  - Official `golangci-lint` hook from `github.com/golangci/golangci-lint`
  - `--fast` and `--fix` flags for speed and auto-correction
  - `fail_fast: false` to run all checks

**Dependencies:** Phase 1 (golangci-lint config must exist)

**Done when:** `pre-commit install` succeeds and `pre-commit run --all-files` executes the hooks.

### Phase 3: Comprehensive Code Review

**Goal:** Analyze the entire codebase for linting issues, algorithm errors, and efficiency problems.

**Components:**
- `docs/code-review-2025-01-23.md` containing:
  - Linting findings from strict golangci-lint run
  - AI-assisted analysis of each source file for:
    - Algorithm and logic errors
    - Computation inefficiencies
    - Memory inefficiencies
  - Categorized findings with file paths, line numbers, severity, and recommendations

**Dependencies:** Phase 1 (need strict config to generate linting findings)

**Done when:** Code review document is complete with all findings categorized and actionable.

## Additional Considerations

**Linter version pinning:** The `.pre-commit-config.yaml` pins a specific golangci-lint version via `rev`. This should be updated periodically to get new linter rules.

**CI integration:** This design focuses on local development. The same `.golangci.yml` can be used in CI pipelines without modification — just remove `--fast` flag for comprehensive checks.
