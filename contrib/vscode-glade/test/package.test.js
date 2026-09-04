const assert = require("assert");
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const manifest = require(path.join(root, "package.json"));
const lock = require(path.join(root, "package-lock.json"));
assert.strictEqual(manifest.license, "Apache-2.0");
const canonicalApache20SHA256 = "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30";
const license = fs.readFileSync(path.join(root, "LICENSE"));
assert.strictEqual(crypto.createHash("sha256").update(license).digest("hex"), canonicalApache20SHA256);
assert.strictEqual(fs.readFileSync(path.join(root, "NOTICE"), "utf8"), "Glade\nCopyright 2026 Matt Simonis\n");
assert.strictEqual(manifest.version, "0.0.2");
assert.strictEqual(lock.version, manifest.version);
assert.strictEqual(lock.packages[""].version, manifest.version);
assert.strictEqual(manifest.homepage, "https://glade.sh/guide/editor");
assert.deepStrictEqual(manifest.repository, {
  type: "git", url: "https://github.com/glade-sh/glade.git", directory: "contrib/vscode-glade",
});
assert.strictEqual(manifest.bugs?.url, "https://github.com/glade-sh/glade/issues");
const readme = fs.readFileSync(path.join(root, "README.md"), "utf8");
const mediaDir = path.join(root, "media");
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
  assert(
    bundleScript.includes("--metafile=out/extension.meta.json"),
    "VSIX packaging must retain esbuild input evidence for archive SBOM inventory",
  );
  assert(
    bundleScript.includes("node scripts/bundle-notices.mjs"),
    "VSIX packaging must retain hash-bound dependency and notice evidence",
  );
  assert(bundleStep >= 0 && packageStep > bundleStep, "VSIX packaging must run after the bundle step");
  assert(
    ignore.includes("node_modules/**"),
    ".vscodeignore must exclude bundled node_modules from the VSIX",
  );
}

assert(ignore.includes("out/**/*.js"), ".vscodeignore must exclude compiled module JavaScript");
assert(ignore.includes("!out/extension.js"), ".vscodeignore must keep the bundled extension entrypoint");
assert(ignore.includes("out/**/*.map"), ".vscodeignore must exclude compiled source maps from the VSIX");
assert(ignore.includes("scripts/**"), ".vscodeignore must exclude the build-only notice helper from the VSIX");
assert(
  !ignore.includes("out/extension.meta.json"),
  ".vscodeignore must retain bundled dependency evidence for archive SBOM inventory",
);
for (const evidence of ["out/bundled-dependencies.json", "out/THIRD_PARTY_NOTICES.txt"]) {
  assert(!ignore.includes(evidence), `.vscodeignore must retain ${evidence} for packaged notice coverage`);
}
assert(fs.existsSync(path.join(root, "scripts", "bundle-notices.mjs")), "bundle notice helper must exist");
assert(
  manifest.scripts.package.includes("node test/vsix-package.test.js"),
  "VSIX packaging must inspect the final archive for project and dependency notices",
);
assert(fs.existsSync(path.join(root, "test", "vsix-package.test.js")), "VSIX archive test must exist");
assert(!ignore.includes("prototypes/**"), ".vscodeignore must not carry stale prototype exclusions");
assert(!fs.existsSync(path.join(root, "prototypes")), "standalone prototypes must not be tracked with the release extension");

const startHereView = manifest.contributes.views.glade.find((view) => view.id === "glade.project");
assert(startHereView, "glade.project view must exist");
assert.strictEqual(startHereView.name, "Start Here");

