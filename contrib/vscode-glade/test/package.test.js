const assert = require("assert");
const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const manifest = require(path.join(root, "package.json"));
const ignore = fs.readFileSync(path.join(root, ".vscodeignore"), "utf8")
  .split(/\r?\n/)
  .map((line) => line.trim())
  .filter((line) => line && !line.startsWith("#"));

if (manifest.dependencies && Object.keys(manifest.dependencies).length > 0) {
  const packageScript = manifest.scripts.package || "";
  const bundleScript = manifest.scripts.bundle || "";
  const bundleStep = packageScript.indexOf("npm run bundle");
  const packageStep = packageScript.indexOf("vsce package");
  assert(
    /\besbuild\b/.test(bundleScript),
    "runtime dependencies must be bundled before node_modules is excluded from the VSIX",
  );
  assert(bundleScript.includes("--minify"), "bundled extension JavaScript must be minified for VSIX packaging");
  assert(bundleStep >= 0 && packageStep > bundleStep, "VSIX packaging must run after the bundle step");
  assert(
    ignore.includes("node_modules/**"),
    ".vscodeignore must exclude bundled node_modules from the VSIX",
  );
}

assert(ignore.includes("out/**/*.js"), ".vscodeignore must exclude compiled module JavaScript");
assert(ignore.includes("!out/extension.js"), ".vscodeignore must keep the bundled extension entrypoint");
assert(ignore.includes("out/**/*.map"), ".vscodeignore must exclude compiled source maps from the VSIX");

const startHereView = manifest.contributes.views.glade.find((view) => view.id === "glade.project");
assert(startHereView, "glade.project view must exist");
assert.strictEqual(startHereView.name, "Start Here");

const viewIds = manifest.contributes.views.glade.map((view) => view.id);
assert.deepStrictEqual(viewIds, [
  "glade.project",
  "glade.recommendedRuns",
  "glade.environments",
  "glade.localOrg",
  "glade.workbench",
  "glade.debugLogs",
  "glade.plugins",
]);
assert(!viewIds.includes("glade.apexTests"), "local Apex tests must use native Testing, not a duplicate sidebar view");
assert(!viewIds.includes("glade.preview"), "LWC and Visualforce preview must stay out of the VS Code sidebar");

const activationEvents = manifest.activationEvents || [];
assert(!activationEvents.includes("onView:glade.apexTests"), "glade.apexTests activation must be removed");
assert(!activationEvents.includes("onView:glade.preview"), "glade.preview must not activate the extension");
assert(activationEvents.includes("onView:glade.plugins"), "glade.plugins view must activate the extension");
assert(activationEvents.includes("onView:glade.workbench"), "glade.workbench view must activate the extension");

const localRunsView = manifest.contributes.views.glade.find((view) => view.id === "glade.recommendedRuns");
assert(localRunsView, "glade.recommendedRuns view must exist");
assert.strictEqual(localRunsView.name, "Local Runs");

const debugView = manifest.contributes.views.glade.find((view) => view.id === "glade.debugLogs");
assert(debugView, "glade.debugLogs view must exist");
assert.strictEqual(debugView.name, "Debug");

const pluginsView = manifest.contributes.views.glade.find((view) => view.id === "glade.plugins");
assert(pluginsView, "glade.plugins view must exist");
assert.strictEqual(pluginsView.name, "Plugins");

const workbenchView = manifest.contributes.views.glade.find((view) => view.id === "glade.workbench");
assert(workbenchView, "glade.workbench view must exist");
assert.strictEqual(workbenchView.name, "Exec & SOQL");

for (const command of [
  "glade.runLocalProof",
  "glade.cloneEnvironment",
  "glade.deleteEnvironment",
  "glade.revealEnvironmentDb",
  "glade.inspectEnvironment",
  "glade.statusQuickPick",
  "glade.openOutput",
  "glade.refreshPlugins",
  "glade.managePlugins",
  "glade.runPluginAction",
  "glade.linkLocalPlugin",
  "glade.installPluginArchive",
  "glade.workbench.newAnonymousApex",
  "glade.workbench.newSoql",
  "glade.workbench.runEntry",
  "glade.workbench.runLastAnonymousApex",
  "glade.workbench.runLastSoql",
  "glade.workbench.describe",
  "glade.workbench.openResult",
]) {
  assert(
    manifest.contributes.commands.some((entry) => entry.command === command),
    `${command} must be contributed`,
  );
}

for (const command of [
  "glade.refreshPreview",
  "glade.startLWCPreview",
  "glade.stopLWCPreview",
  "glade.startVFPreview",
  "glade.stopVFPreview",
  "glade.openPreviewRoute",
  "glade.installToolchain",
]) {
  assert(
    !manifest.contributes.commands.some((entry) => entry.command === command),
    `${command} must not be contributed while preview is parked`,
  );
}
