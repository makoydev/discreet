# 0008. The demo page runs the API's own pipeline, always on the mock model

Status: drafted by Claude Code, awaiting Michael's review
Date: 2026-10-05

## Context

The brief asks for a demo page with four panes (original prompt, what the model saw, the model's answer, the rehydrated answer) plus the call's audit record. A demo that re-implements the gateway's steps can drift from what the API actually does, and then it demonstrates something that isn't true.

## Options

1. **One pipeline function** (`process`) used by both `/v1/chat/completions` and `/demo/run`, returning every stage; the demo endpoint just shows them.
2. **A separate demo handler** with its own simplified flow. Less refactoring, but two copies of the privacy logic.
3. **A static page calling the public API** and reconstructing the stages in the browser. The API (correctly) doesn't return what the model saw, so the browser couldn't show pane 2 honestly.

## Decision

Option 1. The demo always uses the mock model, even when the caller sends a valid access token, and records requests under the tenant `demo`. The page is plain HTML, CSS and JavaScript embedded in the binary, served with a strict Content-Security-Policy: its own scripts only, connections only back to the server, user text inserted as text, never HTML.

## Consequences

- What the demo shows is what the API does; the refactor kept every existing gateway test passing unchanged.
- Demo requests appear in the audit log like any other, which is the point: visitors can watch the record being created.
- The page needs no build step and no JavaScript framework (ADR 0001).
