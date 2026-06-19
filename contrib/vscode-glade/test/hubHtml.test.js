const assert = require("assert");
const html = require("../out/hub/html");

const snapshot = {
  project: {
    workspaceFolder: "/repo",
    projectRoot: "/repo/<script>alert(1)</script>",
    configFound: true,
    namespace: "",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app"],
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 61, records: 1284, users: 18, profiles: 4, permissions: 11 },
  changedSince: "origin/main",
  pluginActionCount: 2,
  pluginFindingCount: 0,
};

const rendered = html.renderHubHtml(snapshot, {
  cspSource: "vscode-resource:",
  nonce: "abc123",
  initialTab: "home",
});

assert(rendered.includes("Glade Home"));
assert(rendered.includes('data-tab="home"'));
assert(rendered.includes('data-tab="state"'));
assert(rendered.includes('data-command="glade.runLocalProof"'));
assert(rendered.includes('data-command="glade.inspectLocalOrg"'));
assert(rendered.includes("Run"));
assert(rendered.includes("Data"));
assert(rendered.includes("Salesforce"));
assert(rendered.includes("&lt;script&gt;alert(1)&lt;/script&gt;"));
assert(!rendered.includes("/repo/<script>alert(1)</script>"));
assert(rendered.includes("script-src 'nonce-abc123'"));
