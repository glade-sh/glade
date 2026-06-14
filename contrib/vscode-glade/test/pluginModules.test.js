const assert = require("assert");
const actions = require("../out/plugins/actions");
const cli = require("../out/plugins/cli");
const findings = require("../out/plugins/findings");
const model = require("../out/plugins/model");

const plugin = {
  name: "compat",
  identityName: "@glade/compat",
  canonicalName: "@glade/compat",
  version: "1.2.3",
  linked: false,
  commands: ["compat"],
  executable: "/repo/.glade/plugins/compat/bin/glade-compat",
  manifest: "/repo/.glade/plugins/compat/glade.plugin.json",
  source: "local",
  editor: {
    actions: [
      {
        id: "compat.open",
        title: "Open compat report",
        icon: "graph",
        command: ["compat", "report"],
        args: ["--project", "${projectRoot}", "--out", "${outputDir}", "--name", "${input.report}"],
        view: "plugins",
        contexts: ["project"],
        inputs: [{ name: "report", label: "Report name", required: true }],
        output: "glade.findings.v1",
      },
      {
        id: "compat.debug",
        title: "Debug current log",
        command: ["debug", "explain"],
        args: ["${activeFile}", "--db", "${activeDb}"],
        view: "debug",
        contexts: ["activeDebugLog", "activeDataEnvironment"],
      },
    ],
  },
};

assert.deepStrictEqual(model.supportedActionViews, [
  "startHere",
  "runs",
  "localOrg",
  "debug",
  "preview",
  "plugins",
]);
assert(model.supportedActionContexts.includes("activeApexFile"));
assert(model.supportedActionContexts.includes("lastLocalRun"));

assert.deepStrictEqual(cli.pluginsListArgs(), ["plugins", "list", "--json"]);
assert.deepStrictEqual(cli.pluginsDoctorArgs(), ["plugins", "doctor", "--json"]);

assert.deepStrictEqual(
  actions.actionsForView([plugin], "plugins", { project: true }).map((action) => action.id),
  ["compat.open"],
);
assert.deepStrictEqual(
  actions.actionsForView([plugin], "startHere", { project: false }).map((action) => action.id),
  [],
);
assert.deepStrictEqual(
  actions.actionsForView([plugin], "debug", { activeDebugLog: true, activeDataEnvironment: true }).map((action) => action.id),
  ["compat.debug"],
);

const resolved = actions.resolveAction(plugin.editor.actions[0], {
  projectRoot: "/repo",
  workspaceFolder: "/repo",
  activeFile: "/repo/force-app/classes/Foo.cls",
  activeDb: "/repo/.glade/envs/dev.sqlite",
  outputDir: "/repo/.glade/plugins/compat",
  inputs: { report: "surface" },
});
assert.deepStrictEqual(resolved, {
  ok: true,
  action: {
    id: "compat.open",
    title: "Open compat report",
    icon: "graph",
    command: ["compat", "report"],
    args: ["--project", "/repo", "--out", "/repo/.glade/plugins/compat", "--name", "surface"],
    argv: ["compat", "report", "--project", "/repo", "--out", "/repo/.glade/plugins/compat", "--name", "surface"],
    output: "glade.findings.v1",
  },
});

const missing = actions.resolveAction(plugin.editor.actions[1], {
  projectRoot: "/repo",
  workspaceFolder: "/repo",
  outputDir: "/repo/.glade/plugins/compat",
  inputs: {},
});
assert.deepStrictEqual(missing, {
  ok: false,
  error: {
    code: "missingTokenValue",
    message: "Missing values for action tokens: activeFile, activeDb",
    missingTokens: ["activeFile", "activeDb"],
  },
});

const parsed = findings.parseFindingsOutput(JSON.stringify({
  kind: "glade.findings.v1",
  summary: { errors: 1, warnings: 1 },
  findings: [
    {
      severity: "fatal",
      file: "force-app/classes/Foo.cls",
      line: 12,
      column: 3,
      message: "Unsupported call",
      ruleId: "Apex.Unsupported",
      source: "compat",
    },
    {
      severity: "hint",
      message: "Try a local fixture",
      source: "compat",
    },
  ],
  artifacts: [{ label: "Report", path: "/tmp/report.json", kind: "json" }],
}));
assert.strictEqual(parsed.kind, "glade.findings.v1");
assert.strictEqual(parsed.findings[0].severity, "warning");
assert.strictEqual(parsed.findings[1].severity, "hint");
assert.deepStrictEqual(parsed.findings[0], {
  severity: "warning",
  file: "force-app/classes/Foo.cls",
  line: 12,
  column: 3,
  message: "Unsupported call",
  ruleId: "Apex.Unsupported",
  source: "compat",
});
assert.deepStrictEqual(parsed.artifacts, [{ label: "Report", path: "/tmp/report.json", kind: "json" }]);
