# Public Playground 1.0 Plan

Date: 2026-05-23  
Project: oaer  
Objective: ship a public Apex playground with fast, safe feedback for parse/check/simple exec.

## Goal

Deliver a public URL where users can paste Apex and get clear results from:

1. `parse`
2. `check`
3. basic `exec`

This is a narrow first release focused on reliability, safety, and quick iteration.

## 1.0 Scope

1. Single-page playground UI.
2. Single source editor.
3. One `Run` action.
4. Mode toggle: `parse`, `check`, `exec`.
5. Clear diagnostics with line/column.
6. Stable response shape with `traceId`.
7. Public deployment on DigitalOcean with custom domain and TLS.
8. Basic abuse controls and runtime limits.

## Out Of Scope (1.0)

1. Full Salesforce parity claims.
2. Persistent user accounts.
3. Multi-file or package project support.
4. Team collaboration or sharing workspace state.
5. Long-running program execution.

## Architecture

## Frontend

1. Static web app (React + TypeScript or equivalent).
2. Layout:
   - left pane: Apex editor
   - right pane: result pane
   - top controls: mode toggle + run button
3. Response rendering:
   - pass/fail status
   - first error promoted at top
   - expandable raw details section

## Backend API

1. Endpoint: `POST /api/run`
2. Handles mode dispatch (`parse`, `check`, `exec`)
3. Returns normalized JSON contract
4. Emits request/run `traceId` for support and debugging

## Runner

1. Isolated worker process around `oaer`.
2. Hard timeout and memory caps.
3. Rejects oversized requests.
4. No direct network/file capabilities for user-provided code path.

## Edge and Platform

1. DigitalOcean App Platform (fastest 1.0 path) or Droplet + reverse proxy.
2. Domain: `playground.<yourdomain>`
3. TLS: managed certificates.
4. Rate limiting at edge and API layer.

## API Contract (1.0)

## Request

```json
{
  "mode": "parse|check|exec",
  "source": "public class A { ... }",
  "options": {
    "limitMode": "strict",
    "timeoutMs": 3000
  }
}
```

## Response

```json
{
  "status": "pass|fail|error",
  "diagnostics": [
    {
      "severity": "error|warning",
      "message": "Unexpected token",
      "line": 4,
      "column": 12
    }
  ],
  "output": {
    "result": null,
    "logs": []
  },
  "traceId": "run_20260523_ab12",
  "durationMs": 184
}
```

## Limits And Safety Rails

1. Maximum source size: 100 KB.
2. Default timeout: 3000 ms.
3. Hard timeout cap: 5000 ms.
4. Per-run memory limit enforced by worker/container.
5. Request rate limit per IP.
6. Concurrency cap per instance.
7. Panic-safe handler paths with stable error envelope.
8. Redaction for sensitive internals in public error responses.

## UX Behavior

1. Keep output terse by default.
2. Show first actionable error with location.
3. Show duration in ms.
4. Keep raw logs hidden behind `Details`.
5. Do not block editor while run is pending; show spinner and cancel button (if possible in 1.0 timeline).

## Milestones

## M1: Local Vertical Slice

1. Implement `POST /api/run`.
2. Wire to `oaer` parse/check/exec flow.
3. Build minimal UI with mode toggle and output panel.
4. Verify common snippets return in <2s locally.

Exit criteria:

1. End-to-end local flow works for all 3 modes.
2. Stable JSON responses for pass/fail/error.

## M2: Hardening

1. Add input size limits.
2. Add timeout and concurrency guards.
3. Add rate limiting.
4. Add panic recovery and trace IDs.
5. Add structured logging.

Exit criteria:

1. Malformed input never crashes service.
2. Abusive load is throttled.

## M3: Staging Deployment (DigitalOcean)

1. Containerize backend + static frontend.
2. Deploy staging app.
3. Attach staging domain + TLS.
4. Run smoke checks and latency checks.

Exit criteria:

1. Staging URL stable for 24h.
2. p95 latency within target for common snippets.

## M4: Public 1.0 Launch

1. Promote to production app.
2. Configure production domain + TLS.
3. Enable minimal observability dashboards/alerts.
4. Publish basic usage notes.

Exit criteria:

1. Public endpoint healthy.
2. Error rate and latency within targets.

## M5: Week-1 Stabilization

1. Triage top public failure signatures.
2. Patch high-frequency diagnostics/UX gaps.
3. Tighten limits based on observed traffic.

Exit criteria:

1. No recurring severity-1 issues.
2. Top user confusion patterns addressed.

## Acceptance Criteria

1. Typical response latency <2s for parse/check.
2. No server panic from malformed user input.
3. Clear line/column diagnostics when available.
4. Stable `traceId` on every response.
5. Rate limiting effective under burst traffic.
6. Overnight uptime and recovery behavior proven.

## DigitalOcean Deployment Checklist

1. Provision container registry.
2. Build and push image.
3. Create app spec (`.do/app-playground.yaml`).
4. Deploy staging app.
5. Configure DNS and TLS.
6. Promote to production.

Suggested commands:

```bash
docker build -t registry.digitalocean.com/<registry>/oaer-playground:1.0.0 .
docker push registry.digitalocean.com/<registry>/oaer-playground:1.0.0
doctl apps create --spec .do/app-playground.yaml
```

Smoke test:

```bash
curl -sS https://playground.<yourdomain>/api/run \
  -H 'content-type: application/json' \
  -d '{"mode":"check","source":"public class A {}","options":{"timeoutMs":3000}}'
```

## First Build Order

1. Backend run endpoint and response normalization.
2. Worker isolation + timeout + memory caps.
3. Minimal frontend editor and result pane.
4. Rate limiting + structured logging.
5. DO app spec and staging deploy.

## Risks And Mitigations

1. Risk: `exec` can consume resources.
   - Mitigation: strict timeout/memory, concurrency ceiling, mode-level kill switch.
2. Risk: public abuse traffic.
   - Mitigation: edge + app rate limits, payload caps, temporary IP blocks.
3. Risk: unclear diagnostics reduce usefulness.
   - Mitigation: first-error summarization with location and short hint.
4. Risk: over-claiming parity.
   - Mitigation: explicit UI copy: "preview feedback powered by oaer local runtime."

## Operational Notes

1. Keep 1.0 logs and metrics simple: request count, latency, failures by mode.
2. Keep retention short while volume is unknown.
3. Ensure error pages return valid JSON on API paths.
4. Track rollout date and known limitations in release notes.
