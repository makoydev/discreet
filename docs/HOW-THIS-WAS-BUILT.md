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

## What the AI got wrong, and how it was caught

| Date | What went wrong | How it was caught | Fix |
| --- | --- | --- | --- |
