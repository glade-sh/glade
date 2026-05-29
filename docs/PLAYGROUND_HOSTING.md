# Hosting the public Glade Playground

`glade playground --public` is the mode intended for `play.glade.sh`. It keeps the local developer default unchanged, but adds guard rails for a server that accepts arbitrary Apex.

## Local container run

```bash
docker build -t glade-playground .
docker run --rm -p 8080:8080 -e PORT=8080 glade-playground
```

Open <http://localhost:8080/playground/>.

The image builds only the Go binary. The playground UI assets are embedded in `glade`, so no separate web build is needed.

## Hardened flags

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

`.do/app.yaml` defines one Dockerfile-backed web service:

```bash
doctl apps create --spec .do/app.yaml
```

For an existing app:

```bash
doctl apps update <app-id> --spec .do/app.yaml
```

Set the repository and branch in the App Platform UI, or let DigitalOcean auto-detect the Dockerfile from this repo. The service listens on port `8080` and health-checks `GET /playground/`.

## DNS for play.glade.sh

In App Platform, add `play.glade.sh` as the primary domain. Then create the DNS record DigitalOcean shows, usually a CNAME from `play` to the App Platform target. Wait for certificate provisioning before sending public traffic.

## Security model and scaling

The playground executes untrusted Apex. Public mode limits CPU-like VM steps, wall-clock runtime, rate, persistent org mutation, seed mutation, and workspace growth. It is not a security sandbox for the Go process. Run it in a container with platform CPU and memory limits, expose it only through HTTPS, and prefer horizontal instances over large single instances. In-process rate limits are per instance; add edge rate limiting if multiple instances serve the same domain.
