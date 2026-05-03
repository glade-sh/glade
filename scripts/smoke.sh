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

go build -o "${TMP}/oaer" ./cmd/oaer
OAER="${TMP}/oaer"

"${OAER}" version >/dev/null

PROJECT="${TMP}/project"
mkdir -p "${PROJECT}/force-app/main/classes"
cat >"${PROJECT}/sfdx-project.json" <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}
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

"${OAER}" parse "${PROJECT}/force-app/main/classes/Sample.cls" --json >"${TMP}/parse.json"
grep -q '"name": "Sample"' "${TMP}/parse.json"

"${OAER}" check --project "${PROJECT}" --json >"${TMP}/check.json"
grep -q '"diagnostics": 0' "${TMP}/check.json"

"${OAER}" exec --trace "${TMP}/trace.json" "Integer x = 1 + 1; System.assertEquals(2, x); System.debug('x=' + x);" >"${TMP}/exec.out"
grep -q 'x=2' "${TMP}/exec.out"

"${OAER}" profile analyze "${TMP}/trace.json" --json >"${TMP}/profile.json"
grep -q '"events"' "${TMP}/profile.json"

"${OAER}" test --project "${PROJECT}" --json >"${TMP}/test.json"
grep -q '"passed": 1' "${TMP}/test.json"

DB="${TMP}/oaer.db"
cat >"${TMP}/fixture.json" <<'JSON'
{
  "version": "oaer.storage.v1",
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
"${OAER}" db seed --db "${DB}" "${TMP}/fixture.json" --json >"${TMP}/db-seed.json"
grep -q '"Account": 1' "${TMP}/db-seed.json"
"${OAER}" db inspect --db "${DB}" >"${TMP}/db-inspect.out"
grep -q 'Account: 1' "${TMP}/db-inspect.out"

LSP_PROJECT="${TMP}/lsp-project"
mkdir -p "${LSP_PROJECT}/force-app/main/classes"
cat >"${LSP_PROJECT}/sfdx-project.json" <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}
JSON
cat >"${LSP_PROJECT}/force-app/main/classes/Broken.cls" <<'APEX'
public class Broken {
  public MissingType run() {
    return null;
  }
}
APEX
"${OAER}" lsp --project "${LSP_PROJECT}" --diagnostics-once >"${TMP}/lsp.out"
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
"${OAER}" server --addr "${ADDR}" --db "${DB}" --project "${PROJECT}" >"${TMP}/server.log" 2>&1 &
SERVER_PID="$!"
for _ in $(seq 1 50); do
  if curl -fsS "http://${ADDR}/services/data" >"${TMP}/server-data.json" 2>/dev/null; then
    break
  fi
  sleep 0.1
done
grep -q 'v61.0' "${TMP}/server-data.json"

"${OAER}" compat mvp >"${TMP}/compat-mvp.out"
grep -q 'MVP readiness: not ready' "${TMP}/compat-mvp.out"
"${OAER}" compat matrix --json >"${TMP}/compat-matrix.json"
grep -q '"ready": false' "${TMP}/compat-matrix.json"
"${OAER}" compat validate docs/fixtures/*.json
"${OAER}" compat run docs/fixtures/*.json
"${OAER}" compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
"${OAER}" compat gaps --check docs/KNOWN_GAPS.md
"${OAER}" compat stdlib --check docs/STDLIB_COVERAGE.md
OAER_BIN="${OAER}" scripts/apex-docs-support-gate.sh

echo "smoke: ok"
