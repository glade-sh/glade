const assert = require("assert");
const fs = require("fs");
const os = require("os");
const path = require("path");

const {
  PluginController,
  pluginActionRows,
  pluginArtifactRows,
} = require("../out/plugins/controller");

const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), "glade-vscode-plugin-"));
const projectRoot = path.join(tempRoot, "project");
fs.mkdirSync(projectRoot, { recursive: true });

const plugin = {
  name: "compat",
  identityName: "@glade/compat",
  version: "1.2.3",
  linked: true,
  editor: {
    actions: [
      {
        id: "compat.scan",
        title: "Scan project",
        command: ["compat", "scan"],
        args: ["--project", "${projectRoot}", "--out", "${outputDir}", "--label", "${input.label}"],
        view: "plugins",
        contexts: ["project"],
        inputs: [{ name: "label", label: "Report label", type: "text", required: true, default: "surface-default" }],
        output: "glade.findings.v1",
      },
      {
        id: "compat.debug",
        title: "Explain log",
        command: ["debug", "explain"],
        args: ["${activeFile}"],
        view: "debug",
        contexts: ["activeDebugLog"],
      },
    ],
  },
};

const calls = [];
const diagnostics = [];
const messages = [];
const controller = new PluginController({
  async project() {
    return { projectRoot, workspaceFolder: projectRoot };
  },
  activeFile() {
    return path.join(projectRoot, "force-app/main/default/classes/Foo.cls");
  },
  activeDb() {
    return path.join(projectRoot, ".glade/envs/dev.sqlite");
  },
  async inputBox(options) {
    calls.push({ type: "input", prompt: options.prompt, title: options.title, value: options.value });
    return options.value;
  },
  async openDialog() {
    return [path.join(tempRoot, "plugin.tgz")];
  },
  async runGlade(args, options) {
    calls.push({ type: "run", args, cwd: options.cwd });
    if (args.join(" ") === "plugins list --json") {
      return { code: 0, stdout: JSON.stringify({ plugins: [plugin] }), stderr: "" };
    }
    if (args[0] === "compat") {
      return {
        code: 0,
        stdout: JSON.stringify({
          kind: "glade.findings.v1",
          findings: [
            {
              severity: "error",
              file: "force-app/main/default/classes/Foo.cls",
              line: 7,
              column: 5,
              message: "Unsupported call",
              ruleId: "Apex.Unsupported",
            },
          ],
          artifacts: [{ label: "Surface report", path: path.join(projectRoot, ".glade/editor/plugins/compat-scan/report.json") }],
        }),
        stderr: "",
      };
    }
    return { code: 0, stdout: "", stderr: "" };
  },
  diagnostics: {
    set(entries) {
      diagnostics.push(entries);
    },
    clear() {
      diagnostics.push([]);
    },
  },
  log(message) {
    messages.push(message);
  },
});

(async () => {
  await controller.refresh();
  assert.deepStrictEqual(controller.plugins().map((entry) => entry.name), ["compat"]);
  assert.deepStrictEqual(
    controller.actionsForView("plugins", { project: true }).map((action) => action.id),
    ["compat.scan"],
  );
  assert.deepStrictEqual(
    pluginActionRows(controller.actionsForView("debug", { activeDebugLog: true })).map((row) => row.label),
    ["Explain log"],
  );

  await controller.runAction(plugin.editor.actions[0]);

  const actionRun = calls.find((call) => call.type === "run" && call.args[0] === "compat");
  assert(actionRun, "plugin action must invoke glade with action argv");
  assert.deepStrictEqual(actionRun.args.slice(0, 2), ["compat", "scan"]);
  assert(actionRun.args.includes(path.join(projectRoot, ".glade/editor/plugins/compat-scan")));
  assert(actionRun.args.includes("surface-default"));
  assert(fs.existsSync(path.join(projectRoot, ".glade/editor/plugins/compat-scan")), "action output directory must exist");
  assert.deepStrictEqual(
    calls.filter((call) => call.type === "input").map((call) => call.value),
    ["surface-default"],
  );

  assert.strictEqual(diagnostics.length, 1);
  assert.strictEqual(diagnostics[0][0].file, path.join(projectRoot, "force-app/main/default/classes/Foo.cls"));
  assert.strictEqual(diagnostics[0][0].severity, "error");
  assert.deepStrictEqual(
    pluginArtifactRows(controller.latestArtifacts()).map((row) => row.label),
    ["Surface report"],
  );
  assert.strictEqual(messages.length, 0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
