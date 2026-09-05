#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
  echo "usage: scripts/smoke-runtime.sh <glade-executable>" >&2
  exit 2
fi

GLADE="$1"
if [[ "${GLADE}" != /* ]]; then
  GLADE="${PWD}/${GLADE}"
fi
if [[ ! -e "${GLADE}" ]]; then
  echo "glade executable does not exist: ${GLADE}" >&2
  exit 2
fi
if [[ ! -f "${GLADE}" || ! -x "${GLADE}" ]]; then
  echo "glade executable is not executable: ${GLADE}" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]]; then
    if kill -0 "${SERVER_PID}" 2>/dev/null; then
      kill "${SERVER_PID}" 2>/dev/null || true
    fi
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  rm -rf "${TMP}"
}
trap cleanup EXIT

cd "${ROOT}"

"${GLADE}" version >/dev/null

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

DB_UI_READY="${TMP}/db-ui-ready.json"
"${GLADE}" db ui --project "${PROJECT}" --db "${DB}" --addr 127.0.0.1:0 --no-open --ready-file "${DB_UI_READY}" >"${TMP}/db-ui.log" 2>&1 &
SERVER_PID="$!"
db_ui_ready=0
for _ in $(seq 1 300); do
  if [[ -s "${DB_UI_READY}" ]]; then
    db_ui_ready=1
    break
  fi
  sleep 0.1
done
if [[ "${db_ui_ready}" != "1" ]]; then
  tail -c 4000 "${TMP}/db-ui.log" >&2
  exit 1
fi
DB_UI_URL="$(python3 - "${DB_UI_READY}" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f)["url"])
PY
)"
curl -fsS "${DB_UI_URL}" >"${TMP}/db-ui.html"
grep -q 'Glade Local Data' "${TMP}/db-ui.html"
kill "${SERVER_PID}" 2>/dev/null || true
wait "${SERVER_PID}" 2>/dev/null || true
SERVER_PID=""

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

PLAYGROUND_ADDR="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
host, port = s.getsockname()
s.close()
print(f"{host}:{port}")
PY
)"
PLAYGROUND_DATA_ROOT="${TMP}/playground-live"
"${GLADE}" playground --addr "${PLAYGROUND_ADDR}" --data-root "${PLAYGROUND_DATA_ROOT}" --db "${TMP}/playground-live.sqlite" --examples --no-open >"${TMP}/playground-live.log" 2>&1 &
SERVER_PID="$!"
playground_ready=0
for _ in $(seq 1 300); do
  if curl -fsS "http://${PLAYGROUND_ADDR}/playground/api/examples" >"${TMP}/playground-examples.json" 2>/dev/null; then
    playground_ready=1
    break
  fi
  sleep 0.1
done
if [[ "${playground_ready}" != "1" ]]; then
  tail -c 4000 "${TMP}/playground-live.log" >&2
  exit 1
fi
curl -fsS -X POST -H 'Content-Type: application/json' --data '{"id":"refinement-service"}' "http://${PLAYGROUND_ADDR}/playground/api/examples/load" >"${TMP}/refinement-meta.json"
REFINEMENT_PROJECT="${PLAYGROUND_DATA_ROOT}/workspaces/default"
"${GLADE}" init --project "${REFINEMENT_PROJECT}" --yes
"${GLADE}" check --project "${REFINEMENT_PROJECT}" --json >"${TMP}/refinement-check.json"
grep -q '"diagnostics": 0' "${TMP}/refinement-check.json"
"${GLADE}" test --project "${REFINEMENT_PROJECT}" --class RefinementServiceTest --json >"${TMP}/refinement-test.json"
python3 - "${TMP}/refinement-test.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    result = json.load(f)
summary = result.get("summary", {})
if summary.get("total", 0) < 1 or summary.get("passed", 0) < 1 or summary.get("errors", 0) != 0:
    raise SystemExit(f"named RefinementServiceTest did not pass: {summary}")
if not any(test.get("methodName") == "createsAndLabelsFileRow" for test in result.get("tests", [])):
    raise SystemExit("createsAndLabelsFileRow was not executed")
PY
python3 - "${TMP}/refinement-meta.json" "${TMP}/refinement-run.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    meta = json.load(f)
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump({"anonymousBody": meta["anonymousBody"], "mode": "scratch", "limitMode": "permissive"}, f)
PY
curl -fsS -X POST -H 'Content-Type: application/json' --data-binary @"${TMP}/refinement-run.json" "http://${PLAYGROUND_ADDR}/playground/api/run" >"${TMP}/refinement-run-result.json"
python3 - "${TMP}/refinement-run-result.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    result = json.load(f)
if result.get("status") != "pass" or any(d.get("severity") == "error" for d in result.get("diagnostics", [])):
    raise SystemExit(f"refinement example did not run cleanly: {result}")
if "Refine 01 #F-100" not in "\n".join(result.get("logs", [])):
    raise SystemExit(f"refinement label missing: {result}")
if not any(diff.get("object") == "Account" and diff.get("inserted") == 1 for diff in result.get("orgDiff", [])):
    raise SystemExit(f"refinement insert missing: {result}")
PY
curl -fsS "http://${PLAYGROUND_ADDR}/playground/api/database" >"${TMP}/refinement-before-invalid.json"
python3 - "${TMP}/refinement-meta.json" "${TMP}/refinement-invalid-source.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    meta = json.load(f)
source = next(file for file in meta["files"] if file["path"].endswith("RefinementService.cls"))
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump({"path": source["path"], "version": source["version"], "content": "public class RefinementService { public static void broken( { }\n"}, f)
PY
curl -fsS -X PUT -H 'Content-Type: application/json' --data-binary @"${TMP}/refinement-invalid-source.json" "http://${PLAYGROUND_ADDR}/playground/api/files" >"${TMP}/refinement-invalid-save.json"
curl -fsS -X POST -H 'Content-Type: application/json' --data-binary @"${TMP}/refinement-run.json" "http://${PLAYGROUND_ADDR}/playground/api/run" >"${TMP}/refinement-invalid-run.json"
curl -fsS "http://${PLAYGROUND_ADDR}/playground/api/database" >"${TMP}/refinement-after-invalid.json"
cmp -s "${TMP}/refinement-before-invalid.json" "${TMP}/refinement-after-invalid.json"
python3 - "${TMP}/refinement-invalid-run.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    result = json.load(f)
if result.get("status") != "compile_error" or result.get("logs") or result.get("orgDiff"):
    raise SystemExit(f"invalid source must not run or change the org: {result}")
PY
kill "${SERVER_PID}" 2>/dev/null || true
wait "${SERVER_PID}" 2>/dev/null || true
SERVER_PID=""

echo "runtime smoke: ok"
