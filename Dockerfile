# syntax=docker/dockerfile:1
#
# Builds the Glade playground image for local container validation.
#
# The Apex parser (github.com/glade-sh/apex-parser) is vendored into this repo at
# third_party/glade-apex-parser and wired in via a `replace` directive in go.mod,
# so the image builds straight from the repo with no extra build context.
#
# CGO is REQUIRED: the declaration parser uses the generated tree-sitter Apex
# parser (C), so class/trigger parsing only works with CGO enabled. A CGO_ENABLED=0
# build returns an APEXPARSECGO diagnostic instead of parsing. The binary therefore
# links against glibc and runs on a glibc base image.
#
# Build from the repo root with:
#
#   docker build -t glade-playground .

FROM golang:1.26 AS build
WORKDIR /src

# Download public module dependencies. The replaced parser module is local
# (third_party/glade-apex-parser), so copy it before `go mod download` so the
# filesystem replace target resolves.
COPY go.mod go.sum ./
COPY third_party/glade-apex-parser ./third_party/glade-apex-parser
RUN go mod download

# Build with CGO enabled (the golang image ships a C toolchain).
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -o /out/glade ./cmd/glade

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r glade && useradd -r -g glade glade
COPY --from=build /out/glade /usr/local/bin/glade
USER glade
ENV PORT=8080
EXPOSE 8080
# Hardened public mode: per-run timeout, forced scratch + strict limits,
# per-IP rate limiting, ephemeral org. See docs/PLAYGROUND_HOSTING.md.
ENTRYPOINT ["sh", "-c", "exec /usr/local/bin/glade playground --examples --public --addr 0.0.0.0:${PORT:-8080}"]
