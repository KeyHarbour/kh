# Developer Experience Prompts

Use these prompts to drive implementation work for CLI developer-experience improvements.

## 1) Interactive Init Wizard

```text
You are helping implement an interactive init wizard for a Go Cobra CLI.

Goal:
Create a new command kh init wizard that interactively generates:
1) CLI config file
2) backend templates
3) sync config file
4) scheduler snippets (cron + GitHub Actions + systemd timer)

Requirements:
- Non-interactive fallback using flags for CI (--non-interactive and --yes)
- Prompt only for missing values
- Validate UUIDs, URLs, and required tokens
- Support defaults from env vars
- Show a final summary and ask confirmation before writing files
- Support --dry-run to print files that would be generated
- Use existing output style and exit code patterns
- Do not overwrite existing files unless --force

Deliverables:
- Command design and flags
- Go code for command handlers and prompt flow
- File templates with placeholders
- Unit tests for prompt flow and file generation
- Example UX transcript and help text
```

## 2) Explain Mode

```text
Design and implement an explain mode for a Go Cobra CLI.

Goal:
Add --explain to selected commands so users see a plain-language plan before execution.

Behavior:
- When --explain is set, print:
  1) what the command will do
  2) what data it will read/write
  3) side effects and risks
  4) required permissions
  5) expected output and exit codes
- If used with --dry-run, explain and then show plan without applying
- If used with --output json, return explain info as structured JSON
- Keep explanations concise and deterministic, no LLM dependency

Deliverables:
- Reusable explain formatter interface
- Integration in at least 3 commands (one read-only, one mutating, one sync-like)
- Tests for text and JSON explain output
- Updated CLI help examples
```

## 3) Better Error Taxonomy DONE

```text
Refactor CLI error handling into a structured error taxonomy.

Goal:
Standardize errors for automation and operator remediation.

Requirements:
- Introduce error categories: validation, auth, network, permission, conflict, not-found, partial, internal
- Each error must include:
  - stable machine code (e.g. KH-AUTH-001)
  - human message
  - remediation hint
  - optional docs key/url field
- Preserve existing exit code contract and map each category clearly
- Add JSON error envelope for --output json
- Ensure secrets are redacted from all error messages

Deliverables:
- Error type definitions and mapping table
- Conversion of current command errors to taxonomy
- Tests for mapping and redaction
- A short operator runbook table: code -> meaning -> next action
```

## 4) Dynamic Shell Completion

```text
Implement dynamic shell completion enhancements for a Cobra CLI.

Goal:
Improve completion for commands that require live resource names/IDs.

Requirements:
- Dynamic completion for:
  - projects
  - workspaces
  - providers (from registry/config)
- Cache completion data briefly to avoid repeated API calls
- Graceful fallback to static completion when offline/auth fails
- No sensitive values in completion output
- Support bash, zsh, and fish

Deliverables:
- Completion functions wired into command flags/args
- Lightweight cache with TTL
- Error-tolerant completion behavior
- Tests for completion results and offline fallback
- Demo script showing completion behavior
```

## 5) Doctor Command

```text
Create a kh doctor command for environment diagnostics.

Goal:
Provide quick diagnostics for local setup and runtime readiness.

Checks:
1) config presence and validity
2) auth token availability and format
3) endpoint DNS/TLS reachability
4) API auth probe
5) permissions/capabilities probe
6) required env vars for sync profiles
7) optional dependencies (scheduler tools) presence

Output:
- Table mode: check name, status, detail, remediation
- JSON mode: structured checks for CI gating
- Exit rules:
  - 0 all pass
  - 2 warnings only
  - 3 one or more failures

Deliverables:
- kh doctor command with --output and --strict
- Modular check framework so new checks are easy to add
- Unit tests plus one integration smoke test
- Example CI usage (kh doctor --output json --strict)
```

## 6) Meta Prompt for a 6-week Execution Plan

```text
Using the five DX features below, create a 6-week implementation plan with:
- week-by-week milestones
- dependencies between features
- risk list and mitigation
- test strategy
- docs updates
- acceptance criteria per milestone

Features:
1) interactive init wizard
2) explain mode
3) error taxonomy
4) dynamic shell completion
5) doctor command

Assume Go + Cobra CLI, automation-first, on-prem/air-gapped friendly constraints.
```
