#!/usr/bin/env bash
set -euo pipefail

if ! command -v sf >/dev/null 2>&1; then
  echo "smoke_glade_org_sf: skipping; sf is not installed"
  exit 0
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" >/dev/null 2>&1; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${TMP}"
}
trap cleanup EXIT

cd "${ROOT}"

export SF_CONFIG_DIR="${TMP}/sf"
ALIAS_NAME="my-glade-org"
DB="${TMP}/${ALIAS_NAME}.sqlite"
ADDR="127.0.0.1:17911"
PROJECT="${TMP}/project"

mkdir -p "${PROJECT}/force-app"
cat >"${PROJECT}/sfdx-project.json" <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}
JSON

go run ./cmd/glade org create "${ALIAS_NAME}" --project "${PROJECT}" --db "${DB}" --addr "${ADDR}"
go run ./cmd/glade org start "${ALIAS_NAME}" --project "${PROJECT}" >"${TMP}/server.log" 2>&1 &
SERVER_PID="$!"

server_ready=0
for _ in $(seq 1 100); do
  if curl -fsS "http://${ADDR}/services/oauth2/userinfo" >/dev/null 2>&1; then
    server_ready=1
    break
  fi
  sleep 0.1
done

if [[ "${server_ready}" != "1" ]]; then
  tail -c 4000 "${TMP}/server.log" >&2 || true
  exit 1
fi

go run ./cmd/glade org auth "${ALIAS_NAME}" --project "${PROJECT}" --sf-config-dir "${SF_CONFIG_DIR}"

sf data create record -o "${ALIAS_NAME}" -s Account -v "Name='SF Smoke'"
sf data query -o "${ALIAS_NAME}" -q "SELECT Id, Name FROM Account WHERE Name = 'SF Smoke'" --json

printf "insert new Account(Name = 'Apex Smoke');\n" >"${TMP}/smoke.apex"
sf apex run -o "${ALIAS_NAME}" -f "${TMP}/smoke.apex"

go run ./cmd/glade db query --db "${DB}" --project "${PROJECT}" --json "SELECT Id, Name FROM Account WHERE Name IN ('SF Smoke','Apex Smoke')"

echo "smoke_glade_org_sf: ok"
