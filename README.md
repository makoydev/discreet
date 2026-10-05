# Discreet

A privacy-preserving gateway for large language models: an OpenAI-compatible proxy that detects and tokenises Singapore personal data on the way in, puts it back on the way out, enforces per-tenant policy, and keeps a tamper-evident audit log. Any client uses it by changing `base_url`.

**Status: in development** for v0.1 (Milestone 2, due 22 Nov 2026). Built so far: the gateway (`discreet serve`, mock model), a tamper-evident audit log, placeholders with an encrypted in-memory vault, and the detection engine, which passes all 369 shared test cases for Singapore personal data (NRIC/FIN, phone, email, card, postal code, unit number, date of birth). The gateway itself comes next; each command says which issue builds it. Every change is reviewed by [Vetted](https://github.com/makoydev/vetted): in shadow mode until 14 Oct 2026, then opt-in.

## Build and check

Needs Go 1.27.

```sh
make check            # gofmt, go vet, go test -race, govulncheck, build
./bin/discreet help
./bin/discreet audit verify -log discreet-audit.jsonl     # check the hash chain
./bin/discreet audit export -csv -log discreet-audit.jsonl > audit.csv
```

## Run it

```sh
export DISCREET_HMAC_KEY=$(openssl rand -hex 32)   # keys the audit log's hashes
./bin/discreet serve                                # listens on :8080, free mock model

curl -s localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' -H 'X-Discreet-Purpose: claims summary' \
  -d '{"model":"gpt-6-luna","messages":[{"role":"user","content":"Call S1234567D on 9123 4567"}]}'
```

To use a real model, point Discreet at it and give callers tokens; anyone without a valid token still gets the mock:

```sh
export DISCREET_UPSTREAM_URL=https://api.openai.com/v1
export DISCREET_UPSTREAM_API_KEY=...            # your OpenAI key, set it yourself
export DISCREET_ACCESS_TOKENS=claims-team:$(openssl rand -hex 16)
./bin/discreet serve                             # gpt-6-luna, US$0.20/day hard cap
```

Run `./bin/discreet serve --help` for every setting (budget, output cap, rate limit, Ollama).

The mock model shows what a real model would have received (placeholders only), and the answer comes back with the real values restored. Any OpenAI client works by pointing `base_url` at `http://localhost:8080/v1` and adding the `X-Discreet-Purpose` header. The example NRIC is synthetic.

## Evidence

[`CONTROLS.md`](CONTROLS.md) · [`THREAT_MODEL.md`](THREAT_MODEL.md) · [`EVALS.md`](EVALS.md) · [`RISKS.md`](RISKS.md) · [`CHANGELOG.md`](CHANGELOG.md) · [decision records](docs/adr/) · [how this was built](docs/HOW-THIS-WAS-BUILT.md)

Part of the AI Governance Toolkit. Programme board: <https://github.com/users/makoydev/projects/1>
