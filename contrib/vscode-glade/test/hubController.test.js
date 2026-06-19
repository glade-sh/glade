const assert = require("assert");
const Module = require("module");

let messageHandler;
let disposeHandler;
const panel = {
  htmlWrites: 0,
  revealCount: 0,
  disposeCount: 0,
  options: undefined,
  webview: {
    cspSource: "vscode-resource:",
    asWebviewUri(uri) {
      return {
        toString() {
          return `vscode-resource:${uri.path}`;
        },
      };
    },
    set html(value) {
      this.latestHtml = value;
      panel.htmlWrites += 1;
    },
    onDidReceiveMessage(handler) {
      messageHandler = handler;
      return { dispose() {} };
    },
  },
  onDidDispose(handler) {
    disposeHandler = handler;
    return { dispose() {} };
  },
  reveal() {
    this.revealCount += 1;
  },
  dispose() {
    this.disposeCount += 1;
  },
};

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    return {
      Uri: {
        joinPath(base, ...segments) {
          return { path: [base.path, ...segments].join("/") };
        },
      },
      ViewColumn: { One: 1 },
      window: {
        createWebviewPanel(_viewType, _title, _column, options) {
          panel.options = options;
          return panel;
        },
      },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const { GladeHomeController } = require("../out/hub/controller");

const executed = [];
const controller = new GladeHomeController({ subscriptions: [], extensionUri: { path: "/extension" } }, {
  snapshot: () => ({ changedSince: "origin/main" }),
  executeCommand(command) {
    executed.push(command);
    return Promise.resolve();
  },
});

(async () => {
  controller.open();
  assert.strictEqual(panel.htmlWrites, 1);
  assert.deepStrictEqual(panel.options.localResourceRoots, [{ path: "/extension/media" }]);
  assert(panel.webview.latestHtml.includes("Glade Home"));
  assert(panel.webview.latestHtml.includes("vscode-resource:/extension/media/glade-brand.svg"));

  await messageHandler({ type: "ready" });
  assert.strictEqual(panel.htmlWrites, 1);

  await messageHandler({ type: "selectTab", tab: "state" });
  assert.strictEqual(panel.htmlWrites, 1);
  controller.update();
  assert(panel.webview.latestHtml.includes('data-tab="state" aria-selected="true"'));
  assert(panel.webview.latestHtml.includes('data-panel="state">'));

  await messageHandler({ type: "runCommand", command: "glade.runLocalProof" });
  assert.deepStrictEqual(executed, ["glade.runLocalProof"]);
  assert.strictEqual(panel.htmlWrites, 3);

  await messageHandler({ type: "runCommand", command: "workbench.action.files.delete" });
  assert.deepStrictEqual(executed, ["glade.runLocalProof"]);

  disposeHandler();
  controller.open();
  assert(panel.webview.latestHtml.includes('data-tab="home" aria-selected="true"'));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
