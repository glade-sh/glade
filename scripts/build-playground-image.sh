#!/usr/bin/env bash
#
# Build (and optionally push) the Glade playground container image.
#
# The Apex parser is vendored into the repo (third_party/glade-apex-parser), so
# the image builds straight from the repo root with no extra build context.
# DigitalOcean App Platform can build from source directly (see .do/app.yaml);
# this script is for building/pushing an image manually (e.g. to a registry).
#
# Usage:
#   scripts/build-playground-image.sh                 # build local tag glade-playground:latest
#   PUSH=1 REGISTRY=registry.digitalocean.com/<reg> scripts/build-playground-image.sh
#
# Env:
#   IMAGE      image name (default: glade-playground)
#   TAG        image tag (default: latest)
#   REGISTRY   registry prefix, e.g. registry.digitalocean.com/<your-registry>
#   PUSH       when set to 1, docker push after build (requires REGISTRY + login)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-glade-playground}"
TAG="${TAG:-latest}"
REGISTRY="${REGISTRY:-}"

REF="${IMAGE}:${TAG}"
if [ -n "${REGISTRY}" ]; then
  REF="${REGISTRY%/}/${IMAGE}:${TAG}"
fi

echo "building ${REF}"
docker build -t "${REF}" "${ROOT}"

if [ "${PUSH:-}" = "1" ]; then
  if [ -z "${REGISTRY}" ]; then
    echo "error: PUSH=1 requires REGISTRY" >&2
    exit 1
  fi
  echo "pushing ${REF}"
  docker push "${REF}"
fi

echo "done: ${REF}"
