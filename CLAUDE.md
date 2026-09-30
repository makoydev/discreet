# CLAUDE.md — Discreet

Programme-wide rules are in `../CLAUDE.md` (loaded automatically) and the full brief is `../BRIEF.md` (private, never committed).

Discreet is a privacy-preserving LLM gateway written in Go. Development starts in Milestone 2 (due 22 Nov 2026): proxy with a mock and one real upstream, detectors from `sg-pii-rules` (vendored, checksummed), tokenise and rehydrate, a hash-chained audit log with `verify`, a four-pane demo UI, a benchmark in `EVALS.md`, and a public mock-mode deployment.

Until then this repository only holds the Vetted pilot: every pull request is reviewed by Vetted, in shadow mode until 2026-10-14 and opt-in afterwards. Work goes through pull requests into `next` (ADR 0009 in Vetted); only Michael merges `next` into `main`.
