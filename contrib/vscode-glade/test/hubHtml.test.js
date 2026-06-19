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
  initialHomeLane: "scratch",
  initialStateLane: "plugins",
  logoUri: "vscode-resource:/media/glade-brand.svg",
});
const csp = rendered.match(/Content-Security-Policy" content="([^"]+)"/)?.[1] || "";

assert(rendered.includes("Glade Home"));
assert(rendered.includes('class="brand-mark"'));
assert(rendered.includes('src="vscode-resource:/media/glade-brand.svg"'));
assert(rendered.includes("Local Apex workbench"));
assert(csp.split(";").map((part) => part.trim()).includes("img-src vscode-resource:"));
assert(!csp.includes("https:"));
assert(!csp.includes("data:"));
assert(rendered.includes("--glade: #9be870"));
assert(rendered.includes("--glade-strong: #b7ff8a"));
assert(rendered.includes("--brand-shell: #070b0d"));
assert(rendered.includes("--warn: #f5c95f"));
assert(rendered.includes("--error: #ff6b61"));
assert(rendered.includes("--info: #7db7ff"));
assert(rendered.includes('data-tab="home"'));
assert(rendered.includes('data-tab="state"'));
assert(rendered.includes('data-command="glade.runLocalProof"'));
assert(rendered.includes('data-command="glade.inspectLocalOrg"'));
assert(rendered.includes('class="rail-layout"'));
assert(rendered.includes('class="hub-rail" role="tablist"'));
assert(rendered.includes('data-lane="data"'));
assert(rendered.includes('data-lane="run"'));
assert(rendered.includes('data-lane="org"'));
assert(rendered.includes('data-lane="scratch"'));
assert(rendered.includes('data-lane="scratch" aria-selected="true"'));
assert(rendered.includes('role="tab" id="home-tab-scratch"'));
assert(rendered.includes('aria-controls="home-panel-scratch"'));
assert(rendered.includes('role="tabpanel" id="home-panel-scratch"'));
assert(rendered.includes('data-lane="salesforce"'));
assert(rendered.includes('data-lane-detail="data"'));
assert(rendered.includes('data-state-lane="data"'));
assert(rendered.includes('data-state-lane="plugins" aria-selected="true"'));
assert(rendered.includes('role="tab" id="state-tab-plugins"'));
assert(rendered.includes('aria-controls="state-panel-plugins"'));
assert(rendered.includes('role="tabpanel" id="state-panel-plugins"'));
assert(rendered.includes('data-state-detail="data"'));
assert(rendered.includes("Data browser"));
assert(rendered.includes("Local tests"));
assert(rendered.includes("Glade org"));
assert(rendered.includes("Scratch editors"));
assert(rendered.includes("Salesforce"));
assert(rendered.includes("Data environment"));
assert(!rendered.includes("Daily work"));
assert(!rendered.includes("Day to day"));
assert(!rendered.includes("First project setup"));
assert(!rendered.includes('class="home-section"'));
assert(!rendered.includes('class="primary-action"'));
assert(!rendered.includes("<summary>More</summary>"));
assert(rendered.includes('data-command="glade.seedLocalOrg"'));
assert(rendered.includes('data-command="glade.resetLocalOrg"'));
assert(rendered.includes('data-command="glade.exportLocalOrg"'));
assert(rendered.includes('data-command="glade.schemaImportDescribe"'));
assert(!rendered.includes('data-command="glade.runPluginAction"'));
assert(!rendered.includes('data-command="glade.managePlugins"'));
assert(!rendered.includes("width: 100%"));
assert(rendered.includes("&lt;script&gt;alert(1)&lt;/script&gt;"));
assert(!rendered.includes("/repo/<script>alert(1)</script>"));
assert(rendered.includes("script-src 'nonce-abc123'"));
