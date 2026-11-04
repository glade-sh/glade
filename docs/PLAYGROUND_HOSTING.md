# Playground Container Runbook

Public playground publishing is paused. Do not attach a public domain or enable automatic deploys until the public isolation plan is implemented and verified.

Use this runbook only to build and validate the playground container.

## Local container run

The Apex parser (`github.com/glade-sh/apex-parser`) is vendored into this repo at
`third_party/glade-apex-parser` and wired in through a `replace` directive in
`go.mod`, so the image builds straight from the repo with no extra build context:

```bash
# from the glade repo root
docker build -t glade-playground .
docker run --rm -p 127.0.0.1:8080:8080 -e PORT=8080 \
  -e GLADE_SERVER_PUBLIC=1 -e GLADE_PLAYGROUND_INSECURE_LOCAL=1 glade-playground
```

Or use the helper:

```bash
scripts/build-playground-image.sh
```

Open <http://localhost:8080/playground/>. The explicit local cookie option
allows browsers to retain the session over loopback HTTP. Publish the container
port only to loopback, as shown: this option is for local validation, not hosted
deployments. It is ignored for configured trusted proxies and App Platform;
HTTPS and non-loopback URL hosts still receive Secure cookies.

The image builds only the Go binary. The playground UI assets are embedded in
`glade`, so no separate web build is needed. The build uses `CGO_ENABLED=1`
because the Apex declaration parser is a tree-sitter (C) parser; the binary links
against glibc and runs on a glibc base image. The purpose-built image and App
Platform spec set the required `GLADE_SERVER_PUBLIC=1` network-bind opt-in.

## Public-mode flags

```bash
GLADE_SERVER_PUBLIC=1 glade playground --examples --public --addr 0.0.0.0:${PORT:-8080}
```

Public mode enforces:

- a per-run context deadline, default `5s` (`--run-timeout 3s` overrides it)
- strict VM governor mode with lower public caps
- scratch run mode, even if the client asks for `persist`
- no run-result cache writes for public runs
- in-process fixed-window rate limiting for mutating requests and database reads, default `30` requests/minute/IP (`--rate-per-minute 60` overrides it)
- at most eight live sessions per client and 64 total sessions; unexpired workspaces are not evicted for new visitors
- rate limiting and session quotas by the direct peer address by default; forwarded headers are not trusted
- separate ephemeral workspaces and in-memory org state per session
- Secure, HttpOnly, SameSite=Lax session cookies with cross-origin mutation checks; the explicit loopback HTTP option above is the only exception to Secure
- bounded concurrent requests, request-body deadlines, public run output and database snapshots (the public snapshot omits empty standard objects and is capped at 512 KiB)
- disabled `/playground/api/seed`
- workspace file and total-size caps for public saves

For a reverse proxy, add `--trusted-proxy <proxy-IP>` only when that proxy is
controlled by the operator and overwrites `X-Forwarded-For` with exactly one
client IP address. The server accepts that header only from the configured
peer; absent, malformed, or multi-address values fall back to the peer address.
The proxy must also overwrite `X-Forwarded-Proto` with exactly `http` or
`https`. Origin-bearing mutations from that proxy fail closed if the header is
missing or ambiguous.
A proxy that appends client-supplied headers must be reconfigured to overwrite
them before enabling this option. Without it, all clients behind the same
proxy share one quota.

The VM checks request context during execution and also uses the strict CPU cap. A tight Apex loop should stop at the deadline or public CPU cap. Code outside the VM execution loop, such as parsing and compilation, observes less frequent cancellation; keep the service behind container CPU and memory limits.

## DigitalOcean App Platform

Because the parser is vendored into the repo, App Platform can build the image
directly from the connected GitHub source. `.do/app.yaml` defines one web service
that builds from the repo `Dockerfile`. The spec does not configure a public
domain and has automatic deploys disabled.

```bash
doctl apps create --spec .do/app.yaml      # first deploy
doctl apps update <app-id> --spec .do/app.yaml   # subsequent deploys
```

Update the `github.repo`/`branch` fields in `.do/app.yaml` only for a temporary
validation app. Keep `deploy_on_push: false` while public publishing is paused.

If you prefer a prebuilt image instead of a source build, push one to a registry
(`PUSH=1 REGISTRY=registry.digitalocean.com/<your-registry> scripts/build-playground-image.sh`)
and point the service at it with an `image:` block in `.do/app.yaml`.

The service listens on port `8080`. App Platform health-checks `GET /healthz`, a
process-liveness probe that stays healthy when readiness fails. `GET /readyz`
verifies that the runner initialized and the workspace is readable. The
App Platform spec enables `GLADE_DO_APP_PLATFORM=1`, which uses the
[platform-provided `DO-Connecting-IP` client address](https://docs.digitalocean.com/support/where-can-i-find-the-client-ip-address-of-a-request-connecting-to-my-app/)
for session and rate quotas. Enable this option only behind the App Platform
ingress: the application must not be reachable by untrusted direct peers or
components able to forge this header. Missing, invalid, or multiple header
values fall back to the direct peer. The option is off outside that spec. App
Platform upgrades external HTTP requests to HTTPS, so origin checks use HTTPS
for this explicitly enabled ingress.

## Security model and scaling

The playground executes untrusted Apex. Public mode limits CPU-like VM steps,
wall-clock runtime, rate, persistent org mutation, seed mutation, and workspace
growth. It is not a security sandbox for the Go process. Any future public
deployment must run in a container with platform CPU and memory limits, expose
only HTTPS, and prefer horizontal instances over large single instances. In-process
rate limits are per instance and keyed by the direct peer. Behind a proxy, clients
share the proxy address; configure edge rate limiting before shared public traffic.

The built-in server currently has one workspace and runner per process. It does
not isolate source or file mutations between visitors. Do not expose it to
untrusted multi-user traffic until request or session isolation is added; public
mode safeguards do not provide visitor isolation.
