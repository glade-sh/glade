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
docker run --rm -p 8080:8080 -e PORT=8080 glade-playground
```

Or use the helper:

```bash
scripts/build-playground-image.sh
```

Open <http://localhost:8080/playground/>.

The image builds only the Go binary. The playground UI assets are embedded in
`glade`, so no separate web build is needed. The build uses `CGO_ENABLED=1`
because the Apex declaration parser is a tree-sitter (C) parser; the binary links
against glibc and runs on a glibc base image.

## Public-mode flags

```bash
glade playground --examples --public --addr 0.0.0.0:${PORT:-8080}
```

Public mode enforces:

- a per-run context deadline, default `5s` (`--run-timeout 3s` overrides it)
- strict VM governor mode with lower public caps
- scratch run mode, even if the client asks for `persist`
- no run-result cache writes for public runs
- in-process fixed-window rate limiting, default `30` mutating requests/minute/IP (`--rate-per-minute 60` overrides it)
- first-hop `X-Forwarded-For` client IP handling for proxy deployments
- disabled `/playground/api/seed`
- workspace file and total-size caps for public saves

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

The service listens on port `8080` and health-checks `GET /playground/`.

## Security model and scaling

The playground executes untrusted Apex. Public mode limits CPU-like VM steps,
wall-clock runtime, rate, persistent org mutation, seed mutation, and workspace
growth. It is not a security sandbox for the Go process. Any future public
deployment must run in a container with platform CPU and memory limits, expose
only HTTPS, and prefer horizontal instances over large single instances. In-process
rate limits are per instance; add edge rate limiting before shared public traffic.
