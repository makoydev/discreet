# Deploying the public demo (Fly.io)

The public demo runs **mock-only**: `fly.toml` never sets `DISCREET_UPSTREAM_URL`, so nothing anyone sends can reach a paid model (ADR 0007, ADR 0009). It sleeps when idle, so it costs well under US$1 a month (one 256 MB shared machine plus a 1 GB volume).

Deployment happens from `main` only, by `.github/workflows/deploy.yml`, after Michael merges `next` → `main`. Until the `FLY_API_TOKEN` secret exists, the workflow skips itself.

## One-time setup (Michael, about 10 minutes)

These steps need your account and card, so only you can do them. Run them from the `discreet` folder.

1. Install the Fly.io command-line tool and sign in. Sign-up asks for a card.
   ```sh
   brew install flyctl
   fly auth signup        # or: fly auth login
   ```
2. In the Fly.io dashboard (Billing), turn on a usage alert so you're emailed before any surprise.
3. Create the app. If the name `discreet-demo` is taken, choose another and change `app` in `fly.toml` to match.
   ```sh
   fly apps create discreet-demo
   ```
4. Create the 1 GB volume for the audit log, in Singapore:
   ```sh
   fly volumes create discreet_data --region sin --size 1 -a discreet-demo
   ```
5. Set the audit log's HMAC key. It's generated and sent straight to Fly; it never appears on screen or in the repository.
   ```sh
   fly secrets set DISCREET_HMAC_KEY="$(openssl rand -hex 32)" -a discreet-demo
   ```
6. Create a deploy token for this app only, and store it as a GitHub secret. Paste the token when `gh` asks for it.
   ```sh
   fly tokens create deploy -a discreet-demo --expiry 2160h   # 90 days
   gh secret set FLY_API_TOKEN -R makoydev/discreet
   ```
7. Deploy: either merge `next` → `main`, or run the workflow by hand:
   ```sh
   gh workflow run deploy.yml -R makoydev/discreet
   ```
   The workflow finishes with a smoke test against `https://discreet-demo.fly.dev`: the page answers, the model saw only placeholders, and the model was the mock.

## Never do this on the public app

- Don't set `DISCREET_UPSTREAM_URL` or `DISCREET_UPSTREAM_API_KEY` with `fly secrets set`. The public demo must never reach a paid model.
- Don't share `DISCREET_HMAC_KEY`. Without it, nobody can test whether a guessed NRIC matches a fingerprint in the audit log.

## Checking the audit log on the server

```sh
fly ssh console -a discreet-demo -C "/discreet audit verify -log /data/discreet-audit.jsonl"
```
