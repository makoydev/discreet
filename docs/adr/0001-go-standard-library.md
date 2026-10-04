# 0001. Build on Go's standard library, without a web framework

Status: drafted by Claude Code, awaiting Michael's review
Date: 2026-10-04

## Context

Discreet is a small HTTP proxy: one endpoint (`POST /v1/chat/completions`), a demo page, and a command-line tool for the audit log. Michael has to be able to read, explain and extend every line, and every dependency is something a security reviewer will ask about.

## Options

1. **Standard library only.** `net/http` has routed on method and path patterns (`POST /v1/chat/completions`) since Go 1.22; `crypto/aes`, `crypto/hmac`, `crypto/sha256`, `encoding/json` and `regexp` (which is RE2) cover the rest.
2. **A web framework such as Gin or Echo.** Adds middleware and binding helpers, plus their dependency trees and a second set of idioms to learn.
3. **A proxy toolkit such as Envoy or a plugin for an existing gateway.** Powerful, but the interesting logic (detection, tokenising, audit) would be buried in configuration and a plugin interface.

## Decision

Option 1. The only planned third-party module is a YAML parser for the policy file (issue D4), chosen and pinned when it is needed. Development tools such as `govulncheck` run at a pinned version and are not linked into the binary.

## Consequences

- One static binary with no runtime dependencies, easy to deploy and audit.
- `govulncheck` in CI only has the Go standard library to check, so its results are easy to act on.
- Some helpers a framework would provide (request binding, middleware chains) are written by hand. They are small, and writing them keeps the code explainable.
