const assert = require("assert");
const model = require("../out/preview/model");
const cli = require("../out/preview/cli");

assert.deepStrictEqual(
  model.parseLWCReadyFile(JSON.stringify({
    url: "http://127.0.0.1:4173",
    addr: "127.0.0.1:4173",
    routes: [
      "/lwc/preview/component/c/contextProbe",
      "/lwc/preview/tab/Visualforce_Tab -> /apex/WidgetHost",
    ],
  })),
  {
    kind: "lwc",
    url: "http://127.0.0.1:4173",
    addr: "127.0.0.1:4173",
    running: true,
    routes: [
      {
        label: "c/contextProbe",
        path: "/lwc/preview/component/c/contextProbe",
      },
      {
        label: "WidgetHost",
        path: "/apex/WidgetHost",
        sourcePath: "/lwc/preview/tab/Visualforce_Tab",
      },
    ],
  },
);

assert.deepStrictEqual(
  model.parseVFReadyFile(JSON.stringify({
    url: "http://127.0.0.1:4174",
    addr: "127.0.0.1:4174",
    pages: ["/apex/Core"],
  })),
  {
    kind: "visualforce",
    url: "http://127.0.0.1:4174",
    addr: "127.0.0.1:4174",
    running: true,
    routes: [
      {
        label: "Core",
        path: "/apex/Core",
      },
    ],
  },
);

assert.deepStrictEqual(
  model.parseToolchainStatus(JSON.stringify({ ok: true, path: "/usr/local/bin/node" })),
  { ok: true, path: "/usr/local/bin/node", detail: "unknown" },
);

assert.deepStrictEqual(
  model.parseToolchainStatus(JSON.stringify({ ok: false, detail: "node missing" })),
  { ok: false, detail: "node missing" },
);

assert.deepStrictEqual(cli.toolchainStatusArgs(), ["toolchain", "status", "--json"]);
assert.deepStrictEqual(cli.toolchainInstallArgs(), ["toolchain", "install"]);
assert.deepStrictEqual(
  cli.devLWCArgs("/repo", "127.0.0.1:4173", "/tmp/lwc-ready.json"),
  ["dev", "lwc", "--project", "/repo", "--addr", "127.0.0.1:4173", "--ready-file", "/tmp/lwc-ready.json"],
);
assert.deepStrictEqual(
  cli.devVFArgs("/repo", "127.0.0.1:4174", "/tmp/vf-ready.json"),
  ["dev", "vf", "--project", "/repo", "--addr", "127.0.0.1:4174", "--ready-file", "/tmp/vf-ready.json"],
);
