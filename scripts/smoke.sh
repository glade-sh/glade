#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  rm -rf "${TMP}"
}
trap cleanup EXIT

cd "${ROOT}"

go build -o "${TMP}/glade" ./cmd/glade
GLADE="${TMP}/glade"

"${GLADE}" version >/dev/null

DIST_DIR="${TMP}/release-dist" VERSION=smoke scripts/release-build.sh >"${TMP}/release-build.out"
grep -q 'release artifact written' "${TMP}/release-build.out"
grep -q 'release manifest written' "${TMP}/release-build.out"
grep -q 'glade_smoke_' "${TMP}/release-build.out"
test -f "${TMP}/release-dist/release-manifest.json"
test -f "${TMP}/release-dist/index.json"
test -f "${TMP}/release-dist/latest/release-manifest.json"
grep -q '"schemaVersion": 2' "${TMP}/release-dist/release-manifest.json"
grep -q '"assets"' "${TMP}/release-dist/release-manifest.json"
grep -q '"sha256"' "${TMP}/release-dist/release-manifest.json"
grep -q '"parserSmoke": "passed"' "${TMP}/release-dist/release-manifest.json"

PROJECT="${TMP}/project"
mkdir -p "${PROJECT}/force-app/main/classes"
cat >"${PROJECT}/sfdx-project.json" <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}
JSON
cat >"${PROJECT}/force-app/main/classes/Sample.cls" <<'APEX'
public class Sample {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
APEX
cat >"${PROJECT}/force-app/main/classes/SampleTest.cls" <<'APEX'
@isTest
private class SampleTest {
  @isTest static void adds() {
    System.assertEquals(3, Sample.add(1, 2));
  }
}
APEX

"${GLADE}" parse "${PROJECT}/force-app/main/classes/Sample.cls" --json >"${TMP}/parse.json"
grep -q '"name": "Sample"' "${TMP}/parse.json"

"${GLADE}" check --project "${PROJECT}" --json >"${TMP}/check.json"
grep -q '"diagnostics": 0' "${TMP}/check.json"

"${GLADE}" exec --trace "${TMP}/trace.json" "Integer x = 1 + 1; System.assertEquals(2, x); System.debug('x=' + x);" >"${TMP}/exec.out"
grep -q 'x=2' "${TMP}/exec.out"

"${GLADE}" profile analyze "${TMP}/trace.json" --json >"${TMP}/profile.json"
grep -q '"events"' "${TMP}/profile.json"

"${GLADE}" test --project "${PROJECT}" --json >"${TMP}/test.json"
grep -q '"passed": 1' "${TMP}/test.json"

DB="${TMP}/glade.db"
cat >"${TMP}/fixture.json" <<'JSON'
{
  "version": "glade.storage.v1",
  "objects": [
    {
      "name": "Account",
      "records": [
        {"alias": "acme", "fields": {"Name": {"kind": "string", "string": "Acme"}}}
      ]
    }
  ]
}
JSON
"${GLADE}" db seed --project "${PROJECT}" --db "${DB}" "${TMP}/fixture.json" --json >"${TMP}/db-seed.json"
grep -q '"Account": 1' "${TMP}/db-seed.json"
"${GLADE}" db inspect --project "${PROJECT}" --db "${DB}" >"${TMP}/db-inspect.out"
grep -q 'Account: 1' "${TMP}/db-inspect.out"

"${GLADE}" playground --data-root "${TMP}/playground" --db "${TMP}/playground.sqlite" --once >"${TMP}/playground.out"
grep -q 'http://127.0.0.1:1789/playground/' "${TMP}/playground.out"

LSP_PROJECT="${TMP}/lsp-project"
mkdir -p "${LSP_PROJECT}/force-app/main/classes"
cat >"${LSP_PROJECT}/sfdx-project.json" <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}
JSON
cat >"${LSP_PROJECT}/force-app/main/classes/Broken.cls" <<'APEX'
public class Broken {
  public MissingType run() {
    return null;
  }
}
APEX
"${GLADE}" lsp --project "${LSP_PROJECT}" --diagnostics-once >"${TMP}/lsp.out"
grep -q 'textDocument/publishDiagnostics' "${TMP}/lsp.out"

ADDR="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
host, port = s.getsockname()
s.close()
print(f"{host}:{port}")
PY
)"
"${GLADE}" server --addr "${ADDR}" --db "${DB}" --project "${PROJECT}" >"${TMP}/server.log" 2>&1 &
SERVER_PID="$!"
server_ready=0
for _ in $(seq 1 300); do
  if curl -fsS "http://${ADDR}/services/data" >"${TMP}/server-data.json" 2>/dev/null; then
    server_ready=1
    break
  fi
  sleep 0.1
done
if [[ "${server_ready}" != "1" ]]; then
  tail -c 4000 "${TMP}/server.log" >&2
  exit 1
fi
grep -q 'v65.0' "${TMP}/server-data.json"
kill "${SERVER_PID}" 2>/dev/null || true
wait "${SERVER_PID}" 2>/dev/null || true
SERVER_PID=""

echo "smoke: ok"