const activityIcon = manifest.contributes.viewsContainers.activitybar.find((entry) => entry.id === "glade")?.icon;
assert.strictEqual(activityIcon, "media/glade.svg", "Glade Activity Bar must use the branded Glade mark");
assert(fs.existsSync(path.join(mediaDir, "glade.svg")), "Activity Bar icon asset must exist");
assert(fs.existsSync(path.join(mediaDir, "glade-brand.svg")), "Home webview brand mark asset must exist");
assert(readme.includes("Glade contour mark"), "README must document the branded Activity Bar mark");

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
for (const view of manifest.contributes.views.glade) {
  if (view.id === "glade.project") {
    assert.strictEqual(view.visibility, "visible", "Start Here should stay open as the calm entry point");
  } else if (view.id === "glade.debugLogs" || view.id === "glade.plugins") {
    assert.strictEqual(view.visibility, "hidden", `${view.id} should stay out of the first-open sidebar`);
  } else {
    assert.strictEqual(view.visibility, "collapsed", `${view.id} should be collapsed on first open`);
  }
}
assert(!viewIds.includes("glade.apexTests"), "local Apex tests must use native Testing, not a duplicate sidebar view");
assert(!viewIds.includes("glade.preview"), "LWC and Visualforce preview must stay out of the VS Code sidebar");

const activationEvents = manifest.activationEvents || [];
assert(!activationEvents.includes("onView:glade.apexTests"), "glade.apexTests activation must be removed");
assert(!activationEvents.includes("onView:glade.preview"), "glade.preview must not activate the extension");
assert(activationEvents.includes("onView:glade.plugins"), "glade.plugins view must activate the extension");
assert(activationEvents.includes("onView:glade.workbench"), "glade.workbench view must activate the extension");
assert(activationEvents.includes("onLanguage:apexlog"), "Apex Log editors must activate the extension");
assert(activationEvents.includes("onLanguage:soql"), "SOQL scratch editors must activate the extension");
for (const command of [
  "glade.openHome",
  "glade.startProjectOrg",
  "glade.stopProjectOrg",
	  "glade.projectOrgStatus",
	  "glade.replayDebugLog",
	  "glade.apexLog.refreshAnalysis",
	  "glade.apexLog.treatAsApexLog",
	  "glade.apexLog.replayFromFrame",
	  "glade.schemaImportDescribe",
  "glade.salesforceTargetStatus",
  "glade.openTui",
  "glade.openTestsTui",
  "glade.openDataTui",
  "glade.openPluginsTui",
]) {
  assert(activationEvents.includes(`onCommand:${command}`), `${command} must activate the extension`);
}

const localRunsView = manifest.contributes.views.glade.find((view) => view.id === "glade.recommendedRuns");
assert(localRunsView, "glade.recommendedRuns view must exist");
assert.strictEqual(localRunsView.name, "Tests");

const debugView = manifest.contributes.views.glade.find((view) => view.id === "glade.debugLogs");
assert(debugView, "glade.debugLogs view must exist");
assert.strictEqual(debugView.name, "Debug");

const pluginsView = manifest.contributes.views.glade.find((view) => view.id === "glade.plugins");
assert(pluginsView, "glade.plugins view must exist");
assert.strictEqual(pluginsView.name, "Plugins");

const workbenchView = manifest.contributes.views.glade.find((view) => view.id === "glade.workbench");
assert(workbenchView, "glade.workbench view must exist");
assert.strictEqual(workbenchView.name, "Apex & SOQL");
assert(readme.includes("Apex & SOQL"), "README must document the Apex & SOQL sidebar view");

