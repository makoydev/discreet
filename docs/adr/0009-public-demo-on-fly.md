# 0009. The public demo runs mock-only on one small Fly.io machine

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-05

## Context

The Milestone 2 plan (approved 2026-10-02) chose Fly.io: pay-as-you-go, with machines that stop when idle (about US$0.50 a month for the smallest, per Fly's pricing page). The brief requires a public deployment in mock mode, and public endpoints never relay to a paid model.

## Decision

- **Image:** a static Go binary (`CGO_ENABLED=0`) on Google's distroless `static-debian13:nonroot`, no shell or package manager, running as a non-root user. Both base images are pinned by digest. A CI job builds the image on every pull request and smoke-tests it the way Fly runs it: non-root, audit log on a volume at `/data`, `audit verify` passing, no planted value in the logs.
- **Fly:** region `sin` (Singapore), one shared-CPU 256 MB machine, `auto_stop_machines = "stop"` and `min_machines_running = 0`, a 1 GB volume for the audit log, a health check on `/healthz`, and the client address taken from `Fly-Client-IP` for rate limiting.
- **Mock only:** `fly.toml` never sets `DISCREET_UPSTREAM_URL`; `docs/DEPLOY.md` says never to set it as a secret on the public app.
- **Deploys** run from `main` only, through a pinned workflow that skips itself without `FLY_API_TOKEN` and ends with a smoke test against the live URL.

## Consequences

- The first request after idling waits a few seconds while the machine starts.
- Fly gives the volume to the image's user, according to Fly staff on the community forum (secondary source); the first deployment's smoke test confirms it, because the demo can't answer without writing its audit record.
- Fly can only be set up by Michael (account, card, token); until then the demo runs locally or in Docker.
