# Discreet

A privacy-preserving gateway for large language models: an OpenAI-compatible proxy that detects and tokenises Singapore personal data on the way in, puts it back on the way out, enforces per-tenant policy, and keeps a tamper-evident audit log. Any client uses it by changing `base_url`.

**Status: in development** for v0.1 (Milestone 2, due 22 Nov 2026). Built so far: the command-line skeleton, a tamper-evident audit log, placeholders with an encrypted in-memory vault, and the detection engine, which passes all 369 shared test cases for Singapore personal data (NRIC/FIN, phone, email, card, postal code, unit number, date of birth). The gateway itself comes next; each command says which issue builds it. Every change is reviewed by [Vetted](https://github.com/makoydev/vetted): in shadow mode until 14 Oct 2026, then opt-in.

## Build and check

Needs Go 1.27.

```sh
make check            # gofmt, go vet, go test -race, govulncheck, build
./bin/discreet help
./bin/discreet audit verify -log discreet-audit.jsonl     # check the hash chain
./bin/discreet audit export -csv -log discreet-audit.jsonl > audit.csv
```

## Evidence

[`CONTROLS.md`](CONTROLS.md) · [`THREAT_MODEL.md`](THREAT_MODEL.md) · [`EVALS.md`](EVALS.md) · [`RISKS.md`](RISKS.md) · [`CHANGELOG.md`](CHANGELOG.md) · [decision records](docs/adr/) · [how this was built](docs/HOW-THIS-WAS-BUILT.md)

Part of the AI Governance Toolkit. Programme board: <https://github.com/users/makoydev/projects/1>