const commandPalette = manifest.contributes.menus.commandPalette || [];
const hiddenCommands = new Set(commandPalette.filter((entry) => entry.when === "false").map((entry) => entry.command));
const visibleCommandAllowlist = [
  "glade.openHome",
  "glade.runLocalProof",
  "glade.startProjectOrg",
  "glade.stopProjectOrg",
	  "glade.inspectLocalOrg",
	  "glade.replayDebugLog",
	  "glade.apexLog.refreshAnalysis",
	  "glade.apexLog.treatAsApexLog",
	  "glade.statusQuickPick",
  "glade.workbench.newAnonymousApex",
  "glade.workbench.newSoql",
  "glade.managePlugins",
  "glade.openTui",
  "glade.openTestsTui",
  "glade.openDataTui",
  "glade.openPluginsTui",
];
assert.strictEqual(hiddenCommands.size, commandPalette.length, "command palette hidden entries must not repeat commands");
for (const entry of commandPalette) {
  assert(
    manifest.contributes.commands.some((command) => command.command === entry.command),
    `${entry.command} must be a contributed command`,
  );
}
assert.deepStrictEqual(
  manifest.contributes.commands
    .filter((entry) => !hiddenCommands.has(entry.command))
    .map((entry) => entry.command),
  visibleCommandAllowlist,
);

for (const command of [
  "glade.runLocalProof",
  "glade.openHome",
  "glade.createProjectOrg",
  "glade.startProjectOrg",
  "glade.stopProjectOrg",
  "glade.projectOrgStatus",
  "glade.cloneEnvironment",
  "glade.deleteEnvironment",
  "glade.revealEnvironmentDb",
  "glade.inspectEnvironment",
  "glade.statusQuickPick",
	  "glade.openOutput",
	  "glade.replayDebugLog",
	  "glade.apexLog.refreshAnalysis",
	  "glade.apexLog.treatAsApexLog",
	  "glade.apexLog.replayFromFrame",
	  "glade.refreshPlugins",
  "glade.openTui",
  "glade.openTestsTui",
  "glade.openDataTui",
  "glade.openPluginsTui",
  "glade.managePlugins",
  "glade.runPluginAction",
  "glade.linkLocalPlugin",
  "glade.installPluginArchive",
  "glade.workbench.newAnonymousApex",
  "glade.workbench.newSoql",
  "glade.runSoql",
  "glade.workbench.runEntry",
  "glade.workbench.runLastSoql",
  "glade.workbench.describe",
  "glade.workbench.openResult",
  "glade.schemaImportDescribe",
  "glade.salesforceTargetStatus",
]) {
  assert(
    manifest.contributes.commands.some((entry) => entry.command === command),
    `${command} must be contributed`,
  );
}

assert(
  !manifest.contributes.commands.some((entry) => entry.command === "glade.workbench.runLastAnonymousApex"),
  "anonymous Apex must not use saved workbench snippets",
);
assert(
  !activationEvents.includes("onCommand:glade.workbench.runLastAnonymousApex"),
  "saved anonymous Apex runner must not activate the extension",
);

const anonymousScratchCommand = manifest.contributes.commands.find((entry) => entry.command === "glade.workbench.newAnonymousApex");
assert(anonymousScratchCommand, "anonymous Apex scratch command must be contributed");
assert.strictEqual(anonymousScratchCommand.title, "Glade: Open Anonymous Apex Scratch");

const soqlScratchCommand = manifest.contributes.commands.find((entry) => entry.command === "glade.workbench.newSoql");
assert(soqlScratchCommand, "SOQL scratch command must be contributed");
assert.strictEqual(soqlScratchCommand.title, "Glade: Open SOQL Scratch");

const soqlRunCommand = manifest.contributes.commands.find((entry) => entry.command === "glade.runSoql");
assert(soqlRunCommand, "SOQL run command must be contributed");
assert.strictEqual(soqlRunCommand.title, "Glade: Run Local SOQL");

const tuiCommands = new Map(manifest.contributes.commands.map((entry) => [entry.command, entry.title]));
assert.strictEqual(tuiCommands.get("glade.openTui"), "Glade: Open TUI");
assert.strictEqual(tuiCommands.get("glade.openTestsTui"), "Glade: Open Tests TUI");
assert.strictEqual(tuiCommands.get("glade.openDataTui"), "Glade: Open Data TUI");
assert.strictEqual(tuiCommands.get("glade.openPluginsTui"), "Glade: Open Plugins TUI");

