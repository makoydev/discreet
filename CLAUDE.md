# CLAUDE.md — Discreet

Programme-wide rules are in `../CLAUDE.md` (loaded automatically) and the full brief is `../BRIEF.md` (private, never committed).

Discreet is a privacy-preserving LLM gateway written in Go. Development starts in Milestone 2 (due 22 Nov 2026): proxy with a mock and one real upstream, detectors from `sg-pii-rules` (vendored, checksummed), tokenise and rehydrate, a hash-chained audit log with `verify`, a four-pane demo UI, a benchmark in `EVALS.md`, and a public mock-mode deployment.

Development started on 2026-10-04 (issue D1). Every pull request is reviewed by Vetted, in shadow mode until 2026-10-14 and opt-in afterwards. Work goes through pull requests into `next` (ADR 0002); only Michael merges `next` into `main`.

- Go 1.27, standard library only (ADR 0001); any new module needs an ADR and an exact version.
- Run `make check` (gofmt, vet, race tests, govulncheck, build) before every commit; never commit past a failure.
- Tests use synthetic data only; build secret- and NRIC-shaped strings at runtime.
- Issues live in `makoydev/vetted` under the "M2: Discreet v0.1" milestone (D1 = vetted#30).
