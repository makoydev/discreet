# 0002. Same integration-branch workflow as Vetted

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-04

## Context

Michael chose an integration branch for Vetted on 2026-09-30 (Vetted ADR 0009): Claude Code merges each CI-green pull request into a protected `next` branch, labelled `ai-merged`, and Michael reviews `next` → `main` at the end of the milestone. He approved the same approach for Discreet in the Milestone 2 plan on 2026-10-02.

## Options

1. **Reuse Vetted's workflow as is.** One way of working across the programme.
2. **Michael merges every Discreet pull request.** Strongest per-change evidence, but it makes him the bottleneck again, which is why Vetted moved away from it.

## Decision

Option 1. `next` and `main` carry the same protection: pull request required, the same required checks (`Lint and test`, `Analyze (go)`, `Analyze (actions)`, `CodeQL`), linear history, applied to admins, no force pushes. Releases are tagged from `main` only, after Michael's merge.

## Consequences

- Individual changes are reviewed by a human as a batch before `main`, not one by one. The `ai-merged` label and `docs/HOW-THIS-WAS-BUILT.md` say so.
- Vetted reviews every pull request into `next`, so its pilot numbers keep growing with Discreet's development.
