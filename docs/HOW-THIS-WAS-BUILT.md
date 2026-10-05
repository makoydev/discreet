# How this was built

## Who does what

- **Michael Mendoza** owns the project: he set the brief, approved the Milestone 2 plan on 2026-10-02, and reviews `next` → `main` at the end of each milestone.
- **Claude Code** (an AI coding assistant) drafts the code, tests, documents and decision records, opens a pull request for every change, and merges CI-green pull requests into `next` with the `ai-merged` label (ADR 0002).
- **Vetted** reviews every pull request with advisory comments.

## Guardrails

- A private brief and the programme's `CLAUDE.md` set the non-negotiables: no employer material, no real personal data, money capped, humans accountable.
- Every change goes through a pull request with required checks (`make check`, CodeQL) on both `next` and `main`, applied to admins.
- Code is broken deliberately to prove the tests catch it, then restored from a backup.
- Facts that may have changed since the AI's training (tool versions, prices, APIs) are checked against primary sources before use.

## Decisions changed on human review

| Date | Proposed by Claude Code | Changed by Michael to | Where |
| --- | --- | --- | --- |

## How facts and code were checked

| Date | Check | Result |
| --- | --- | --- |
| 2026-10-04 | Latest Go release (go.dev download feed) | Go 1.27.1; the module targets Go 1.27 |
| 2026-10-04 | Latest `actions/setup-go`, `actions/checkout`, CodeQL Action and `govulncheck` releases (GitHub API, Go module proxy) | Pinned v7.0.0, v7.0.1, v4.38.2 and v1.8.0 by commit or version |
| 2026-10-04 | Deliberate break: `discreet serve` made to exit 0 | The test failed (`exit code = 0, want 1`); restored from backup |
| 2026-10-05 | sg-pii-rules `v0.2.0-rc.1` vendored by script from the release tag | Every file matched the release's `SHA256SUMS` |
| 2026-10-05 | All 369 shared conformance cases through the Go engine | 369 / 369 pass, first run |
| 2026-10-05 | Go YAML libraries (Go module proxy, GitHub) | `gopkg.in/yaml.v3` archived since April 2025; used the maintained fork `go.yaml.in/yaml/v3` v3.0.5 |
| 2026-10-05 | Three deliberate breaks of the placeholder session: vault keys without the session, no placeholder reuse, redact restored | Each failed at least one test; restored from backup |
| 2026-10-05 | Five deliberate breaks of the engine and vendored rules (EVALS.md) | Four caught at once. The reversed overlap tie-break was not: no shared case covers it. Added toy-detector overlap tests (now 3 fail on that break) and opened vetted#44 for the shared suite |

## What the AI got wrong, and how it was caught

| Date | What went wrong | How it was caught | Fix |
| --- | --- | --- | --- |
