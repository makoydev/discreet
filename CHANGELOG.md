# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Go 1.27 project skeleton: the `discreet` command with `serve`, `audit verify`, `audit export` and `version`, each describing itself until it is built (issue D1).
- CI on every pull request: `gofmt`, `go vet`, `go test -race`, `govulncheck` and a build (`make check`), plus CodeQL for Go and the workflows. All Actions pinned by commit SHA.
- Evidence files: `CONTROLS.md`, `THREAT_MODEL.md`, `EVALS.md`, `RISKS.md`, `docs/HOW-THIS-WAS-BUILT.md` and the first two decision records.
- Vetted reviews every pull request (shadow mode until 2026-10-14).
