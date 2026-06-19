const assert = require("assert");
const Module = require("module");

const originalLoad = Module._load;
let activeEnvironment;
let configuredEnvironments;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    class TreeItem {
      constructor(label, collapsibleState) {
        this.label = label;
        this.collapsibleState = collapsibleState;
      }
    }
    class ThemeIcon {
      constructor(id) {
        this.id = id;
      }
    }
    class EventEmitter {
      constructor() {
        this.event = () => undefined;
      }
      fire() {}
    }
    return {
      TreeItem,
      ThemeIcon,
      EventEmitter,
      TreeItemCollapsibleState: { None: 0, Expanded: 2 },
      Uri: { file: (fsPath) => ({ fsPath }), parse: (uri) => ({ uri }) },
      debug: { breakpoints: [] },
      workspace: {
        getConfiguration: () => ({
          get: (key) => {
            if (key === "activeEnvironment") {
              return activeEnvironment;
            }
            if (key === "environments") {
              return configuredEnvironments;
            }
            return undefined;
          },
        }),
      },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const { EnvironmentsView } = require("../out/views/environmentsView");
const { LocalOrgView } = require("../out/views/localOrgView");
const { DebugView } = require("../out/views/debugView");
const { PluginsView } = require("../out/views/pluginsView");
const { RunsView } = require("../out/views/runsView");
const { StartHereView } = require("../out/views/startHereView");
const { StartHereState } = require("../out/startHereState");

const project = {
  workspaceFolder: "/repo",
  projectRoot: "/repo",
  configFound: true,
  namespace: "",
  sourceApiVersion: "63.0",
  packageDirs: ["force-app"],
};

const environments = new EnvironmentsView();
environments.setProject(project);
assert.deepStrictEqual(
  environments.getChildren().map((item) => item.label),
  ["dev", "Create data environment"],
);

const localOrg = new LocalOrgView();
localOrg.setProject(project);
assert.deepStrictEqual(
  localOrg.getChildren().map((item) => item.label),
  ["Active: dev", "Inspect data"],
);
configuredEnvironments = [
  { name: "dev", dbPath: ".glade/envs/dev.sqlite" },
  { name: "qa", dbPath: ".glade/envs/qa.sqlite" },
];
activeEnvironment = "qa";
localOrg.refresh();
assert.deepStrictEqual(
  localOrg.getChildren().map((item) => item.label),
  ["Active: qa", "Inspect data"],
);
activeEnvironment = undefined;
configuredEnvironments = undefined;

const debug = new DebugView();
assert.deepStrictEqual(debug.getChildren(), []);
debug.setProject(project);
assert.deepStrictEqual(
  debug.getChildren().map((item) => item.label),
  ["Debug current test"],
);

const plugins = new PluginsView();
assert.deepStrictEqual(
  plugins.getChildren().map((item) => item.label),
  ["Manage plugins"],
);

const runs = new RunsView();
assert.deepStrictEqual(runs.getChildren(), []);
runs.setState({ projectReady: true });
assert.deepStrictEqual(
  runs.getChildren().map((item) => item.label),
  ["Run changed tests", "Start watch"],
);
runs.setState({ lastRun: { label: "Changed tests", passed: 1, failed: 2 }, watchRunning: true });
assert.deepStrictEqual(
  runs.getChildren().map((item) => item.label),
  ["Run changed tests", "Stop watch"],
);
runs.setState({ failedTestCount: 2 });
assert.deepStrictEqual(
  runs.getChildren().map((item) => item.label),
  ["Run changed tests", "Run failed tests", "Stop watch"],
);

const startHere = new StartHereView(new StartHereState());
startHere.setProject(project);
startHere.setPluginActions([
  { id: "plugin.one", label: "Plugin One", action: { id: "one", command: "one", title: "One" } },
  { id: "plugin.two", label: "Plugin Two", action: { id: "two", command: "two", title: "Two" } },
]);
assert.deepStrictEqual(
  startHere.getChildren().map((item) => item.label),
  ["Glade Home", "Data environment", "Run changed tests"],
);
