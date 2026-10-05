# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Go 1.27 project skeleton: the `discreet` command with `serve`, `audit verify`, `audit export` and `version`, each describing itself until it is built (issue D1).
- CI on every pull request: `gofmt`, `go vet`, `go test -race`, `govulncheck` and a build (`make check`), plus CodeQL for Go and the workflows. All Actions pinned by commit SHA.
- Evidence files: `CONTROLS.md`, `THREAT_MODEL.md`, `EVALS.md`, `RISKS.md`, `docs/HOW-THIS-WAS-BUILT.md` and the first two decision records.
- Vetted reviews every pull request (shadow mode until 2026-10-14).
- Detection engine (`internal/detect`, issue D3): implements the sg-pii-rules specification with Go's `regexp` (RE2), including the five named validators (NRIC/FIN, NRIC look-alikes, payment cards, postal sectors, calendar dates), `value` groups and the overlap rule. Passes all 369 shared conformance cases.
- Placeholders and the vault (`internal/protect`, `internal/vault`, issue D4): personal data is replaced per request with numbered placeholders such as `<NRIC_1>` (the same value gets the same placeholder), redacted as `[REDACTED_CARD]`, or the request is blocked, following a YAML policy (`internal/protect/default-policy.yaml`). Values are kept encrypted in memory with AES-256-GCM for at most 10 minutes and restored only into the same request's answer (ADR 0004). First third-party module: `go.yaml.in/yaml/v3` v3.0.5.
- sg-pii-rules `v0.2.0-rc.1` vendored in `third_party/sgpiirules` and embedded in the binary; a test re-checks every file against `SHA256SUMS` (ADR 0003). `scripts/vendor-sg-pii-rules.sh <tag>` updates it.
