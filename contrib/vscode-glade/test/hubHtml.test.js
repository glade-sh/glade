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
assert(rendered.includes('class="home-section" data-phase="setup"'));
assert(rendered.includes('class="home-section" data-phase="daily"'));
assert(rendered.includes('class="home-section" data-phase="data"'));
assert(!rendered.includes('class="home-section" data-phase="review"'));
assert(rendered.includes('class="primary-action"'));
assert(rendered.includes('class="more-actions"'));
assert(rendered.includes("<summary>More</summary>"));
assert(!rendered.includes('class="actions"'));
assert(rendered.includes("First project setup"));
assert(rendered.includes("Daily work"));
assert(rendered.includes("Database inspection"));
for (const command of [
  "glade.seedLocalOrg",
  "glade.resetLocalOrg",
  "glade.exportLocalOrg",
  "glade.cloneEnvironment",
  "glade.schemaImportDescribe",
  "glade.runPluginAction",
  "glade.managePlugins",
]) {
  assert(!rendered.includes(`data-command="${command}"`));
}
assert(rendered.includes("&lt;script&gt;alert(1)&lt;/script&gt;"));
assert(!rendered.includes("/repo/<script>alert(1)</script>"));
assert(rendered.includes("script-src 'nonce-abc123'"));
