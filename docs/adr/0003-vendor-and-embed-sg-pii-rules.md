# 0003. Vendor sg-pii-rules releases and embed them in the binary

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-05

## Context

Discreet and Vetted must detect the same personal data the same way. The rules live in sg-pii-rules as data (`detectors.json`) with a shared conformance suite, and consumers vendor a tagged release and verify it against `SHA256SUMS` (sg-pii-rules ADR 0004). Discreet also has to run as one self-contained binary on a small host (ADR 0001).

## Options

1. **Vendor a tagged release into `third_party/sgpiirules` and embed it with `go:embed`.** The rules, schemas and fixtures are compiled into the binary; a Go test re-checks every file's SHA-256 on every `go test`.
2. **Import sg-pii-rules as a Go module.** sg-pii-rules is a TypeScript and data repository with no Go module, and module versions would add a second way to say which rules are in use.
3. **Load `detectors.json` from disk at start-up.** Lets an operator change rules without a rebuild, but then the running rules are whatever file is on the machine, not a reviewed release.

## Decision

Option 1. `scripts/vendor-sg-pii-rules.sh <tag>` downloads a release, verifies it against its `SHA256SUMS` and records the tag and commit in `VERSION`. The engine (`internal/detect`) implements `SPEC.md` with Go's `regexp`, which is RE2, the dialect the rules are written in.

## Consequences

- The binary always runs the exact rules of one reviewed release; `discreet` can report which (`VERSION`).
- A hand edit to a vendored file, or a file missing from `SHA256SUMS`, fails CI.
- Changing rules means vendoring a new release and passing its conformance suite: deliberate, reviewable upgrades. Currently vendored: `v0.2.0-rc.1` (`9d40723`); it becomes `v0.2.0`, byte for byte, after Michael's Milestone 2 review.