const languages = manifest.contributes.languages || [];
assert(
  languages.some((entry) => entry.id === "soql" && entry.aliases.includes("SOQL") && entry.extensions.includes(".soql")),
  "SOQL scratch editors need a contributed language",
);
assert(
  languages.some((entry) =>
    entry.id === "apexlog"
    && entry.aliases.includes("Apex Log")
    && entry.extensions.includes(".apexlog")
    && entry.extensions.includes(".apex.log")
    && !entry.extensions.includes(".log")
    && /APEX_CODE/.test(entry.firstLine || "")
    && /NONE/.test(entry.firstLine || "")
  ),
  "debug logs need a contributed Apex Log language without claiming every .log file",
);
assert(
  (manifest.contributes.grammars || []).some((entry) => entry.language === "soql" && entry.scopeName === "source.soql"),
  "SOQL scratch editors need syntax highlighting",
);
assert(
  (manifest.contributes.grammars || []).some((entry) =>
    entry.language === "apexlog"
    && entry.scopeName === "source.apexlog"
    && entry.path === "./syntaxes/apexlog.tmLanguage.json"
  ),
  "Apex debug logs need syntax highlighting",
);
assert(fs.existsSync(path.join(root, "syntaxes", "apexlog.tmLanguage.json")), "Apex Log grammar asset must exist");

const editorTitle = manifest.contributes.menus["editor/title"] || [];
for (const command of ["glade.executeAnonymous", "glade.debugAnonymous"]) {
  const item = editorTitle.find((entry) => entry.command === command);
  assert(item, `${command} must be available from untitled Apex editors`);
  assert(
    item.when.includes("resourceScheme == untitled") && item.when.includes("editorLangId == apex"),
    `${command} must be scoped to untitled Apex editors`,
  );
}

const soqlRunTitle = editorTitle.find((entry) => entry.command === "glade.runSoql");
assert(soqlRunTitle, "SOQL scratch editors must expose a run action");
assert(
  soqlRunTitle.when.includes("resourceScheme == untitled") && soqlRunTitle.when.includes("editorLangId == soql"),
  "SOQL run action must be scoped to untitled SOQL editors",
);

const replayTitle = editorTitle.find((entry) => entry.command === "glade.replayDebugLog");
assert(replayTitle, "Apex Log editors must expose replay");
assert(
  replayTitle.when.includes("editorLangId == apexlog"),
  "Apex Log replay must be scoped to Apex Log editors",
);

const refreshApexLogTitle = editorTitle.find((entry) => entry.command === "glade.apexLog.refreshAnalysis");
assert(refreshApexLogTitle, "Apex Log editors must expose analysis refresh");
assert(
  refreshApexLogTitle.when.includes("editorLangId == apexlog"),
  "Apex Log refresh must be scoped to Apex Log editors",
);

const configProps = manifest.contributes.configuration.properties;
for (const key of [
  "glade.apexLog.smartFeatures.enabled",
  "glade.apexLog.maxAnalysisBytes",
  "glade.apexLog.codeLens.enabled",
]) {
  assert(configProps[key], `${key} must be contributed`);
}

assert(
  (manifest.contributes.keybindings || []).some((entry) =>
    entry.command === "glade.executeAnonymous"
    && entry.key === "ctrl+enter"
    && entry.mac === "cmd+enter"
    && entry.when.includes("resourceScheme == untitled")
    && entry.when.includes("editorLangId == apex")
  ),
  "anonymous Apex scratch editors must support Ctrl/Cmd+Enter execution",
);

assert(
  (manifest.contributes.keybindings || []).some((entry) =>
    entry.command === "glade.runSoql"
    && entry.key === "ctrl+enter"
    && entry.mac === "cmd+enter"
    && entry.when.includes("resourceScheme == untitled")
    && entry.when.includes("editorLangId == soql")
  ),
  "SOQL scratch editors must support Ctrl/Cmd+Enter execution",
);

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
