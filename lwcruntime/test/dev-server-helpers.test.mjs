import assert from "node:assert/strict";
import childProcess from "node:child_process";
import { EventEmitter } from "node:events";
import fs from "node:fs";
import { syncBuiltinESMExports } from "node:module";
import path from "node:path";
import test, { after } from "node:test";
import { repoRoot, startLWCDevServer, startVisualforceDevServer } from "./helpers.mjs";

const fixture = "lwcruntime/test/fixtures/wire-contract";
let binaryDir;

// helpers.mjs registers its process-wide cleanup before this hook.
after(() => {
  if (binaryDir) {
    assert.equal(fs.existsSync(binaryDir), false, "shared binary cleaned after all tests");
  }
});

test("dev server helpers share successful builds while isolating server state", async (t) => {
  const builds = [];
  const servers = [];
  const requests = [];
  t.mock.method(childProcess, "spawnSync", (command, args, options) => {
    assert.equal(command, "go");
    assert.deepEqual(args.slice(0, 2), ["build", "-o"]);
    assert.equal(args[3], "./cmd/glade");
    assert.equal(options.cwd, repoRoot);
    const binary = args[2];
    builds.push(binary);
    fs.writeFileSync(binary, "fake executable");
    return builds.length === 1
      ? { status: 1, stderr: "# package\nfixture compiler failure\n", stdout: "" }
      : { status: 0, stderr: "", stdout: "" };
  });
  t.mock.method(childProcess, "spawn", (binary, args, options) => {
    assert.equal(fs.existsSync(binary), true);
    const child = new EventEmitter();
    child.stdout = new EventEmitter();
    child.stderr = new EventEmitter();
    child.exitCode = null;
    child.signalCode = null;
    child.kill = (signal) => {
      assert.equal(signal, "SIGTERM");
      assert.equal(child.signalCode, null, "server stopped only once");
      child.signalCode = signal;
      child.emit("exit", null, signal);
      return true;
    };
    const readyFile = args[args.indexOf("--ready-file") + 1];
    const projectRoot = args[args.indexOf("--project") + 1];
    const tmpDir = options.env.TMPDIR;
    assert.equal(path.dirname(readyFile), tmpDir);
    assert.equal(options.env.TMP, tmpDir);
    assert.equal(options.env.TEMP, tmpDir);
    assert.notEqual(path.dirname(binary), tmpDir);
    const url = `http://fixture.invalid/${servers.length}`;
    fs.writeFileSync(readyFile, JSON.stringify({ url, pages: ["Fixture"], routes: ["/fixture"] }));
    servers.push({ binary, child, projectRoot, tmpDir, options, url });
    return child;
  });
  t.mock.method(globalThis, "fetch", async (url) => {
    requests.push(url);
    return { ok: true, arrayBuffer: async () => new ArrayBuffer(0) };
  });
  syncBuiltinESMExports();
  t.after(() => {
    t.mock.restoreAll();
    syncBuiltinESMExports();
  });

  await t.test("a failed build is reported, removed and retried", async (t) => {
    const skipped = [];
    const server = await startLWCDevServer({
      after: (fn) => t.after(fn),
      skip: (message) => skipped.push(message),
    }, { projectRel: fixture });
    assert.equal(server, null);
    assert.deepEqual(skipped, ["cannot build local glade binary: fixture compiler failure"]);
    assert.equal(fs.existsSync(path.dirname(builds[0])), false);
    assert.equal(servers.length, 0);
  });

  await t.test("Visualforce gets a private project and principal", async (t) => {
    const server = await startVisualforceDevServer(t, { projectRel: fixture });
    const call = servers.at(-1);
    binaryDir = path.dirname(call.binary);
    assert.deepEqual(server.pages, ["Fixture"]);
    assert.equal(call.projectRoot, path.join(call.tmpDir, "project"));
    assert.equal(call.options.env.GLADE_VISUALFORCE_HTML_USER_ID, "005000000000011");
    const principal = JSON.parse(fs.readFileSync(path.join(call.projectRoot, "data/browser-principal.json"), "utf8"));
    assert.equal(principal.objects[0].records[0].id, "005000000000011");
    fs.writeFileSync(path.join(call.projectRoot, "server-only.txt"), "private state");
    await server.close();
    assert.equal(fs.existsSync(call.tmpDir), false);
    assert.equal(fs.existsSync(call.binary), true);
  });

  await t.test("LWC reuses the binary after the previous test cleans up", async (t) => {
    const server = await startLWCDevServer(t, { projectRel: fixture });
    const call = servers.at(-1);
    assert.deepEqual(server.routes, ["/fixture"]);
    assert.equal(call.projectRoot, path.join(repoRoot, fixture));
    assert.equal(call.binary, servers[0].binary);
    await server.close();
    assert.equal(fs.existsSync(call.tmpDir), false);
  });

  await t.test("another Visualforce server gets fresh project state", async (t) => {
    const server = await startVisualforceDevServer(t, { projectRel: fixture });
    const call = servers.at(-1);
    assert.equal(fs.existsSync(path.join(call.projectRoot, "server-only.txt")), false);
    assert.equal(fs.existsSync(path.join(call.projectRoot, "data/browser-principal.json")), true);
    assert.equal(call.binary, servers[0].binary);
    await server.close();
    assert.equal(fs.existsSync(call.tmpDir), false);
  });

  assert.equal(builds.length, 2, "one failed attempt and one successful build");
  assert.equal(servers.length, 3, "each helper starts a separate server");
  assert.equal(new Set(servers.map((server) => server.tmpDir)).size, 3);
  assert.ok(servers.every((server) => server.child.signalCode === "SIGTERM"));
  assert.deepEqual(requests, [
    `${servers[0].url}/apex/MultiWidgetHost`,
    `${servers[1].url}/lwc/preview/component/c/contextProbe`,
    `${servers[2].url}/apex/MultiWidgetHost`,
  ]);
});
