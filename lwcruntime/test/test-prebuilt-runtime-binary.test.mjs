import assert from "node:assert/strict";
import childProcess from "node:child_process";
import { EventEmitter } from "node:events";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { syncBuiltinESMExports } from "node:module";
import test, { after } from "node:test";
import { repoRoot, startLWCDevServer } from "./helpers.mjs";

const fixtureDir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-prebuilt-runtime-binary-"));
const binaryPath = path.join(fixtureDir, "glade");
fs.writeFileSync(binaryPath, "fixture binary");

after(() => fs.rmSync(fixtureDir, { recursive: true, force: true }));

test("dev server helpers reuse a configured shared binary", async (t) => {
  const previous = process.env.GLADE_LWCRUNTIME_TEST_BINARY;
  process.env.GLADE_LWCRUNTIME_TEST_BINARY = path.relative(repoRoot, binaryPath);
  t.after(() => {
    if (previous === undefined) delete process.env.GLADE_LWCRUNTIME_TEST_BINARY;
    else process.env.GLADE_LWCRUNTIME_TEST_BINARY = previous;
  });

  t.mock.method(childProcess, "spawnSync", () => {
    throw new Error("configured binary should avoid another Go build");
  });
  t.mock.method(childProcess, "spawn", (binary, args) => {
    assert.equal(binary, binaryPath);
    const child = new EventEmitter();
    child.stdout = new EventEmitter();
    child.stderr = new EventEmitter();
    child.exitCode = null;
    child.signalCode = null;
    child.kill = (signal) => {
      child.signalCode = signal;
      queueMicrotask(() => child.emit("exit", null, signal));
      return true;
    };
    const readyFile = args[args.indexOf("--ready-file") + 1];
    fs.writeFileSync(readyFile, JSON.stringify({ url: "http://fixture.invalid", routes: ["/fixture"] }));
    return child;
  });
  t.mock.method(globalThis, "fetch", async () => ({
    ok: true,
    arrayBuffer: async () => new ArrayBuffer(0),
  }));
  syncBuiltinESMExports();

  const server = await startLWCDevServer({
    after: (fn) => t.after(fn),
    skip: (message) => { throw new Error(message); },
  }, { projectRel: "lwcruntime/test/fixtures/wire-contract" });
  assert.deepEqual(server.routes, ["/fixture"]);
  await server.close();
});
