import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { test } from "node:test";
import vm from "node:vm";

const css = await readFile(new URL("../.vitepress/theme/custom.css", import.meta.url), "utf8");
const index = await readFile(new URL("../docs-src/index.md", import.meta.url), "utf8");
const theme = await readFile(new URL("../.vitepress/theme/index.ts", import.meta.url), "utf8");
const docsEnhancer = await readFile(new URL("../.vitepress/theme/DocsEnhancer.vue", import.meta.url), "utf8");
const codeMirrorWorkbench = await readFile(new URL("../.vitepress/theme/GladeEditorWorkbench.vue", import.meta.url), "utf8").catch(() => "");
const editorSupportTs = await readFile(new URL("../.vitepress/theme/generated/editorSupport.ts", import.meta.url), "utf8").catch(() => "");
const editorSupportJsonText = await readFile(new URL("../docs-src/public/data/editor-support.json", import.meta.url), "utf8").catch(() => "{}");
const editorSupportTypes = await readFile(new URL("../.vitepress/theme/editor/editorSupportTypes.ts", import.meta.url), "utf8").catch(() => "");
const apexLanguageModule = await readFile(new URL("../.vitepress/theme/editor/apexLanguage.ts", import.meta.url), "utf8").catch(() => "");
const apexCompletionsModule = await readFile(new URL("../.vitepress/theme/editor/apexCompletions.ts", import.meta.url), "utf8").catch(() => "");
const buildEditorSupportScript = await readFile(new URL("../scripts/build-editor-support.mjs", import.meta.url), "utf8").catch(() => "");
const checkDocRoutesScript = await readFile(new URL("../scripts/check-doc-routes.mjs", import.meta.url), "utf8").catch(() => "");
const config = await readFile(new URL("../.vitepress/config.ts", import.meta.url), "utf8");
const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
const siteReadme = await readFile(new URL("../README.md", import.meta.url), "utf8");
const pagesWorkflow = await readFile(new URL("../../.github/workflows/pages.yml", import.meta.url), "utf8").catch(() => "");
const ciWorkflow = await readFile(new URL("../../.github/workflows/ci.yml", import.meta.url), "utf8");
const releaseWorkflow = await readFile(new URL("../../.github/workflows/release.yml", import.meta.url), "utf8");
const securityWorkflow = await readFile(new URL("../../.github/workflows/security.yml", import.meta.url), "utf8").catch(() => "");
const agentGuide = await readFile(new URL("../../AGENTS.md", import.meta.url), "utf8");
const repoReadme = await readFile(new URL("../../README.md", import.meta.url), "utf8");
const repoSecurityPolicy = await readFile(new URL("../../SECURITY.md", import.meta.url), "utf8").catch(() => "");
const repoCompatibility = await readFile(new URL("../../docs/COMPATIBILITY.md", import.meta.url), "utf8");
const repoApexLanguageCompatibility = await readFile(new URL("../../docs/APEX_LANGUAGE_COMPATIBILITY.md", import.meta.url), "utf8").catch(() => "");
const repoDocsIndex = await readFile(new URL("../../docs/README.md", import.meta.url), "utf8");
const reservedIdentifierImplementation = await readFile(new URL("../../third_party/glade-apex-parser/reserved_identifier_words.go", import.meta.url), "utf8");
const repoCompatibilityDashboard = await readFile(new URL("../../docs/COMPATIBILITY_DASHBOARD.md", import.meta.url), "utf8");
const repoLwcSupport = await readFile(new URL("../../docs/LWC_SUPPORT.md", import.meta.url), "utf8");
const releaseNotes = await readFile(new URL("../../docs/RELEASE_NOTES.md", import.meta.url), "utf8");
const repoInstallDocs = await readFile(new URL("../../docs/INSTALL.md", import.meta.url), "utf8");
const repoLocalTesting = await readFile(new URL("../../docs/LOCAL_TESTING.md", import.meta.url), "utf8");
const repoPlugins = await readFile(new URL("../../docs/PLUGINS.md", import.meta.url), "utf8");
const repoDistributionWorkflow = await readFile(new URL("../../docs/DISTRIBUTION_WORKFLOW.md", import.meta.url), "utf8");
const storageSchema = await readFile(new URL("../../docs/storage-schema.md", import.meta.url), "utf8");
const highlight = await readFile(new URL("../docs-src/public/js/highlight.js", import.meta.url), "utf8");
const homeScript = await readFile(new URL("../docs-src/public/js/home.js", import.meta.url), "utf8");
const ciArtifacts = await readFile(new URL("../docs-src/guide/ci-artifacts.md", import.meta.url), "utf8");
const automation = await readFile(new URL("../docs-src/guide/automation.md", import.meta.url), "utf8");
const configuration = await readFile(new URL("../docs-src/guide/configuration.md", import.meta.url), "utf8");
const guideErrors = await readFile(new URL("../docs-src/guide/errors.md", import.meta.url), "utf8");
const installation = await readFile(new URL("../docs-src/guide/installation.md", import.meta.url), "utf8");
const overview = await readFile(new URL("../docs-src/guide/overview.md", import.meta.url), "utf8");
const securityTrust = await readFile(new URL("../docs-src/guide/security-trust.md", import.meta.url), "utf8").catch(() => "");
const quickstart = await readFile(new URL("../docs-src/guide/quickstart.md", import.meta.url), "utf8");
const cliReference = await readFile(new URL("../docs-src/guide/cli-reference.md", import.meta.url), "utf8");
const localTesting = await readFile(new URL("../docs-src/guide/local-testing.md", import.meta.url), "utf8");
const affectedTests = await readFile(new URL("../docs-src/guide/affected-tests.md", import.meta.url), "utf8");
const playground = await readFile(new URL("../docs-src/guide/playground.md", import.meta.url), "utf8");
const workflowsIndex = await readFile(new URL("../docs-src/guide/workflows.md", import.meta.url), "utf8").catch(() => "");
const workflowApexTests = await readFile(new URL("../docs-src/guide/workflows/apex-tests.md", import.meta.url), "utf8").catch(() => "");
const workflowDebugApex = await readFile(new URL("../docs-src/guide/workflows/debug-apex.md", import.meta.url), "utf8").catch(() => "");
const workflowLwcPreview = await readFile(new URL("../docs-src/guide/workflows/lwc-preview.md", import.meta.url), "utf8").catch(() => "");
const workflowVisualforcePreview = await readFile(new URL("../docs-src/guide/workflows/visualforce-preview.md", import.meta.url), "utf8").catch(() => "");
const workflowLocalData = await readFile(new URL("../docs-src/guide/workflows/local-data.md", import.meta.url), "utf8").catch(() => "");
const workflowCi = await readFile(new URL("../docs-src/guide/workflows/ci.md", import.meta.url), "utf8").catch(() => "");
const modulesIndex = await readFile(new URL("../docs-src/guide/modules.md", import.meta.url), "utf8").catch(() => "");
const moduleApexRuntime = await readFile(new URL("../docs-src/guide/modules/apex-runtime.md", import.meta.url), "utf8").catch(() => "");
const moduleTestRunner = await readFile(new URL("../docs-src/guide/modules/test-runner.md", import.meta.url), "utf8").catch(() => "");
const moduleLocalOrgData = await readFile(new URL("../docs-src/guide/modules/local-org-data.md", import.meta.url), "utf8").catch(() => "");
const moduleLwcPreview = await readFile(new URL("../docs-src/guide/modules/lwc-preview.md", import.meta.url), "utf8").catch(() => "");
const moduleVisualforcePreview = await readFile(new URL("../docs-src/guide/modules/visualforce-preview.md", import.meta.url), "utf8").catch(() => "");
const moduleDebugProfile = await readFile(new URL("../docs-src/guide/modules/debug-profile.md", import.meta.url), "utf8").catch(() => "");
const moduleEditor = await readFile(new URL("../docs-src/guide/modules/editor.md", import.meta.url), "utf8").catch(() => "");
const modulePlugins = await readFile(new URL("../docs-src/guide/modules/plugins.md", import.meta.url), "utf8").catch(() => "");
const referenceCli = await readFile(new URL("../docs-src/reference/cli.md", import.meta.url), "utf8").catch(() => "");
const referenceConfig = await readFile(new URL("../docs-src/reference/config.md", import.meta.url), "utf8").catch(() => "");
const referenceErrors = await readFile(new URL("../docs-src/reference/errors.md", import.meta.url), "utf8").catch(() => "");
const referenceApexLanguageCompatibility = await readFile(new URL("../docs-src/reference/apex-language-compatibility.md", import.meta.url), "utf8").catch(() => "");
const referenceApexSupport = await readFile(new URL("../docs-src/reference/apex-support.md", import.meta.url), "utf8").catch(() => "");
const referenceLwcSupport = await readFile(new URL("../docs-src/reference/lwc-support.md", import.meta.url), "utf8").catch(() => "");
const referenceVisualforceSupport = await readFile(new URL("../docs-src/reference/visualforce-support.md", import.meta.url), "utf8").catch(() => "");
const referenceLocalApiRoutes = await readFile(new URL("../docs-src/reference/local-api-routes.md", import.meta.url), "utf8").catch(() => "");
const workbench = await readFile(new URL("../docs-src/guide/workbench.md", import.meta.url), "utf8").catch(() => "");
const aiAssistedApex = await readFile(new URL("../docs-src/guide/ai-assisted-apex.md", import.meta.url), "utf8").catch(() => "");
const testerFieldGuide = await readFile(new URL("../docs-src/guide/tester-field-guide.md", import.meta.url), "utf8");
const editor = await readFile(new URL("../docs-src/guide/editor.md", import.meta.url), "utf8");
const supportMap = await readFile(new URL("../docs-src/guide/support-map.md", import.meta.url), "utf8");
const localApiServer = await readFile(new URL("../docs-src/guide/local-api-server.md", import.meta.url), "utf8");
const gladeOrgs = await readFile(new URL("../docs-src/guide/glade-orgs.md", import.meta.url), "utf8");
const lwcLocalShell = await readFile(new URL("../docs-src/guide/lwc-local-shell.md", import.meta.url), "utf8");
const enterpriseWorkflows = await readFile(new URL("../docs-src/guide/enterprise-workflows.md", import.meta.url), "utf8");
const plugins = await readFile(new URL("../docs-src/guide/plugins.md", import.meta.url), "utf8");
const firstPartyPlugins = await readFile(new URL("../docs-src/guide/plugins/first-party.md", import.meta.url), "utf8");
const pluginInstallManage = await readFile(new URL("../docs-src/guide/plugins/install-manage.md", import.meta.url), "utf8");
const pluginLockCi = await readFile(new URL("../docs-src/guide/plugins/lock-ci.md", import.meta.url), "utf8");
const maintainerIndex = await readFile(new URL("../docs-src/maintainer/index.md", import.meta.url), "utf8").catch(() => "");
const extendRuntime = await readFile(new URL("../docs-src/maintainer/extend-runtime.md", import.meta.url), "utf8").catch(() => "");
const gladeToolsMaintainer = await readFile(new URL("../docs-src/maintainer/glade-tools.md", import.meta.url), "utf8").catch(() => "");
const pluginRuntime = await readFile(new URL("../docs-src/maintainer/plugin-runtime.md", import.meta.url), "utf8").catch(() => "");
const helpIndex = await readFile(new URL("../docs-src/help/index.md", import.meta.url), "utf8").catch(() => "");
const helpFirstLocalCheck = await readFile(new URL("../docs-src/help/first-local-check.md", import.meta.url), "utf8").catch(() => "");
const helpRunOneApexTest = await readFile(new URL("../docs-src/help/run-one-apex-test.md", import.meta.url), "utf8").catch(() => "");
const helpDebugApexVsCode = await readFile(new URL("../docs-src/help/debug-apex-vscode.md", import.meta.url), "utf8").catch(() => "");
const helpAnonymousApexScratch = await readFile(new URL("../docs-src/help/anonymous-apex-scratch.md", import.meta.url), "utf8").catch(() => "");
const helpLocalDataEnvironments = await readFile(new URL("../docs-src/help/local-data-environments.md", import.meta.url), "utf8").catch(() => "");
const helpChangedTestsBeforePr = await readFile(new URL("../docs-src/help/changed-tests-before-pr.md", import.meta.url), "utf8").catch(() => "");
const helpGladeOrgSfDataImport = await readFile(new URL("../docs-src/help/glade-org-sf-data-import.md", import.meta.url), "utf8").catch(() => "");
const helpProfileApexDebugLog = await readFile(new URL("../docs-src/help/profile-apex-debug-log.md", import.meta.url), "utf8").catch(() => "");
const helpCiSetup = await readFile(new URL("../docs-src/help/ci-setup.md", import.meta.url), "utf8").catch(() => "");
const helpScreenshotReadme = await readFile(new URL("../docs-src/public/help/screenshots/README.md", import.meta.url), "utf8").catch(() => "");
const captureHelpScreenshotsScript = await readFile(new URL("../scripts/capture-help-screenshots.sh", import.meta.url), "utf8").catch(() => "");
const captureHelpScreenshotTargetScript = await readFile(new URL("../scripts/capture-help-screenshot-target.sh", import.meta.url), "utf8").catch(() => "");
const checkHelpScreenshotsScript = await readFile(new URL("../scripts/check-help-screenshots.mjs", import.meta.url), "utf8").catch(() => "");
const helpProjectSetupScript = await readFile(new URL("../scripts/help-project/setup.mjs", import.meta.url), "utf8").catch(() => "");
const logoMark = await readFile(new URL("../docs-src/public/logo-mark.svg", import.meta.url), "utf8");
const logoMarkOpen = await readFile(new URL("../docs-src/public/logo-mark-open.svg", import.meta.url), "utf8");
const socialCardPng = await readFile(new URL("../docs-src/public/social-card.png", import.meta.url)).catch(() => Buffer.alloc(0));

async function readSiteSourceFiles(relativeDir) {
  const dir = new URL(relativeDir, import.meta.url);
  const entries = await readdir(dir, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const child = new URL(`${relativeDir}${entry.name}${entry.isDirectory() ? "/" : ""}`, import.meta.url);
    if (entry.isDirectory()) return readSiteSourceFiles(`${relativeDir}${entry.name}/`);
    if (!/\.(md|ts|vue|js)$/.test(entry.name)) return [];
    return [[child.pathname, await readFile(child, "utf8")]];
  }));
  return files.flat();
}

const siteSourceFiles = await readSiteSourceFiles("../docs-src/");
const allPublicGuideText = siteSourceFiles
  .filter(([file]) => file.includes("/docs-src/guide/"))
  .map(([, contents]) => contents)
  .join("\n");
const siteCopy = [
  index,
  config,
  workbench,
  codeMirrorWorkbench,
  homeScript,
  ...siteSourceFiles.map(([, contents]) => contents)
].join("\n");

test("home page names the latest stable release from release notes", () => {
  const releaseMatch = releaseNotes.match(/^## (v\d+\.\d+\.\d+) - /m);
  assert.ok(releaseMatch, "release notes should name the latest release");
  const latestRelease = releaseMatch[1];

  assert.match(index, new RegExp(`<span class="home-release-version">${latestRelease}</span>`));
  assert.match(index, /Latest stable release/);
  assert.match(index, /Latest stable release:<span class="home-release-version">/);
  assert.doesNotMatch(index, /Latest stable release[\s\S]*0\.0\.0-dev/);
});

test("release notes cover the latest stable release", () => {
  assert.match(releaseNotes, /^## v0\.2\.9 - 2026-07-25/m);
  assert.match(releaseNotes, /11\.45% lower duration/);
  assert.match(releaseNotes, /11,526-test corpus/);
  assert.match(releaseNotes, /filesystem\/root confinement/);
  assert.match(releaseNotes, /Security and release trust/);
  assert.match(releaseNotes, /duplicate asset name fails instead of replacing published bytes/);
});

test("v0.2.9 docs describe the live registry and release safety", () => {
  for (const publicPage of [index, plugins, firstPartyPlugins, pluginInstallManage, pluginLockCi, testerFieldGuide]) {
    assert.doesNotMatch(publicPage, /registry (?:commands (?:are )?)?is (?:still )?preview|registry is not live yet|once the registry .* serves|until the registry is published/i);
  }
  assert.match(index, /The default public registry serves the first-party plugin catalog/);
  assert.match(firstPartyPlugins, /https:\/\/plugins\.glade\.sh\/index\.json/);
  assert.match(firstPartyPlugins, /Common command roots/);
  assert.match(firstPartyPlugins, /registry row are authoritative for[\s\S]*complete command-root list/);
  assert.match(pluginInstallManage, /Direct archives and local links remain available for offline, private, and development use/);
  assert.match(repoPlugins, /The default public registry is `https:\/\/plugins\.glade\.sh\/index\.json`/);
  assert.match(repoPlugins, /only regular files and directories/);
  assert.match(repoPlugins, /Representative command roots/);
  assert.match(repoPlugins, /all command roots derived from the packaged plugin\.json/);
  assert.match(repoPlugins, /links and special entries are\s+rejected/);
  assert.match(repoDistributionWorkflow, /conditional create/i);
  assert.match(repoDistributionWorkflow, /mutable pointers last/i);
  assert.doesNotMatch(repoDistributionWorkflow, /wrangler r2 object put glade-downloads\/vX\.Y\.Z\//);
});

test("v0.2.9 docs expose local performance artifacts without weakening Salesforce validation", () => {
  for (const testDocs of [repoLocalTesting, localTesting, cliReference]) {
    assert.match(testDocs, /--cpu-profile/);
    assert.match(testDocs, /--mem-profile/);
    assert.match(testDocs, /--perf-json/);
  }
  assert.match(localTesting, /do not replace Salesforce validation/i);
  assert.doesNotMatch(repoLocalTesting, /only the edited file is re-scanned, never the whole project/);
  assert.match(repoLocalTesting, /authoritative rebuild/i);
  assert.match(repoLocalTesting, /unsafe paths and symlink escapes are\s+rejected/i);
});

test("repo compatibility CLI surface matches contributor guide inventory", () => {
  const guideMatch = agentGuide.match(/Product CLI commands:\s*([\s\S]*?)\.\n/);
  assert.ok(guideMatch, "AGENTS.md should list product CLI commands");
  const compatibilityMatch = repoCompatibility.match(/\| CLI surface \| supported \| ([^|]+) exist\./);
  assert.ok(compatibilityMatch, "COMPATIBILITY.md should list the supported CLI surface");

  const commandsFromGuide = [...guideMatch[1].matchAll(/`([^`]+)`/g)].map((match) => match[1]);
  const commandsFromCompatibility = [...compatibilityMatch[1].matchAll(/`([^`]+)`/g)].map((match) => match[1]);
  assert.deepEqual(commandsFromCompatibility, commandsFromGuide);
});

test("theme defines complete light and dark color tokens", () => {
  assert.match(css, /html:not\(\.dark\)\s*\{[\s\S]*--vp-c-bg:/);
  assert.match(css, /\.dark\s*\{[\s\S]*--vp-c-bg:/);
  assert.match(css, /--glade-bg-grid-line:/);
  assert.match(css, /--glade-code-bg:/);
  assert.match(css, /--glade-nav-bg: var\(--bg\);/);
});

test("site is dark-only and does not render the appearance switch", () => {
  assert.match(config, /appearance: 'force-dark'/);
  assert.doesNotMatch(config, /appearance: 'dark'/);
  assert.doesNotMatch(css, /VPNavBarAppearance|VPSwitchAppearance/);
});

test("social share metadata exposes a raster preview card", () => {
  assert.equal(socialCardPng.toString("ascii", 1, 4), "PNG");
  assert.ok(socialCardPng.length > 10_000, "social card should be a real raster asset");
  assert.match(config, /\['meta', \{ property: 'og:url', content: 'https:\/\/glade\.sh\/' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:site_name', content: 'Glade' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:image', content: 'https:\/\/glade\.sh\/social-card\.png' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:image:secure_url', content: 'https:\/\/glade\.sh\/social-card\.png' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:image:type', content: 'image\/png' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:image:width', content: '1200' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:image:height', content: '630' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:image:alt', content: 'Glade local Apex runtime social preview' \}\]/);
  assert.match(config, /\['meta', \{ name: 'twitter:card', content: 'summary_large_image' \}\]/);
  assert.match(config, /\['meta', \{ name: 'twitter:title', content: 'Glade — Local Apex runtime for SFDX projects' \}\]/);
  assert.match(config, /\['meta', \{ name: 'twitter:description', content: 'Run supported Apex checks before the Salesforce round trip\.' \}\]/);
  assert.match(config, /\['meta', \{ name: 'twitter:image', content: 'https:\/\/glade\.sh\/social-card\.png' \}\]/);
  assert.match(config, /\['meta', \{ name: 'twitter:image:alt', content: 'Glade local Apex runtime social preview' \}\]/);
  const imageTags = config.match(/\['meta', \{ (?:property|name): '(?:og|twitter):image[^']*', content: '[^']+' \}\]/g) || [];
  assert.ok(imageTags.every((tag) => !tag.includes("logo-mark.svg")));
});

test("vite config keeps dev and preview tunnel output clean", () => {
  assert.match(config, /const tunnelAllowedHosts = \[/);
  assert.ok(config.includes("'.ngrok-free.app'"));
  assert.ok(config.includes("'.ngrok.app'"));
  assert.ok(config.includes("'.ngrok.io'"));
  assert.match(config, /server:\s*\{[\s\S]*allowedHosts: tunnelAllowedHosts/);
  assert.match(config, /preview:\s*\{[\s\S]*allowedHosts: tunnelAllowedHosts/);
  assert.match(config, /\['script', \{ defer: true, src: '\/js\/highlight\.js' \}\]/);
  assert.match(config, /\['script', \{ defer: true, src: '\/js\/home\.js' \}\]/);
  assert.match(config, /rollupOptions:\s*\{[\s\S]*onwarn\(warning, warn\)/);
  assert.match(config, /warning\.code === 'INVALID_ANNOTATION'/);
  assert.match(config, /@vueuse\/core/);
  assert.match(config, /#\__PURE__/);
});

test("home page keeps marketing icons out of the first proof", () => {
  assert.doesNotMatch(theme, /@lucide\/vue/);
  assert.equal((index.match(/class="home-feature-icon"/g) || []).length, 0);
  assert.equal((index.match(/<Icon(?:SearchCheck|FlaskConical|SquareTerminal|ServerCog|PlayCircle)\b/g) || []).length, 0);
  const numericEntities = index.match(/&#\d+;/g) || [];
  assert.deepEqual(numericEntities.filter((entity) => entity !== "&#10;"), []);
});

test("home page uses a static local proof and final go-live workflow copy", () => {
  assert.doesNotMatch(index, /name: "Glade"/);
  assert.doesNotMatch(index, /^hero:/m);
  assert.equal((index.match(/<h1>/g) || []).length, 1);
  assert.match(index, /<h1>Local Apex runtime for SFDX projects<\/h1>/);
  assert.match(index, /class="[^"]*\bhome-hero-shell\b[^"]*"/);
  assert.match(index, /class="home-type-eyebrow"[\s\S]*Run, test, and debug Apex locally/);
  assert.doesNotMatch(index, /class="home-lead"/);
  assert.match(index, /class="home-deck"[\s\S]*Run supported Apex checks, focused tests, SOQL\/DML, triggers, and anonymous Apex against local project state\. Debug supported paths from VS Code\. Check human and AI-generated changes before the org round trip\. Salesforce remains the validation gate\./);
  assert.match(index, /href="\/guide\/installation"[^>]*data-demo-link[^>]*>Install Glade<\/a>/);
  assert.match(index, /href="\/guide\/quickstart"[\s\S]*>Run your first local check<\/a>/);
  assert.match(index, /No Salesforce org login required for supported local checks\./);
  assert.doesNotMatch(index, /release gate|pre-gate|deploy gate|org gate/);
  const homeLoopStart = index.indexOf('<div class="home-loop-visual"');
  const homeLoopEnd = index.indexOf("</section>", homeLoopStart);
  assert.ok(homeLoopStart > -1);
  assert.ok(homeLoopEnd > homeLoopStart);
  const homeLoopBlock = index.slice(homeLoopStart, homeLoopEnd);
  assert.doesNotMatch(homeLoopBlock, /\n\s*\n/);
  assert.match(homeLoopBlock, /aria-label="Project loop command and diagnostic"/);
  assert.match(index, /<strong>Project loop<\/strong>/);
  assert.match(homeLoopBlock, /Local check/);
  assert.doesNotMatch(index, /AI edit check|Proof now/);
  assert.doesNotMatch(homeLoopBlock, /data-home-loop|home-loop-stage|home-loop-trace|home-loop-tabs|home-loop-badge-row/);
  assert.doesNotMatch(homeLoopBlock, /glade test|glade exec|2 passed|Quote total verified/);
  assert.match(homeLoopBlock, /class="home-loop-command"[\s\S]*glade check --project \. --no-progress/);
  assert.match(homeLoopBlock, /class="home-loop-result-status"[\s\S]*<span class="home-loop-mark warn" aria-hidden="true">!<\/span>[\s\S]*Diagnostic/);
  assert.match(homeLoopBlock, /Cannot resolve variable renewalQuote/);
  assert.match(homeLoopBlock, /RenewalQuoteService\.cls:42/);
  assert.match(index, /class="home-loop-mark warn" aria-hidden="true">!<\/span>/);
  assert.doesNotMatch(homeLoopBlock, /class="home-loop-mark" aria-hidden="true">✓<\/span>/);
  assert.doesNotMatch(homeLoopBlock, /class="home-loop-mark" aria-hidden="true">›<\/span>/);
  assert.match(index, /class="home-loop-metric-proof"><strong>0<\/strong>deploys<\/span>/);
  assert.doesNotMatch(index, /OpportunityRenewalServiceTest/);
  assert.doesNotMatch(index, /reprice-renewals\.apex/);
  assert.doesNotMatch(index, /rebuild\.apex/);
  assert.match(index, /aria-label="Daily local workflow"/);
  assert.match(index, /<p class="home-eyebrow">Daily local workflow<\/p>/);
  assert.match(index, /<h2 class="home-h2">One local loop for CLI, VS Code, AI, and CI\.<\/h2>/);
  assert.match(index, /class="home-command-grid"[\s\S]*<h3>CLI<\/h3>[\s\S]*Check source, run focused tests, execute anonymous Apex, and inspect SOQL\/DML behavior from your terminal\./);
  assert.match(index, /class="home-command-grid"[\s\S]*<h3>VS Code<\/h3>[\s\S]*Open Glade Home for local proof, data, debug, and ship actions\. Run local tests from Test Explorer and CodeLens\./);
  assert.match(index, /class="home-command-grid"[\s\S]*<h3>AI-assisted changes<\/h3>[\s\S]*Give agents a small local contract: run a check, quote the diagnostic, fix the smallest source change, and rerun the same command\./);
  assert.match(index, /class="home-command-grid"[\s\S]*<h3>CI<\/h3>[\s\S]*Use JSON, SARIF, JUnit, stable exit codes, affected-test selection, and saved run artifacts in pull request workflows\./);
  assert.doesNotMatch(index, /<h3>Check<\/h3>[\s\S]*glade check --project \. --no-progress/);
  assert.doesNotMatch(index, /<h3>Test<\/h3>[\s\S]*RefinementServiceTest/);
  assert.doesNotMatch(index, /<h3>Exec<\/h3>[\s\S]*check-renewal-total/);
  assert.doesNotMatch(index, /class="home-proof-strip"/);
  assert.doesNotMatch(index, /class="[^"]*\bhome-editor-demo\b[^"]*"/);
  assert.doesNotMatch(index, /data-editor-demo/);
  assert.doesNotMatch(index, /data-static-editor-code/);
  assert.doesNotMatch(index, /Autocomplete suggestions/);
  assert.doesNotMatch(index, /Apex autocomplete with local support labels/);
  assert.doesNotMatch(index, /data-apex-editor/);
  assert.doesNotMatch(index, /<textarea/);
  assert.doesNotMatch(index, /data-completion-menu/);
  assert.match(index, /class="home-support-preview"/);
  assert.match(index, /data-generated-support-preview/);
  assert.match(index, /<header class="home-support-preview-header">[\s\S]*<h3>What runs locally<\/h3>[\s\S]*<p>Examples from the checked capability map\.<\/p>[\s\S]*<\/header>/);
  assert.match(index, /class="home-capability-list"/);
  assert.equal((index.match(/class="home-capability-row"/g) || []).length, 3);
  assert.match(index, /Database\.insert/);
  assert.match(index, /Schema\.DescribeSObjectResult/);
  assert.match(index, /Answers\.findSimilar/);
  assert.doesNotMatch(index, /Capability preview/);
  assert.doesNotMatch(index, /Generated from checked Glade capability rows/);
  assert.match(index, /<a href="\/guide\/support-map">What runs locally<\/a>/);
  assert.doesNotMatch(index, /Run checks before the org round trip\./);
  assert.doesNotMatch(index, /Run Glade before the org round trip/);
  assert.doesNotMatch(index, /Edit Apex -&gt; glade check\/test locally/);
  assert.match(index, /Runs locally/);
  assert.match(index, /Runs with limits/);
  assert.match(index, /Requires Salesforce/);
  assert.match(index, /Check what runs locally before relying on it\./);
  assert.match(index, /Glade lists local behavior in three groups: runs locally, runs with limits, and requires Salesforce\./);
  assert.match(index, /Apex parse \+ semantic checks/);
  assert.match(index, /Local Salesforce API routes/);
  assert.match(index, /Visualforce and LWC local shells remain preview features\./);
  assert.match(index, /aria-label="Local data and playground"/);
  assert.match(index, /<h2 class="home-h2">Local data without a scratch org<\/h2>/);
  assert.match(index, /Run anonymous Apex, SOQL, DML, triggers, local API routes, and playground examples against local project state\./);
  assert.match(index, /glade playground --project \. --open/);
  assert.match(index, /glade server --project \. --db \.glade\/refinement-local\.sqlite --addr 127\.0\.0\.1:8080/);
  assert.match(index, /glade db seed --db \.glade\/refinement-local\.sqlite --project \. data\/file-rows\.json/);
  assert.match(index, /aria-label="Optional plugins"/);
  assert.match(index, /<h2 class="home-h2">Optional plugins<\/h2>/);
  assert.match(index, /The base runtime stays focused on local Apex workflows\. Add plugins only when a project needs capability reports, advisory scans, or custom local checks\./);
  assert.match(index, /Base Glade workflows do not require plugins\. The default public registry serves the first-party plugin catalog\./);
  assert.match(index, /glade plugins list/);
  assert.match(index, /glade plugins install @glade\/performance/);
  assert.match(index, /glade plugins install @glade\/orgpackage/);
  assert.match(index, /See first-party plugin install and lock-file docs\./);
  assert.doesNotMatch(index, /glade plugins link --exec \.\/glade-plugin-quality/);
  assert.doesNotMatch(index, /glade plugins install @glade\/compat/);
  assert.doesNotMatch(index, /authoring, and marketplace docs/);
  assert.match(index, /aria-label="Salesforce validation boundary"/);
  assert.match(index, /<h2 class="home-h2">Salesforce remains the validation gate\.<\/h2>/);
  assert.match(index, /Use Salesforce for live auth, hosted service engines, deploy and retrieve, exact Lightning Experience behavior, Streaming, Pub\/Sub, GraphQL, and exact production governor accounting\./);
  assert.doesNotMatch(index, /Developer loop and CI gate/);
  assert.doesNotMatch(index, /class="home-info-list"/);
  assert.match(index, /Supported paths run locally\. Unsupported platform services fail with named diagnostics\. Salesforce remains the validation gate\./);
  assert.match(index, /curl -fsSL https:\/\/glade\.sh\/install\.sh \| sh/);
  assert.match(index, /glade doctor/);
  assert.match(index, /glade check --project \./);
  assert.match(index, /aria-label="Copy install command"/);
  const workflowIndex = index.indexOf('aria-label="Daily local workflow"');
  const previewIndex = index.indexOf('data-generated-support-preview');
  const runsIndex = index.indexOf('aria-label="What runs locally"');
  const dataIndex = index.indexOf('aria-label="Local data and playground"');
  const pluginIndex = index.indexOf('aria-label="Optional plugins"');
  const boundaryIndex = index.indexOf('aria-label="Salesforce validation boundary"');
  assert.ok(workflowIndex > homeLoopEnd);
  assert.ok(previewIndex > workflowIndex);
  assert.ok(runsIndex > previewIndex);
  assert.ok(dataIndex > runsIndex);
  assert.ok(pluginIndex > dataIndex);
  assert.ok(boundaryIndex > pluginIndex);
  assert.doesNotMatch(index, /class="[^"]*\bhome-workbench\b[^"]*"/);
  assert.doesNotMatch(index, /data-scenario-workbench/);
  assert.doesNotMatch(index, /class="home-workflow-tab/);
  assert.doesNotMatch(index, /data-scenario-id=/);
  assert.doesNotMatch(index, /data-output-tab=/);
  assert.doesNotMatch(index, /home-support-map/);
  assert.doesNotMatch(index, /Capability[\s\S]*Local support[\s\S]*Boundary/);
  assert.doesNotMatch(index, /home-next-card/);
  assert.doesNotMatch(index, /Feature Arcade/);
  assert.doesNotMatch(index, /mock REST/i);
  assert.doesNotMatch(index, /0 diagnostics/);
  assert.match(config, /message: 'Glade is local-first Apex tooling\.'/);
  assert.match(config, /copyright: 'Released by the Glade project\.'/);
  assert.match(config, /\{ text: 'Playground', link: '\/guide\/playground' \}/);
  assert.match(config, /\{ text: 'What runs locally', link: '\/guide\/support-map' \}/);
  assert.match(config, /\{ text: 'Use VS Code', link: '\/guide\/editor' \}/);
  assert.match(config, /\{ text: 'sf target orgs', link: '\/guide\/glade-orgs' \}/);
  assert.match(config, /\{ text: 'What is Glade\?', link: '\/guide\/overview' \}/);
  assert.match(config, /\{ text: 'Install', link: '\/guide\/installation' \}/);
  assert.match(config, /\{ text: 'Tester field guide', link: '\/guide\/tester-field-guide' \}/);
  assert.match(config, /\{ text: 'AI-assisted Apex', link: '\/guide\/ai-assisted-apex' \}/);
  assert.match(config, /\{ text: 'Execute anonymous Apex and SOQL', link: '\/guide\/workbench' \}/);
  assert.doesNotMatch(config, /\{ text: 'Coverage', link: '\/guide\/workbench' \}/);
  assert.doesNotMatch(config, /\{ text: 'Capability map', link: '\/guide\/support-map' \}/);
});

test("docs navigation exposes workflows modules and references as separate trails", () => {
  function assertConfigLink(text, link) {
    const escapedText = text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const escapedLink = link.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    assert.match(config, new RegExp(`\\{ text: '${escapedText}', link: '${escapedLink}' \\}`));
  }

  assertConfigLink("Workflows", "/guide/workflows");
  assertConfigLink("Product areas", "/guide/modules");
  assertConfigLink("Reference", "/reference/cli");
  assertConfigLink("Run Apex tests", "/guide/workflows/apex-tests");
  assertConfigLink("Debug Apex", "/guide/workflows/debug-apex");
  assertConfigLink("Preview LWC", "/guide/workflows/lwc-preview");
  assertConfigLink("Preview Visualforce", "/guide/workflows/visualforce-preview");
  assertConfigLink("Work with local data", "/guide/workflows/local-data");
  assertConfigLink("Add Glade to CI", "/guide/workflows/ci");
  assertConfigLink("Apex runtime", "/guide/modules/apex-runtime");
  assertConfigLink("Test runner", "/guide/modules/test-runner");
  assertConfigLink("Local org and data", "/guide/modules/local-org-data");
  assertConfigLink("LWC preview", "/guide/modules/lwc-preview");
  assertConfigLink("Visualforce preview", "/guide/modules/visualforce-preview");
  assertConfigLink("Debug and profile", "/guide/modules/debug-profile");
  assertConfigLink("Editor and workbench", "/guide/modules/editor");
  assertConfigLink("Plugins", "/guide/modules/plugins");
  assertConfigLink("CLI reference", "/reference/cli");
  assertConfigLink("Config reference", "/reference/config");
  assertConfigLink("Error codes", "/reference/errors");
  assertConfigLink("Apex language compatibility", "/reference/apex-language-compatibility");
  assertConfigLink("Apex support map", "/reference/apex-support");
  assertConfigLink("LWC support matrix", "/reference/lwc-support");
  assertConfigLink("Visualforce support matrix", "/reference/visualforce-support");
  assertConfigLink("Local API routes", "/reference/local-api-routes");
});

test("new docs pages use clear page roles and link to deeper references", () => {
  assert.match(workflowsIndex, /^# Choose a Glade workflow/m);
  assert.match(workflowsIndex, /Run Apex tests/);
  assert.match(workflowsIndex, /Preview LWC locally/);
  assert.match(workflowsIndex, /Preview Visualforce locally/);

  assert.match(workflowApexTests, /^# Run Apex tests/m);
  assert.match(workflowApexTests, /glade test --project \./);
  assert.match(workflowApexTests, /\[Test runner\]\(\/guide\/modules\/test-runner\)/);
  assert.match(workflowDebugApex, /^# Debug Apex/m);
  assert.match(workflowDebugApex, /glade dap --project/);
  assert.match(workflowDebugApex, /\[Debug and profile\]\(\/guide\/modules\/debug-profile\)/);
  assert.match(workflowLwcPreview, /^# Preview LWC locally/m);
  assert.match(workflowLwcPreview, /glade dev lwc --project \. --open/);
  assert.match(workflowLwcPreview, /\[LWC support matrix\]\(\/reference\/lwc-support\)/);
  assert.match(workflowVisualforcePreview, /^# Preview Visualforce locally/m);
  assert.match(workflowVisualforcePreview, /glade dev vf --project \./);
  assert.match(workflowVisualforcePreview, /\[Visualforce support matrix\]\(\/reference\/visualforce-support\)/);
  assert.match(workflowLocalData, /^# Work with local data/m);
  assert.match(workflowCi, /^# Add Glade to CI/m);

  assert.match(modulesIndex, /^# Product areas/m);
  assert.match(moduleApexRuntime, /^# Apex runtime/m);
  assert.match(moduleTestRunner, /^# Test runner/m);
  assert.match(moduleLocalOrgData, /^# Local org and data/m);
  assert.match(moduleLwcPreview, /^# LWC preview/m);
  assert.match(moduleVisualforcePreview, /^# Visualforce preview/m);
  assert.match(moduleDebugProfile, /^# Debug and profile/m);
  assert.match(moduleEditor, /^# Editor and workbench/m);
  assert.match(modulePlugins, /^# Plugins/m);

  assert.match(referenceCli, /^# CLI reference/m);
  assert.match(referenceConfig, /^# Config reference/m);
  assert.match(referenceErrors, /^# Error codes/m);
  assert.match(referenceApexLanguageCompatibility, /^# Apex language compatibility/m);
  assert.match(referenceApexSupport, /^# Apex support map/m);
  assert.match(referenceLwcSupport, /^# LWC support matrix/m);
  assert.match(referenceVisualforceSupport, /^# Visualforce support matrix/m);
  assert.match(referenceLocalApiRoutes, /^# Local API routes/m);
});

test("Apex language compatibility docs publish the checked identifier contract", () => {
  const implementationMatch = reservedIdentifierImplementation.match(/salesforceReservedIdentifiers = wordSet\(`([\s\S]*?)`\)/);
  assert.ok(implementationMatch, "reserved identifier implementation should expose its checked word set");
  const implementationWords = implementationMatch[1].trim().toLowerCase().split(/\s+/);

  for (const docs of [repoApexLanguageCompatibility, referenceApexLanguageCompatibility]) {
    assert.match(docs, /121 Salesforce\s+reserved words/);
    assert.match(docs, /case-insensitive/i);
    assert.match(docs, /`currency`/);
    assert.match(docs, /method names/i);
    assert.match(docs, /`APEXPARSE002`/);
    assert.match(docs, /`APEXPARSE003`/);
    assert.match(docs, /`GLADESEMA_ANONYMOUS_PARSE`/);
    assert.match(docs, /Invalid identifier/);
    assert.match(docs, /400 checked language-rule rows/);
    assert.match(docs, /Salesforce remains the validation gate/);

    const documentedMatch = docs.match(/```text\n([\s\S]*?)\n```/);
    assert.ok(documentedMatch, "compatibility reference should publish the reserved word set");
    assert.deepEqual(documentedMatch[1].trim().toLowerCase().split(/\s+/), implementationWords);
  }

  assert.match(repoReadme, /\[Apex language compatibility\]\(docs\/APEX_LANGUAGE_COMPATIBILITY\.md\)/);
  assert.match(repoDocsIndex, /\[APEX_LANGUAGE_COMPATIBILITY\.md\]\(APEX_LANGUAGE_COMPATIBILITY\.md\)/);
  assert.match(repoCompatibility, /121 Salesforce reserved words/);
  assert.match(releaseNotes, /121 Salesforce reserved words/);
  assert.match(supportMap, /\[Apex language compatibility\]\(\/reference\/apex-language-compatibility\)/);
  assert.match(moduleApexRuntime, /\[Apex language compatibility\]\(\/reference\/apex-language-compatibility\)/);
  assert.match(referenceApexSupport, /\[Apex language compatibility\]\(\/reference\/apex-language-compatibility\)/);
  assert.match(guideErrors, /APEXPARSE002[\s\S]*not\s+currently accepted by `glade explain`/);
  assert.match(referenceErrors, /If it reports an unknown code, use the detailed guide/);
});

test("site copy is task-first and names local capabilities plainly", () => {
  assert.match(index, /<h1>Local Apex runtime for SFDX projects<\/h1>/);
  assert.match(index, /Run, test, and debug Apex locally/);
  assert.match(index, /Debug supported paths from VS Code/);
  assert.match(index, /Check human and AI-generated changes before the org round trip/);
  assert.match(index, /Unsupported platform services fail with named diagnostics\./);
  assert.match(index, /Run your first local check/);
  assert.match(index, /No Salesforce org login required for supported local checks\./);
  assert.match(index, /Salesforce remains the validation gate\./);
  assert.match(index, /What runs locally/);
  assert.match(index, /One local loop for CLI, VS Code, AI, and CI\./);
  assert.match(index, /Daily local workflow/);
  assert.match(index, /Optional plugins/);
  assert.match(index, /Local data without a scratch org/);
  assert.match(index, /Project loop/);
  assert.match(index, /Diagnostic/);
  for (const stale of [
    /AI edit check|Proof now/i,
    new RegExp("10" + "-minute", "i"),
    new RegExp("pi" + "lot", "i"),
    new RegExp("support " + "reports", "i"),
    new RegExp("coverage " + "report", "i"),
    new RegExp("marketplace " + "plugins", "i"),
    new RegExp("Local Apex Support " + "Show" + "case|Support " + "Show" + "case|" + "Show" + "case|Salesforce " + "Teams", "i"),
    new RegExp("<roo" + " t>|<Clas" + " s>|<Metho" + " d>|<active-d" + " b>", "i")
  ]) {
    assert.doesNotMatch(siteCopy, stale);
  }
  assert.match(index, /Anonymous Apex/);
  assert.match(workbench, /Type Apex expressions and see whether the API runs locally, runs with limits, or requires Salesforce\./);
  assert.match(workbench, /Use the editor as a live capability map: type a dot, read the label, and see the boundary before you depend on an API\./);
  assert.match(workbench, /Capability cards/);
  assert.match(workbench, /Workflow gallery/);
  assert.match(workbench, /Local result/);
  assert.match(codeMirrorWorkbench, /Try capability-backed autocomplete\./);
  assert.match(codeMirrorWorkbench, /Type a dot after/);
  assert.match(config, /description: 'Local Apex checks and focused tests before the Salesforce validation gate\.'/);
  assert.match(config, /\{ text: 'What runs locally', link: '\/guide\/support-map' \}/);
  assert.match(supportMap, /## Area details/);
  assert.match(supportMap, /## Capability claims/);
  assert.doesNotMatch(index, /View capability map/);
  assert.doesNotMatch(index, /Open the local coverage workbench/);
  assert.doesNotMatch(config, /text: 'Coverage'/);
  assert.doesNotMatch(config, /text: 'Capability map'/);
  assert.doesNotMatch(siteCopy, /release gate|pre-gate|deploy gate|org gate|See what runs locally/);
  assert.doesNotMatch(siteCopy, /Support showcase/i);
  assert.doesNotMatch(siteCopy, /support-aware/i);
  assert.doesNotMatch(siteCopy, /Salesforce-shaped/i);
  assert.doesNotMatch(siteCopy, /What Glade proved/i);
  assert.doesNotMatch(siteCopy, /Inspect what Glade proved/i);
  assert.doesNotMatch(siteCopy, /Runtime shape/i);
  assert.doesNotMatch(siteCopy, /First-layer status/i);
  assert.match(siteCopy, /packageShims/);
  assert.match(siteCopy, /glade plugins install @glade\/orgpackage/);
  assert.match(cliReference, /glade package capture --target-org packaging --namespace pkg/);
  assert.equal((cliReference.match(/auto-connect through `\.glade\/test\/serve\.sock` unless `--no-serve` is set\./g) || []).length, 1);
});

test("security trust surface is public, checked, and release-backed", () => {
  assert.match(repoReadme, /Security workflow/);
  assert.match(repoReadme, /https:\/\/github\.com\/glade-sh\/glade\/actions\/workflows\/security\.yml\/badge\.svg\?branch=main/);
  assert.match(repoReadme, /https:\/\/github\.com\/glade-sh\/glade\/actions\/workflows\/security\.yml/);
  assert.doesNotMatch(repoReadme, /api\.scorecard\.dev/);
  assert.doesNotMatch(repoReadme, /scorecard\.dev\/viewer/);
  assert.match(repoReadme, /\[Security & Trust\]\(docs\/SECURITY_TRUST\.md\)/);
  assert.match(repoReadme, /\[Security policy\]\(SECURITY\.md\)/);

  assert.match(repoSecurityPolicy, /^# Security Policy/m);
  assert.match(repoSecurityPolicy, /Supported versions/);
  assert.match(repoSecurityPolicy, /Report a vulnerability/);
  assert.match(repoSecurityPolicy, /Local laptop behavior/);
  assert.match(repoSecurityPolicy, /Glade does not require a Salesforce org login for supported local checks\./);

  assert.match(config, /\{ text: 'Security & Trust', link: '\/guide\/security-trust' \}/);
  assert.match(securityTrust, /^# Security & Trust/m);
  assert.match(securityTrust, /OpenSSF Scorecard/);
  assert.match(securityTrust, /after the repository is public/);
  assert.match(securityTrust, /govulncheck/);
  assert.match(securityTrust, /CodeQL/);
  assert.match(securityTrust, /gosec/);
  assert.match(securityTrust, /CycloneDX SBOM/);
  assert.match(securityTrust, /Artifact attestations/);
  assert.match(securityTrust, /gh attestation verify/);
  assert.match(securityTrust, /SHA256SUMS\.txt/);
  assert.match(securityTrust, /Laptop behavior/);
  assert.match(securityTrust, /Network access/);
  assert.match(securityTrust, /Local storage/);

  assert.match(securityWorkflow, /name: Security/);
  assert.match(securityWorkflow, /golang\.org\/x\/vuln\/cmd\/govulncheck@v1\.6\.0/);
  assert.match(securityWorkflow, /github\/codeql-action\/init@[0-9a-f]{40}/);
  assert.match(securityWorkflow, /- uses: security-extended/);
  assert.match(securityWorkflow, /timeout-minutes: 5/);
  assert.match(securityWorkflow, /- go\/allocation-size-overflow/);
  assert.match(securityWorkflow, /- go\/incorrect-integer-conversion/);
  assert.match(securityWorkflow, /github\/codeql-action\/analyze@[0-9a-f]{40}/);
  assert.match(securityWorkflow, /securego\/gosec@/);
  assert.match(securityWorkflow, /upload-sarif/);
  assert.match(securityWorkflow, /npm audit --omit=dev --audit-level=high/);
  assert.match(securityWorkflow, /working-directory: third_party\/lwc/);
  assert.match(securityWorkflow, /working-directory: contrib\/vscode-glade/);
  assert.match(securityWorkflow, /actions\/dependency-review-action@[0-9a-f]{40}/);
  assert.match(securityWorkflow, /ossf\/scorecard-action@[0-9a-f]{40}/);
  assert.match(securityWorkflow, /publish_results: true/);

  assert.match(ciWorkflow, /go-version: "1\.26\.5"/);
  assert.match(releaseWorkflow, /go-version: "1\.26\.5"/);
  assert.match(releaseWorkflow, /cyclonedx-gomod/);
  assert.match(releaseWorkflow, /tar -xzf "\$archive" -C "\$extract_dir" glade/);
  assert.match(releaseWorkflow, /cyclonedx-gomod bin -json -version "\$VERSION" -output "\$sbom" "\$extract_dir\/glade"/);
  assert.match(releaseWorkflow, /glade_.*\.sbom\.json/);
  assert.match(releaseWorkflow, /actions\/attest@[0-9a-f]{40}/);
  assert.match(releaseWorkflow, /attestations: write/);

  assert.match(repoInstallDocs, /Security verification/);
  assert.match(repoInstallDocs, /gh attestation verify/);
  assert.match(installation, /Security verification/);
  assert.match(installation, /gh attestation verify/);
});

test("guided help articles are a small screenshot-backed set", () => {
  const articles = [
    helpFirstLocalCheck,
    helpRunOneApexTest,
    helpDebugApexVsCode,
    helpAnonymousApexScratch,
    helpLocalDataEnvironments,
    helpChangedTestsBeforePr,
    helpGladeOrgSfDataImport,
    helpProfileApexDebugLog,
    helpCiSetup
  ];

  assert.match(config, /\{ text: 'Help', link: '\/help\/' \}/);
  assert.match(config, /text: 'Guided help'/);
  assert.match(helpIndex, /^# Guided Help/m);
  assert.equal(articles.length, 9);

  for (const article of articles) {
    assert.match(article, /class="docs-intro"/);
    assert.match(article, /## Before you start/);
    assert.match(article, /## Common wrong turn/);
    assert.match(article, /## Next/);
    assert.match(article, /!\[[^\]]+\]\(\/help\/screenshots\/[a-z0-9-]+\.png\)/);
  }
  const helpArticleCopy = articles.join("\n");
  assert.doesNotMatch(helpArticleCopy, /RefinementService|RefinementServiceTest|macrodata-apex|refine-local|insert account|opensFile/);
  assert.doesNotMatch(helpArticleCopy, /Walkthrough refinement|Quarterly refinement|Imported refinement file|generated help fixture|fixture/);
  assert.doesNotMatch(helpArticleCopy, /data\/insertOrder\.json|data\/accounts\.json|data\/contacts\.json|dev\.sqlite/);

  assert.match(helpRunOneApexTest, /Debug from Test Explorer/);
  assert.match(helpRunOneApexTest, /Variables/);
  assert.match(helpLocalDataEnvironments, /Open the Glade side view/);
  assert.doesNotMatch(helpLocalDataEnvironments, /Open the Glade Activity Bar/);
  assert.match(helpIndex, /Profile an Apex debug log/);
  assert.match(helpIndex, /Add Glade to CI/);
  assert.match(config, /Profile a debug log/);
  assert.match(config, /CI setup/);
  assert.match(helpProfileApexDebugLog, /glade debug profile --log <debug-log> --format markdown/);
  assert.match(helpProfileApexDebugLog, /glade debug profile --log <debug-log> --json/);
  assert.match(helpProfileApexDebugLog, /glade profile analyze/);
  assert.match(helpCiSetup, /fetch-depth: 0/);
  assert.match(helpCiSetup, /glade check --project \. --format sarif --output reports\/glade-check\.sarif/);
  assert.match(helpCiSetup, /glade test changed --project \. --since origin\/main --json --no-progress/);
  assert.match(helpCiSetup, /glade test --project \. --junit reports\/glade-junit\.xml --no-progress/);
  assert.match(helpChangedTestsBeforePr, /glade test --project \. --junit reports\/glade-junit\.xml --no-progress/);
  assert.match(helpChangedTestsBeforePr, /\[Add Glade to CI\]\(\/help\/ci-setup\)/);
  assert.match(helpGladeOrgSfDataImport, /glade org create <local-target> --project \./);
  assert.match(helpCiSetup, /Expected: the report files exist and have non-zero size\./);
  assert.match(helpCiSetup, /actions\/upload-artifact@v4/);
});

test("guided help screenshot capture uses terminal copy and clean VS Code profiles", () => {
  const publicHelpCopy = [
    helpIndex,
    helpFirstLocalCheck,
    helpRunOneApexTest,
    helpDebugApexVsCode,
    helpAnonymousApexScratch,
    helpLocalDataEnvironments,
    helpChangedTestsBeforePr,
    helpGladeOrgSfDataImport,
    helpProfileApexDebugLog,
    helpCiSetup,
    helpScreenshotReadme
  ].join("\n");

  assert.doesNotMatch(publicHelpCopy, /Ghostty|ghostty/);
  assert.doesNotMatch(publicHelpCopy, /\/Users\/matt|apollo|matt@/i);
  assert.match(captureHelpScreenshotsScript, /--user-data-dir/);
  assert.match(captureHelpScreenshotsScript, /--extensions-dir/);
  assert.match(captureHelpScreenshotsScript, /VSCODE_PROFILE_ROOT="\$\{TMPDIR:-\/tmp\}\/glade-help-vscode"/);
  assert.match(captureHelpScreenshotsScript, /--new-window/);
  assert.match(captureHelpScreenshotsScript, /vscode-glade-\$?\{?npm_package_version|vscode-glade-\*\.vsix|vscode-glade-\.\*\.vsix/);
  assert.match(captureHelpScreenshotsScript, /salesforce/);
  assert.match(captureHelpScreenshotsScript, /code --list-extensions/);
  assert.match(captureHelpScreenshotsScript, /RefinementServiceTest\.cls/);
  assert.match(captureHelpScreenshotsScript, /VSCODE_WINDOW_ZOOM="\$\{VSCODE_WINDOW_ZOOM:-1\.15\}"/);
  assert.match(captureHelpScreenshotsScript, /VSCODE_CAPTURE_WIDTH="\$\{VSCODE_CAPTURE_WIDTH:-1100\}"/);
  assert.match(captureHelpScreenshotsScript, /VSCODE_CAPTURE_HEIGHT="\$\{VSCODE_CAPTURE_HEIGHT:-750\}"/);
  assert.match(captureHelpScreenshotsScript, /CATPPUCCIN_EXTENSION_ID="\$\{CATPPUCCIN_EXTENSION_ID:-catppuccin\.catppuccin-vsc\}"/);
  assert.match(captureHelpScreenshotsScript, /SALESFORCE_APEX_EXTENSION_ID="\$\{SALESFORCE_APEX_EXTENSION_ID:-salesforce\.salesforcedx-vscode-apex\}"/);
  assert.match(captureHelpScreenshotsScript, /vscode_open_command=\(\n  env\n  HOME="\$SF_CAPTURE_HOME"\n  SF_USE_GENERIC_UNIX_KEYCHAIN=true\n  SFDX_USE_GENERIC_UNIX_KEYCHAIN=true\n  SF_DISABLE_TELEMETRY=true\n  SFDX_DISABLE_TELEMETRY=true\n  SF_SKIP_NEW_VERSION_CHECK=true\n  code/);
  assert.match(captureHelpScreenshotsScript, /--password-store=basic/);
  assert.match(captureHelpScreenshotsScript, /--use-mock-keychain/);
  assert.match(captureHelpScreenshotsScript, /--skip-welcome/);
  assert.match(captureHelpScreenshotTargetScript, /--skip-welcome/);
  assert.match(captureHelpScreenshotTargetScript, /open_vscode_location "\$file" 1 1/);
  assert.doesNotMatch(captureHelpScreenshotsScript, /--disable-extension salesforce\.salesforcedx-vscode-core/);
  assert.doesNotMatch(captureHelpScreenshotsScript, /--disable-extension salesforce\.salesforcedx-vscode-services/);
  assert.match(captureHelpScreenshotsScript, /--install-extension "\$CATPPUCCIN_EXTENSION_ID"/);
  assert.match(captureHelpScreenshotsScript, /--install-extension "\$SALESFORCE_APEX_EXTENSION_ID"/);
  assert.match(captureHelpScreenshotsScript, /Only Glade, Catppuccin, Salesforce Apex, its Salesforce dependencies/);
  assert.match(captureHelpScreenshotsScript, /"workbench\.colorTheme": "Catppuccin Mocha"/);
  assert.match(captureHelpScreenshotsScript, /"workbench\.sideBar\.location": "left"/);
  assert.match(captureHelpScreenshotsScript, /"workbench\.activityBar\.location": "hidden"/);
  assert.match(captureHelpScreenshotsScript, /"workbench\.secondarySideBar\.defaultVisibility": "hidden"/);
  assert.match(captureHelpScreenshotsScript, /"window\.zoomLevel": 1\.15/);
  assert.match(captureHelpScreenshotsScript, /"editor\.fontSize": 16/);
  assert.match(captureHelpScreenshotsScript, /"editor\.minimap\.enabled": false/);
  assert.match(captureHelpScreenshotsScript, /"debug\.showInStatusBar": "never"/);
  assert.match(captureHelpScreenshotsScript, /"debug\.autoExpandLazyVariables": "on"/);
  assert.match(captureHelpScreenshotsScript, /"salesforcedx-vscode-core\.telemetry\.enabled": false/);
  assert.match(captureHelpScreenshotsScript, /"salesforcedx-vscode-core\.show-cli-success-msg": false/);
  assert.match(captureHelpScreenshotsScript, /"salesforcedx-vscode-apex\.advanced\.enable-completion-statistics": false/);
  assert.match(captureHelpScreenshotsScript, /"salesforcedx-vscode-apex\.enable-apex-ls-error-to-telemetry": false/);
  assert.match(captureHelpScreenshotsScript, /"\*\.apex": "apex"/);
  assert.match(captureHelpScreenshotsScript, /GHOSTTY_FONT_SIZE="\$\{GHOSTTY_FONT_SIZE:-18\}"/);
  assert.match(captureHelpScreenshotsScript, /--font-size="\$GHOSTTY_FONT_SIZE"/);
  assert.match(captureHelpScreenshotsScript, /--window-show-tab-bar=never/);
  assert.match(captureHelpScreenshotsScript, /OPEN_HELP_CAPTURE_APPS/);
  assert.match(captureHelpScreenshotsScript, /SF_CAPTURE_HOME="\$PROJECT_ROOT\/\.glade\/sf-home"/);
  assert.match(captureHelpScreenshotsScript, /SF_USE_GENERIC_UNIX_KEYCHAIN=true/);
  assert.match(captureHelpScreenshotsScript, /SFDX_USE_GENERIC_UNIX_KEYCHAIN=true/);
  assert.match(captureHelpScreenshotsScript, /SF_DISABLE_TELEMETRY=true/);
  assert.match(captureHelpScreenshotsScript, /SFDX_DISABLE_TELEMETRY=true/);
  assert.match(captureHelpScreenshotsScript, /SF_SKIP_NEW_VERSION_CHECK=true/);
  assert.match(captureHelpScreenshotsScript, /workbench\.welcomePage\.walkthroughs\.openOnInstall/);
  assert.match(captureHelpScreenshotsScript, /chat\.allowAnonymousAccess/);
  assert.match(captureHelpScreenshotsScript, /chat\.titleBar\.signIn\.enabled/);
  assert.match(captureHelpScreenshotsScript, /git\.openRepositoryInParentFolders/);
  assert.match(captureHelpScreenshotsScript, /"git\.enabled": false/);
  assert.match(captureHelpScreenshotsScript, /"extensions\.ignoreRecommendations": true/);
  assert.match(captureHelpScreenshotsScript, /"update\.showReleaseNotes": false/);
  assert.match(captureHelpScreenshotsScript, /--disable-extension github\.copilot-chat/);
  assert.match(captureHelpScreenshotsScript, /--disable-extension vscode\.git/);
  assert.match(captureHelpScreenshotsScript, /--disable-extension vscode\.github-authentication/);
  assert.match(captureHelpScreenshotsScript, /screencapture/);
  assert.match(captureHelpScreenshotsScript, /Use a terminal for CLI screenshots/);
  assert.match(captureHelpScreenshotsScript, /Open terminal when ready/);
  assert.match(captureHelpScreenshotsScript, /open -na Ghostty\.app/);
  assert.match(captureHelpScreenshotsScript, /--title="?Glade Help CLI Capture"?/);
  assert.match(captureHelpScreenshotTargetScript, /terminal_done_file/);
  assert.match(captureHelpScreenshotTargetScript, /terminal_ready_file/);
  assert.match(captureHelpScreenshotTargetScript, /wait_for_terminal_exit/);
  assert.match(captureHelpScreenshotTargetScript, /wait_for_terminal_ready/);
  assert.match(captureHelpScreenshotTargetScript, /TERMINAL_SETTLE_SECONDS="\$\{TERMINAL_SETTLE_SECONDS:-2\}"/);
  assert.match(captureHelpScreenshotTargetScript, /position_process_window "Ghostty" "\$terminal_pid" "\$rect"[\s\S]*sleep "\$TERMINAL_SETTLE_SECONDS"/);
  assert.match(captureHelpScreenshotTargetScript, /touch "\$done_file"/);
  assert.match(captureHelpScreenshotTargetScript, /printf 'touch %q\\n' "\$ready_file"/);
  assert.doesNotMatch(captureHelpScreenshotTargetScript, /sleep 600/);
  assert.match(checkHelpScreenshotsScript, /help\/screenshots/);
  assert.match(checkHelpScreenshotsScript, /local-data-environments-02-terminal\.png/);
  assert.doesNotMatch(checkHelpScreenshotsScript, /local-data-environments-02-ghostty/);
  assert.match(checkHelpScreenshotsScript, /minWidth/);
  assert.match(checkHelpScreenshotsScript, /minHeight/);
  assert.match(checkHelpScreenshotsScript, /maxWidth/);
  assert.match(checkHelpScreenshotsScript, /maxHeight/);
  assert.match(helpProjectSetupScript, /glade\.yml/);
  assert.match(helpProjectSetupScript, /packageDirs: \[force-app\]/);
  assert.match(helpProjectSetupScript, /macrodata-apex/);
  assert.match(helpProjectSetupScript, /RefinementServiceTest/);
  assert.match(helpProjectSetupScript, /Walkthrough refinement file/);

  assert.match(helpFirstLocalCheck, /terminal/);
  assert.match(helpChangedTestsBeforePr, /terminal/);
  assert.match(helpDebugApexVsCode, /clean VS Code profile/);
  assert.match(helpDebugApexVsCode, /Open an Apex class or test file/);
  assert.match(helpAnonymousApexScratch, /clean VS Code profile/);
  assert.match(helpScreenshotReadme, /salesforce\.salesforcedx-vscode-apex/);
  assert.match(helpScreenshotReadme, /Salesforce Core and Services dependencies/);
  assert.match(helpScreenshotReadme, /Do not disable Salesforce Core or Services/);
  assert.match(helpRunOneApexTest, /Salesforce Apex extension/);
  assert.match(helpRunOneApexTest, /glade test --project \. --class <TestClass> --no-progress/);
  assert.match(helpRunOneApexTest, /Set a breakpoint on the line you want to inspect before starting the debug action/);
  assert.doesNotMatch(helpRunOneApexTest, /RefinementService|insert account|opensFile|macrodata-apex/);
  assert.doesNotMatch(helpRunOneApexTest, /RefinementServiceTest --json/);
  assert.match(captureHelpScreenshotTargetScript, /run-one-apex-test-01-cli\)[\s\S]*RefinementServiceTest --no-progress/);
  assert.doesNotMatch(captureHelpScreenshotTargetScript, /RefinementServiceTest --json/);
  assert.match(helpAnonymousApexScratch, /Salesforce Apex extension/);
  assert.match(helpLocalDataEnvironments, /Salesforce Apex extension/);
  assert.match(helpGladeOrgSfDataImport, /terminal/);
  assert.match(helpGladeOrgSfDataImport, /disposable Salesforce CLI home/);
  assert.match(helpGladeOrgSfDataImport, /SF_USE_GENERIC_UNIX_KEYCHAIN=true/);
  assert.match(helpGladeOrgSfDataImport, /SF_SKIP_NEW_VERSION_CHECK=true/);
  assert.match(helpGladeOrgSfDataImport, /sf org list/);
  assert.match(helpGladeOrgSfDataImport, /<local-target>/);
  assert.match(helpGladeOrgSfDataImport, /sf data import tree --plan <plan-file> --target-org <local-target>/);
  assert.match(helpGladeOrgSfDataImport, /sf data query --query "SELECT Id, Name FROM Account" --target-org <local-target>/);
  assert.doesNotMatch(helpGladeOrgSfDataImport, /--sf-config-dir/);
  assert.doesNotMatch(helpGladeOrgSfDataImport, / -o refine-local/);
  assert.match(helpGladeOrgSfDataImport, /sf data import tree/);
  assert.doesNotMatch(captureHelpScreenshotTargetScript, /ls -lh reports/);
  assert.doesNotMatch(helpScreenshotReadme, /ls -lh reports/);
  assert.match(captureHelpScreenshotTargetScript, /mark_changed_apex_file\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /changed-tests-before-pr-01-changed-tests\)[\s\S]*changed-apex/);
  assert.match(captureHelpScreenshotTargetScript, /changed-tests-before-pr-02-reports\)[\s\S]*changed-apex/);
  assert.match(captureHelpScreenshotTargetScript, /glade-org-sf-data-import-02-auth-list\)[\s\S]*reuse-project/);
  assert.match(captureHelpScreenshotTargetScript, /glade-org-sf-data-import-03-import-query\)[\s\S]*reuse-project/);
  assert.match(captureHelpScreenshotTargetScript, /run_terminal_with_server\(\)[\s\S]*live/);
  assert.match(captureHelpScreenshotTargetScript, /stop_local_org_server\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /cleanup_help_screenshot_capture\(\)[\s\S]*stop_local_org_server/);
  assert.match(captureHelpScreenshotTargetScript, /glade-org-sf-data-import-02-auth-list\)[\s\S]*TERMINAL_WIDE_RECT/);
  assert.match(captureHelpScreenshotTargetScript, /glade-org-sf-data-import-03-import-query\)[\s\S]*TERMINAL_WIDE_RECT/);
  assert.match(captureHelpScreenshotTargetScript, /glade org auth refine-local --project \. >\/dev\/null/);
  assert.match(captureHelpScreenshotTargetScript, /ensure_local_org_server\(\)[\s\S]*org\.json/);
  assert.match(captureHelpScreenshotTargetScript, /pkill -f "glade org start refine-local --project \$PROJECT_ROOT"/);
  assert.match(captureHelpScreenshotTargetScript, /close_bottom_panel\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /run_vscode\(\)[\s\S]*close_bottom_panel/);
  assert.match(captureHelpScreenshotTargetScript, /reset_vscode_capture_state\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /show_explorer_view\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /show_run_and_debug_view\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /set_vscode_breakpoint_from_menu\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /open_glade_sidebar\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /dismiss_vscode_first_run_prompts\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /notifications\.clearAll/);
  assert.match(captureHelpScreenshotTargetScript, /workbench\.debug\.action\.focusVariablesView/);
  assert.match(captureHelpScreenshotTargetScript, /focus_debug_variables_view\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /clear_vscode_launch_config\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /close_vscode_capture_windows\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /pkill -f "\$VSCODE_USER"/);
  assert.match(captureHelpScreenshotTargetScript, /pkill -9 -f "\$VSCODE_USER"/);
  assert.match(captureHelpScreenshotTargetScript, /cleanup_help_screenshot_capture\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /trap cleanup_help_screenshot_capture EXIT/);
  assert.match(captureHelpScreenshotTargetScript, /open_vscode_file\(\)[\s\S]*close_vscode_capture_windows[\s\S]*env \\/);
  assert.match(captureHelpScreenshotTargetScript, /open_vscode_file\(\)[\s\S]*dismiss_vscode_first_run_prompts/);
  assert.match(captureHelpScreenshotTargetScript, /open_vscode_location\(\)[\s\S]*dismiss_vscode_first_run_prompts/);
  assert.match(captureHelpScreenshotTargetScript, /run_vscode\(\)[\s\S]*reset_vscode_capture_state/);
  assert.match(captureHelpScreenshotTargetScript, /run_vscode\(\)[\s\S]*clear_vscode_launch_config/);
  assert.match(captureHelpScreenshotTargetScript, /run_vscode\(\)[\s\S]*capture_rect "\$target" "\$VSCODE_RECT"[\s\S]*close_vscode_capture_windows/);
  const vscodeActionBlock = (name) => {
    const match = captureHelpScreenshotTargetScript.match(new RegExp(`\\n    ${name}\\)\\n([\\s\\S]*?)\\n      ;;`));
    assert.ok(match, `missing VS Code capture action: ${name}`);
    return match[1];
  };
  assert.doesNotMatch(vscodeActionBlock("breakpoint"), /click_relative/);
  assert.doesNotMatch(vscodeActionBlock("debug-toolbar"), /click_relative 103 242|key_code 96/);
  assert.doesNotMatch(vscodeActionBlock("glade-data"), /click_relative 51 450/);
  assert.match(captureHelpScreenshotTargetScript, /wc -c reports\/glade-test-changed\.json reports\/glade-junit\.xml/);
  assert.match(helpScreenshotReadme, /wc -c reports\/glade-test-changed\.json reports\/glade-junit\.xml/);
  assert.match(helpChangedTestsBeforePr, /wc -c reports\/glade-test-changed\.json reports\/glade-junit\.xml/);
  assert.match(helpProfileApexDebugLog, /terminal/);
  assert.match(helpProfileApexDebugLog, /Hot events/);
  assert.match(helpCiSetup, /terminal/);
  assert.match(helpCiSetup, /reports\/glade-check\.sarif/);
  assert.match(captureHelpScreenshotTargetScript, /profile-apex-debug-log-01-profile\)[\s\S]*glade debug profile --log reports\/anonymous-output\.txt --format markdown/);
  assert.match(captureHelpScreenshotTargetScript, /profile-apex-debug-log-02-json\)[\s\S]*glade debug profile --log reports\/anonymous-output\.txt --json/);
  assert.match(captureHelpScreenshotTargetScript, /ci-setup-01-workflow\)[\s\S]*write_ci_workflow/);
  assert.match(captureHelpScreenshotTargetScript, /ci-setup-02-artifacts\)[\s\S]*glade-check\.sarif/);
});

test("guided help screenshot runner names every capture target", () => {
  const screenshotNames = [
    "first-local-check-01-doctor.png",
    "first-local-check-02-check.png",
    "run-one-apex-test-01-cli.png",
    "run-one-apex-test-02-codelens.png",
    "run-one-apex-test-03-test-explorer.png",
    "debug-apex-vscode-01-breakpoint.png",
    "debug-apex-vscode-02-debug-toolbar.png",
    "debug-apex-vscode-03-variables.png",
    "anonymous-apex-scratch-01-buffer.png",
    "anonymous-apex-scratch-02-run.png",
    "local-data-environments-01-sidebar.png",
    "local-data-environments-02-terminal.png",
    "changed-tests-before-pr-01-changed-tests.png",
    "changed-tests-before-pr-02-reports.png",
    "glade-org-sf-data-import-01-create-start.png",
    "glade-org-sf-data-import-02-auth-list.png",
    "glade-org-sf-data-import-03-import-query.png",
    "profile-apex-debug-log-01-profile.png",
    "profile-apex-debug-log-02-json.png",
    "ci-setup-01-workflow.png",
    "ci-setup-02-artifacts.png"
  ];

  assert.match(packageJson.scripts["help:screenshot"], /scripts\/capture-help-screenshot-target\.sh/);
  assert.match(captureHelpScreenshotTargetScript, /--list/);
  assert.match(captureHelpScreenshotTargetScript, /--all/);
  assert.match(captureHelpScreenshotTargetScript, /run_terminal/);
  assert.match(captureHelpScreenshotTargetScript, /run_vscode/);
  assert.match(captureHelpScreenshotTargetScript, /find_ghostty_pid_for_command/);
  assert.match(captureHelpScreenshotTargetScript, /position_process_window "Ghostty"/);
  assert.match(captureHelpScreenshotTargetScript, /CAPTURE_XDG_DATA_HOME/);
  assert.match(captureHelpScreenshotTargetScript, /debug-breakpoint/);
  assert.match(captureHelpScreenshotTargetScript, /menu item "Start Debugging"/);
  assert.match(captureHelpScreenshotTargetScript, /run-one-apex-test-03-test-explorer\)[\s\S]*debug-breakpoint/);
  assert.match(captureHelpScreenshotTargetScript, /expand_debug_locals\(\)/);
  assert.match(captureHelpScreenshotTargetScript, /expand_debug_locals\(\)[\s\S]*focus_debug_variables_view[\s\S]*key_code 124[\s\S]*key_code 125[\s\S]*key_code 124/);
  assert.match(captureHelpScreenshotTargetScript, /debug-breakpoint\)[\s\S]*start_debugging_from_menu[\s\S]*expand_debug_locals/);
  assert.match(captureHelpScreenshotTargetScript, /breakpoint\)[\s\S]*open_vscode_location "\$file" 4 1[\s\S]*set_vscode_breakpoint_from_menu/);
  assert.match(captureHelpScreenshotTargetScript, /debug-apex-vscode-01-breakpoint\)[\s\S]*RefinementService\.cls" breakpoint/);
  assert.doesNotMatch(captureHelpScreenshotTargetScript, /tell application "Visual Studio Code"/);
  assert.doesNotMatch(captureHelpScreenshotTargetScript, /tell application "\$app_name"/);
  assert.match(captureHelpScreenshotTargetScript, /screencapture/);
  assert.match(captureHelpScreenshotTargetScript, /PROMPT='macrodata-apex % '/);
  assert.doesNotMatch(captureHelpScreenshotTargetScript, /apollo/i);

  for (const name of screenshotNames) {
    const target = name.replace(/\.png$/, "");
    assert.match(captureHelpScreenshotTargetScript, new RegExp(`${target}\\)`));
    assert.match(helpScreenshotReadme, new RegExp(`npm --prefix site run help:screenshot -- ${target}`));
  }
});

test("editor support catalog is generated from checked Glade support data", () => {
  const editorSupportJson = JSON.parse(editorSupportJsonText || "{}");

  assert.match(buildEditorSupportScript, /docs\/STDLIB_COVERAGE\.md/);
  assert.match(buildEditorSupportScript, /--check/);
  assert.match(editorSupportTs, /export const editorSupportCatalog/);
  assert.equal(editorSupportJson.schemaVersion, 1);
  assert.equal(editorSupportJson.generatedFrom, "docs/STDLIB_COVERAGE.md");

  const database = editorSupportJson.receivers?.Database?.items || [];
  assert.ok(database.some((item) => item.label === "insert" && item.status === "supported"));
  assert.ok(database.some((item) => item.label === "rollback" && item.status === "supported"));
  assert.ok(database.some((item) => item.label === "setSavepoint" && item.status === "supported"));

  const answers = editorSupportJson.receivers?.Answers?.items || [];
  assert.ok(answers.some((item) => item.label === "findSimilar" && item.status === "unsupported"));

  const describe = editorSupportJson.receivers?.["Schema.DescribeSObjectResult"]?.items || [];
  assert.ok(describe.some((item) => item.label === "getChildRelationships" && item.status === "supported"));
  assert.ok(describe.some((item) => item.label === "fields.getMap" && item.status === "partial"));
});

test("workbench imports generated editor support instead of owning a static catalog", () => {
  assert.match(codeMirrorWorkbench, /editorSupportCatalog/);
  assert.match(codeMirrorWorkbench, /createApexCompletions/);
  assert.match(codeMirrorWorkbench, /apexLanguage/);
  assert.match(editorSupportTypes, /export type EditorSupportStatus/);
  assert.match(editorSupportTypes, /signature\?: string/);
  assert.match(editorSupportTypes, /signatures\?: readonly string\[\]/);
  assert.match(editorSupportTs, /import type \{ EditorSupportCatalog \} from '\.\.\/editor\/editorSupportTypes'/);
  assert.match(editorSupportTs, /satisfies EditorSupportCatalog/);
  assert.match(apexLanguageModule, /export const apexLanguage = StreamLanguage\.define/);
  assert.match(apexLanguageModule, /export const gladeHighlight = HighlightStyle\.define/);
  assert.match(apexCompletionsModule, /export function createApexCompletions/);
  assert.match(apexCompletionsModule, /export function maybeOpenReceiverCompletion/);
  assert.doesNotMatch(codeMirrorWorkbench, /const completionCatalog: Record/);
  assert.doesNotMatch(codeMirrorWorkbench, /const rootCompletions: Completion\[\]/);
  assert.doesNotMatch(codeMirrorWorkbench, /const DEMO_RECEIVER_TYPES/);
});

test("workbench completion info exposes support status labels", () => {
  assert.match(apexCompletionsModule, /function completionInfo/);
  assert.match(apexCompletionsModule, /glade-completion-info/);
  assert.match(apexCompletionsModule, /glade-completion-status-\$\{completion\.status\}/);
  assert.match(css, /\.glade-completion-info\s*\{/);
  assert.match(css, /\.glade-completion-status-supported\s*\{/);
  assert.match(css, /\.glade-completion-status-partial,/);
  assert.match(css, /\.glade-completion-status-unsupported\s*\{/);
});

test("home script powers the local workbench demo", () => {
  assert.doesNotMatch(index, /src: \/js\/highlight\.js/);
  assert.doesNotMatch(index, /src: \/js\/home\.js/);
  assert.doesNotMatch(workbench, /src: \/js\/highlight\.js/);
  assert.doesNotMatch(workbench, /src: \/js\/home\.js/);
  assert.match(homeScript, /initCopyControls\(\)[\s\S]*initWorkbenchDemo\(\)/);
  assert.match(homeScript, /window\.gladeInitHomeDemos = init/);
  assert.match(homeScript, /window\.addEventListener\("glade:content-updated", init\)/);
  assert.match(homeScript, /copyControlsInitialized/);
  assert.match(homeScript, /homeControlsInitialized/);
  assert.match(homeScript, /function initCopyControls\(\)/);
  assert.match(homeScript, /document\.addEventListener\("click"/);
  assert.ok(homeScript.includes('target.closest("[data-copy-target]")'));
  assert.doesNotMatch(homeScript, /initCompletionDemo/);
  assert.doesNotMatch(homeScript, /data-apex-editor/);
  assert.doesNotMatch(homeScript, /data-completion-menu/);
  assert.doesNotMatch(homeScript, /completionSurfaces/);
  assert.doesNotMatch(homeScript, /initHomeLoop|setHomeLoopState|animateHomeLoop|easeHomeLoop/);
  assert.doesNotMatch(homeScript, /data-home-loop|homeLoopStates|homeLoopLabels|homeLoopActiveNodes/);
  assert.doesNotMatch(homeScript, /requestAnimationFrame|manualHomeLoopUntil/);
  assert.match(homeScript, /hideHomeSearch/);
  assert.match(homeScript, /copyLabel \+ " copied to clipboard"/);
  assert.match(homeScript, /function initWorkbenchDemo\(\)/);
  assert.match(homeScript, /var scenarios = \{/);
  assert.match(homeScript, /data-scenario-id/);
  assert.match(homeScript, /runActiveScenario/);
  assert.match(homeScript, /data-output-tab/);
  assert.match(homeScript, /data-workflow-count/);
  assert.match(homeScript, /glade debug profile --log logs\/apex-debug\.log/);
});

test("home local loop styles stay quiet and responsive", () => {
  assert.match(highlight, /\.join\(""\)/);
  assert.match(highlight, /window\.gladeHighlightCodeBlock = highlightCodeBlock/);
  assert.match(highlight, /window\.gladeHighlightAllCodeBlocks = highlightAllCodeBlocks/);
  assert.match(highlight, /window\.addEventListener\("glade:content-updated", highlightAllCodeBlocks\)/);
  assert.match(highlight, /dataset\.gladeHighlightSource/);
  assert.match(highlight, /APEX_ANNOTATIONS/);
  assert.match(highlight, /APEX_ANNOTATION_ATTRIBUTES/);
  assert.match(highlight, /calendar_month/);
  assert.match(highlight, /method-declaration/);
  assert.match(css, /--text-hero: clamp\(2\.85rem, 5\.2vw, 4\.1rem\);/);
  assert.match(css, /--space-1: 4px;/);
  assert.match(css, /--space-5: 20px;/);
  assert.match(css, /--space-14: 56px;/);
  assert.match(css, /--space-16: 64px;/);
  assert.match(css, /--space-20: 80px;/);
  assert.match(css, /--radius-card: 16px;/);
  assert.match(css, /--radius-panel: 12px;/);
  assert.match(css, /:where\(a, button, \[tabindex\]\):focus-visible\s*\{[\s\S]*outline: 2px solid var\(--glade-focus-ring\);[\s\S]*outline-offset: 3px;/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*grid-template-columns: minmax\(500px, 1fr\) minmax\(360px, 560px\);/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*gap: var\(--space-14\);/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*padding: 42px 0 20px;/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*width: var\(--glade-shell-width\);/);
  assert.match(css, /\.vp-doc \.home-cta,[\s\S]*text-decoration-line: none !important;/);
  assert.match(css, /\.vp-doc \.home-cta\.primary:hover,\s*\n\.vp-doc \.home-cta\.primary:focus-visible\s*\{[\s\S]*background: var\(--glade-strong\);[\s\S]*color: var\(--text-inverse\) !important;/);
  assert.match(css, /\.home-hero-shell h1\s*\{[\s\S]*font-size: var\(--text-hero\);[\s\S]*letter-spacing: var\(--heading-track\);[\s\S]*line-height: 0\.96;/);
  assert.doesNotMatch(css, /\.home-proof-strip\s*\{/);
  assert.match(css, /\.vp-doc \.home-h2\s*\{[\s\S]*margin: 0;[\s\S]*border-top: 0;[\s\S]*padding-top: 0;/);
  assert.match(css, /\.home-loop-visual\s*\{[\s\S]*max-width: 560px;[\s\S]*min-height: 300px;[\s\S]*max-height: 380px;[\s\S]*padding: 20px;[\s\S]*overflow: hidden;/);
  assert.match(css, /\.home-loop-command\s*\{[\s\S]*margin-top: 16px;[\s\S]*padding: 14px 16px;[\s\S]*border-radius: 12px;/);
  assert.match(css, /\.home-loop-command code\s*\{[\s\S]*overflow-wrap: anywhere;/);
  assert.match(css, /\.home-loop-result\s*\{[\s\S]*margin-top: 12px;[\s\S]*gap: 8px;[\s\S]*padding: 16px;[\s\S]*border: 1px solid var\(--line\);/);
  assert.match(css, /\.home-loop-result-status\s*\{[\s\S]*grid-template-columns: 42px minmax\(0, 1fr\);[\s\S]*gap: 12px;/);
  assert.match(css, /\.home-loop-state-label\s*\{[\s\S]*font-size: 12px;[\s\S]*text-transform: uppercase;/);
  assert.match(css, /\.home-loop-metrics\s*\{[\s\S]*margin-top: 16px;[\s\S]*padding-top: 14px;[\s\S]*border-top: 1px solid var\(--line\);/);
  assert.doesNotMatch(css, /\.home-loop-stage|\.home-loop-trace|@keyframes home-loop-trace/);
  assert.doesNotMatch(css, /\.home-loop-tabs|\.home-loop-badge-row|\.home-loop-terminal|\.home-loop-state\s*\{/);
  assert.match(css, /\.home-command-grid\s*\{[\s\S]*grid-template-columns: repeat\(4, minmax\(0, 1fr\)\);/);
  assert.match(css, /\.home-command-card\s*\{[\s\S]*padding: 20px;[\s\S]*border-radius: 12px;/);
  assert.match(css, /\.home-command-card code\s*\{[\s\S]*overflow-wrap: anywhere;/);
  assert.match(css, /\.home-command-block pre\s*\{[\s\S]*margin: 0;[\s\S]*overflow-x: auto;[\s\S]*padding: 16px;/);
  assert.match(css, /\.home-command-block code\s*\{[\s\S]*font-family: var\(--vp-font-family-mono\);/);
  assert.match(css, /@media \(any-pointer: coarse\)\s*\{[\s\S]*\.home-cta,[\s\S]*\.home-support-preview a,[\s\S]*\.home-install-strip button\s*\{[\s\S]*min-height: 44px;/);
  assert.doesNotMatch(css, /\.home-editor-demo/);
  assert.doesNotMatch(css, /\.home-static-editor/);
  assert.doesNotMatch(css, /\.home-static-completion/);
  assert.doesNotMatch(css, /\.home-apex-textarea/);
  assert.doesNotMatch(css, /\.home-completion-menu/);
  assert.match(css, /\.home-capability-line\s*\{[\s\S]*display: flex;/);
  assert.match(css, /\.home-support-preview\s*\{/);
  assert.match(css, /\.home-support-preview-header\s*\{[\s\S]*display: grid;[\s\S]*gap: 8px;/);
  assert.match(css, /\.home-capability-list\s*\{[\s\S]*display: grid;[\s\S]*gap: 10px;[\s\S]*margin-top: 4px;/);
  assert.match(css, /\.home-capability-row\s*\{[\s\S]*grid-template-columns: minmax\(0, 1fr\) auto;[\s\S]*gap: 16px;[\s\S]*min-height: 36px;[\s\S]*padding: 8px 0;/);
  assert.match(css, /\.home-completion-status-supported\s*\{/);
  assert.match(css, /\.home-completion-status-limited\s*\{/);
  assert.match(css, /\.home-completion-status-salesforce\s*\{/);
  assert.match(css, /\.home-install-strip code\s*\{[\s\S]*overflow-wrap: anywhere;/);
  assert.match(css, /body:has\(\.VPHome\) \.VPNavBarSearch\s*\{[\s\S]*display: none;[\s\S]*pointer-events: none;/);
  assert.match(css, /\.VPNavBarSearch\s*\{[\s\S]*opacity: 0\.92;/);
  assert.match(css, /--glade-action-bg: var\(--glade\);/);
  assert.match(css, /@media \(max-width: 1120px\)\s*\{[\s\S]*\.home-hero-shell\s*\{[\s\S]*grid-template-columns: 1fr;[\s\S]*gap: 24px;[\s\S]*padding-bottom: 12px;/);
  assert.match(css, /@media \(max-width: 1120px\)\s*\{[\s\S]*\.home-command-grid\s*\{[\s\S]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-hero-shell,[\s\S]*\.home-support-preview,[\s\S]*\.home-capability-section,[\s\S]*\.home-install-strip\s*\{[\s\S]*width: 100%;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-hero-shell\s*\{[\s\S]*gap: 12px;[\s\S]*padding-top: 20px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.vp-doc \.home-type-eyebrow\s*\{[\s\S]*margin-bottom: 10px;[\s\S]*font-size: 12px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-hero-shell h1\s*\{[\s\S]*max-width: 320px;[\s\S]*font-size: 31px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.vp-doc \.home-deck\s*\{[\s\S]*margin-top: 10px;[\s\S]*font-size: 15px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.vp-doc \.home-local-line\s*\{[\s\S]*margin-top: 8px;[\s\S]*font-size: 14px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-loop-visual\s*\{[\s\S]*min-height: 0;[\s\S]*max-height: none;[\s\S]*padding: 12px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-loop-command\s*\{[\s\S]*margin-top: 12px;[\s\S]*padding: 12px 14px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-loop-result\s*\{[\s\S]*margin-top: 10px;[\s\S]*padding: 12px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-command-grid\s*\{[\s\S]*grid-template-columns: 1fr;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-loop-metrics\s*\{[\s\S]*grid-template-columns: repeat\(3, minmax\(0, 1fr\)\);[\s\S]*margin-top: 12px;[\s\S]*padding-top: 10px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-loop-metrics span\s*\{[\s\S]*font-size: 10px;[\s\S]*padding: 5px;/);
  assert.doesNotMatch(css, /margin-inline: calc\(var\(--glade-page-gutter\) \* -0\.5\);/);
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)\s*\{[\s\S]*transition-duration: 0\.001ms !important;/);
  assert.match(css, /\.workbench-page \.home-workbench\s*\{[\s\S]*width: 100%;[\s\S]*max-width: 100%;/);
  assert.match(css, /\.home-workflow-tabs\s*\{/);
  assert.match(css, /\.home-output-tabs\s*\{/);
  assert.doesNotMatch(css, /\.home-support-map table\s*\{/);
  assert.doesNotMatch(css, /\.home-next-card\s*\{/);
  assert.doesNotMatch(css, /--glade-instrument-bg/);
});

test("home mobile sections do not let command samples widen the page", () => {
  assert.match(css, /\.home-capability-section > \*\s*\{[\s\S]*min-width: 0;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.home-command-block code\s*\{[\s\S]*white-space: pre-wrap;[\s\S]*overflow-wrap: anywhere;/);
});

test("apex highlighter calls out platform and qualified types", () => {
  const context = {
    document: { querySelectorAll: () => [] },
    window: {}
  };
  vm.runInNewContext(`${highlight}\nwindow.__gladeHighlightApex = highlightApex;`, context);

  const output = context.window.__gladeHighlightApex([
    "Account account = new Account();",
    "Database.SaveResult[] results = Database.insert(accounts, false);",
    "Schema.DescribeSObjectResult describe = Account.SObjectType.getDescribe();"
  ].join("\n"));

  assert.match(output, /token class-name platform-type">Account<\/span>/);
  assert.match(output, /token class-name platform-type">Database<\/span>/);
  assert.match(output, /token class-name platform-type">SaveResult<\/span>/);
  assert.match(output, /token class-name platform-type">Schema<\/span>/);
  assert.match(output, /token class-name platform-type">DescribeSObjectResult<\/span>/);
  assert.match(highlight, /const PLATFORM_TYPES = new Set/);
  assert.match(css, /--glade-syntax-type: var\(--warning-text\);/);
  assert.match(css, /\.token\.platform-type\s*\{/);
  assert.match(apexLanguageModule, /const PLATFORM_TYPES = new Set/);
  assert.match(apexLanguageModule, /state\.lastText === '\.' && nextChar === '\('/);
  assert.match(apexLanguageModule, /tags\.standard\(tags\.variableName\)/);
  assert.match(apexLanguageModule, /style = 'builtin'/);
  assert.doesNotMatch(apexLanguageModule, /function\(variableName\)/);
});

test("interactive capability map carries autocomplete and workflow examples", () => {
  assert.match(workbench, /^# Interactive capability map/m);
  assert.match(workbench, /data-coverage-workbench/);
  assert.match(workbench, /class="coverage-workbench-cards"/);
  assert.match(workbench, /Database\.insert/);
  assert.match(workbench, /BusinessHours\.nextStartDate/);
  assert.match(workbench, /Answers\.findSimilar/);
  assert.match(workbench, /Requires Salesforce/);
  assert.match(workbench, /<GladeEditorWorkbench \/>/);
  assert.match(workbench, /class="[^"]*\bworkbench-page\b[^"]*"/);
  assert.match(workbench, /class="[^"]*\bhome-workbench\b[^"]*"/);
  assert.match(workbench, /id="local-apex-workbench"/);
  assert.match(workbench, /data-scenario-workbench/);
  assert.match(workbench, /aria-label="Local capability workflow demo"/);
  assert.match(workbench, /<p class="home-eyebrow">Workflow gallery<\/p>/);
  assert.match(workbench, /Run a scenario to see the command, JSON, trace, local result, and copyable CLI form\./);
  assert.match(workbench, /data-scenario-id="check"/);
  assert.match(workbench, /data-scenario-id="test"/);
  assert.match(workbench, /data-scenario-id="exec"/);
  assert.match(workbench, /data-scenario-id="debug"/);
  assert.match(workbench, /data-output-tab="output"/);
  assert.match(workbench, /data-output-tab="json"/);
  assert.match(workbench, /data-output-tab="trace"/);
  assert.match(workbench, /data-command-output/);
  assert.match(workbench, /data-cli-output/);
  assert.match(workbench, /aria-label="Copy workbench JSON command"/);
  assert.match(workbench, /Copy JSON command/);
  assert.match(workbench, /Shortcuts: 1-4 switch jobs · R run · C copy/);
  assert.match(homeScript, /function highlightCommandOutput/);
  assert.match(homeScript, /function highlightJsonOutput/);
  assert.match(homeScript, /function highlightTraceOutput/);
  assert.match(homeScript, /output\.innerHTML = highlightCommandOutput/);
  assert.match(css, /\.cli-token\.cli-command\s*\{/);
  assert.match(css, /\.cli-token\.cli-error\s*\{/);
  assert.match(css, /\.cli-token\.cli-json-key\s*\{/);
  assert.match(css, /\.cli-token\.cli-trace-event\s*\{/);
  assert.doesNotMatch(config, /text: 'Coverage', link: '\/guide\/workbench'/);
  assert.doesNotMatch(workbench, /^# Local Apex Workbench/m);
  assert.doesNotMatch(workbench, /Command workbench/);
  assert.doesNotMatch(workbench, /Support showcase/i);
  assert.doesNotMatch(workbench, /Local coverage workbench/);
  assert.doesNotMatch(workbench, /Coverage cards/);
});

test("workbench page mounts a real CodeMirror editor", () => {
  assert.match(theme, /defineAsyncComponent/);
  assert.match(theme, /app\.component\('GladeEditorWorkbench', defineAsyncComponent\(\(\) => import\('\.\/GladeEditorWorkbench\.vue'\)\)\)/);
  assert.match(codeMirrorWorkbench, /from 'codemirror'/);
  assert.match(codeMirrorWorkbench, /@codemirror\/autocomplete/);
  assert.match(codeMirrorWorkbench, /editorSupportCatalog/);
  assert.match(codeMirrorWorkbench, /apexLanguage/);
  assert.match(codeMirrorWorkbench, /gladeHighlight/);
  assert.match(codeMirrorWorkbench, /createApexCompletions/);
  assert.doesNotMatch(codeMirrorWorkbench, /@codemirror\/lang-java/);
  assert.doesNotMatch(codeMirrorWorkbench, /StreamLanguage/);
  assert.doesNotMatch(codeMirrorWorkbench, /const apexLanguage = StreamLanguage\.define/);
  assert.doesNotMatch(codeMirrorWorkbench, /const APEX_ANNOTATIONS/);
  assert.match(apexLanguageModule, /APEX_ANNOTATIONS/);
  assert.match(apexCompletionsModule, /Database\.SaveResult\[\]/);
  assert.match(apexCompletionsModule, /Schema\.DescribeSObjectResult/);
  assert.match(apexCompletionsModule, /function inferReceiverType/);
  assert.match(apexCompletionsModule, /function apexCompletions/);
  assert.match(apexCompletionsModule, /function maybeOpenReceiverCompletion/);
  assert.match(codeMirrorWorkbench, /autocompletion\(\{[\s\S]*override: \[apexCompletions\],[\s\S]*activateOnTyping: false/);
  assert.doesNotMatch(codeMirrorWorkbench, /activateOnTyping: true/);
  assert.match(codeMirrorWorkbench, /EditorView\.updateListener\.of\(\(update\) => \{/);
  assert.match(codeMirrorWorkbench, /doc: startDoc/);
  assert.match(codeMirrorWorkbench, /public static String rebuild\(Id businessHoursId\)/);
  assert.match(codeMirrorWorkbench, /new Account\(Name = 'Acme', BillingCity = 'Twin Lakes'\)/);
  assert.match(codeMirrorWorkbench, /Savepoint marker = Database\.setSavepoint\(\)/);
  assert.match(codeMirrorWorkbench, /Database\.rollback\(marker\)/);
  assert.match(codeMirrorWorkbench, /JSON\.serialize\(results\[0\]\.getErrors\(\)\)/);
  assert.match(codeMirrorWorkbench, /BusinessHours\.nextStartDate/);
  assert.match(codeMirrorWorkbench, /\n    describe\n  \}\n\}`/);
  assert.doesNotMatch(codeMirrorWorkbench, /\n    describe\.\n  \}\n\}`/);
  assert.doesNotMatch(codeMirrorWorkbench, /EditorView\.scrollIntoView/);
  assert.doesNotMatch(codeMirrorWorkbench, /selection:\s*\{\s*anchor:\s*cursor\s*\}/);
  assert.doesNotMatch(codeMirrorWorkbench, /const cursor = startDoc\.indexOf\('describe\.'\)/);
  assert.match(apexCompletionsModule, /startCompletion/);
  assert.match(apexCompletionsModule, /if \(currentView\(\) === view\) startCompletion\(view\)/);
  assert.doesNotMatch(codeMirrorWorkbench, /window\.setTimeout\(\(\) => \{[\s\S]*startCompletion\(editorView\)/);
  assert.match(editorSupportTs, /getRecordTypeInfosByDeveloperName[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /isSuccess[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /insert[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /setSavepoint[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /SObjectField[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /Schema\.DescribeSObjectResult\.fields/);
  assert.match(editorSupportTs, /Map<String, Schema\.SObjectField>/);
  assert.match(editorSupportTs, /getDescribe[\s\S]*Field metadata[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /"fieldMap": "Map<String, Schema\.SObjectField>"/);
  assert.match(apexCompletionsModule, /function indexedReceiverType/);
  assert.match(apexCompletionsModule, /catalog\.demoReceivers\[variableName\]/);
  assert.match(editorSupportTs, /getDmlRows[\s\S]*Runs locally/);
  assert.match(editorSupportTs, /findSimilar[\s\S]*Requires Salesforce/);
  assert.doesNotMatch(codeMirrorWorkbench, /Answers\.findSimilar/);
  assert.match(codeMirrorWorkbench, /class="glade-cm-support"/);
  assert.match(codeMirrorWorkbench, /Type a dot after the final describe, Account, Database, BusinessHours, Schema, describe\.fields, results\[0\], or fieldMap\./);
  assert.match(codeMirrorWorkbench, /<code>describe\.<\/code>/);
  assert.match(codeMirrorWorkbench, /<code>BusinessHours\.<\/code>/);
  assert.match(codeMirrorWorkbench, /data-codemirror-workbench/);
  assert.match(codeMirrorWorkbench, /<span><strong>Boundary labels<\/strong> Runs locally, Runs with limits, Requires Salesforce<\/span>/);
  assert.match(codeMirrorWorkbench, /borderLeftColor: '#9be870'/);
  assert.match(codeMirrorWorkbench, /caretColor: '#f3f7f5'/);
  assert.match(css, /\.glade-cm-workbench\s*\{/);
  assert.match(css, /\.glade-cm-support\s*\{/);
  assert.match(css, /\.glade-cm-support code\s*\{/);
  assert.match(css, /\.glade-cm-editor \.cm-editor\s*\{[\s\S]*min-height: 430px;/);
  const deps = {
    ...(packageJson.dependencies || {}),
    ...(packageJson.devDependencies || {}),
  };
  assert.ok(deps.codemirror);
  assert.ok(deps["@codemirror/autocomplete"]);
  assert.equal(deps["@codemirror/lang-java"], undefined);
});

test("docs code blocks and tables fill their content lane cleanly", () => {
  assert.match(css, /\.vp-doc div\[class\*='language-'\] pre\s*\{[\s\S]*padding: 22px 24px;/);
  assert.match(css, /\.vp-doc div\[class\*='language-'\] code\s*\{[\s\S]*font-size: max\(13\.5px, var\(--fs-code\)\);[\s\S]*line-height: 1\.55;/);
  assert.match(css, /\.vp-doc div\[class\*='language-'\] > span\.lang\s*\{[\s\S]*top: 0;[\s\S]*right: 0;[\s\S]*display: inline-flex;[\s\S]*height: 28px;/);
  assert.match(css, /\.vp-doc div\[class\*='language-'\] > button\.copy\s*\{[\s\S]*width: 34px;[\s\S]*height: 28px;/);
  assert.match(css, /\.vp-doc table\s*\{[\s\S]*width: 100%;[\s\S]*margin: 20px 0 28px;[\s\S]*border-radius: 12px;/);
  assert.match(css, /\.vp-doc th,\s*\n\.vp-doc td\s*\{[\s\S]*padding: 14px 18px;/);
  assert.match(css, /\.vp-doc tbody tr:nth-child\(even\)\s*\{[\s\S]*background:/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.vp-doc table\s*\{[\s\S]*display: block;[\s\S]*overflow-x: auto;/);
});

test("theme uses the Host Signal design direction", () => {
  assert.match(packageJson.devDependencies["@fontsource-variable/host-grotesk"], /^\^5\./);
  assert.match(packageJson.devDependencies["@fontsource/monaspace-argon"], /^\^5\./);
  assert.equal(packageJson.devDependencies["@fontsource/apfel-grotezk"], undefined);
  assert.equal(packageJson.devDependencies["@fontsource/commit-mono"], undefined);
  assert.equal(packageJson.devDependencies["@fontsource-variable/fraunces"], undefined);
  assert.equal(packageJson.devDependencies["@lucide/vue"], undefined);
  assert.match(theme, /@fontsource-variable\/host-grotesk/);
  assert.match(theme, /@fontsource\/monaspace-argon\/400\.css/);
  assert.match(theme, /@fontsource\/monaspace-argon\/600\.css/);
  assert.doesNotMatch(theme, /apfel-grotezk|commit-mono|fraunces/);
  assert.doesNotMatch(config, /fonts\.googleapis|fonts\.gstatic|Newsreader|Mona\+Sans|JetBrains\+Mono|IBM\+Plex|Space\+Grotesk|Atkinson/);
  assert.doesNotMatch(css, /Apfel Grotezk|Commit Mono|Fraunces Variable|Newsreader|Mona Sans|JetBrains Mono|Source Serif|Literata|Lora|IBM Plex|Atkinson/);
  assert.match(css, /--vp-font-family-base: 'Host Grotesk Variable', 'Host Grotesk'/);
  assert.match(css, /--vp-font-family-mono: 'Monaspace Argon'/);
  assert.match(css, /--glade-font-accent: var\(--vp-font-family-base\);/);
  assert.match(css, /--font-sans: var\(--vp-font-family-base\);/);
  assert.match(css, /--font-mono: var\(--vp-font-family-mono\);/);
  assert.match(css, /--heading-track: 0em;/);
  assert.match(css, /--body-track: 0em;/);
  assert.match(css, /--code-track: 0em;/);
  assert.match(css, /--heading-weight: 700;/);
  assert.match(css, /--fs-micro: 0\.75rem;/);
  assert.match(css, /--fs-label: 0\.8125rem;/);
  assert.match(css, /--fs-ui: 0\.875rem;/);
  assert.match(css, /--fs-body: 1rem;/);
  assert.match(css, /--fs-code: 0\.875rem;/);
  assert.match(css, /--fs-table: 0\.875rem;/);
  assert.match(css, /--fs-helper: 0\.875rem;/);
  assert.match(css, /--lh-tight: 1\.1;/);
  assert.match(css, /--lh-ui: 1\.35;/);
  assert.match(css, /--lh-body: 1\.65;/);
  assert.match(css, /--lh-code: 1\.6;/);
  assert.match(css, /--vp-sidebar-width: 220px;/);
  assert.match(css, /--vp-layout-max-width: 1360px;/);
  assert.match(css, /--glade-page-max: 1280px;/);
  assert.match(css, /--bg: #070b0d;/);
  assert.match(css, /--surface: #10191e;/);
  assert.match(css, /--text-muted: #a9b8ad;/);
  assert.match(css, /--text-subtle: #7f9187;/);
  assert.match(css, /--success-text: #dfffd1;/);
  assert.match(css, /--danger-text: #ffd8d5;/);
  assert.match(css, /--warning-text: #ffe7a6;/);
  assert.match(css, /--glade: #9be870;/);
  assert.match(css, /--glade-strong: #b7ff8a;/);
  assert.match(css, /--danger: #ff6b61;/);
  assert.match(css, /--focus: #b7ff8a;/);
  assert.match(css, /--vp-nav-height: 60px;/);
  assert.match(css, /--glade-page-gutter: 48px;/);
  assert.match(css, /--glade-shell-width: min\(var\(--glade-page-max\), calc\(100vw - var\(--glade-page-gutter\)\)\);/);
  assert.match(css, /\.VPNavBarMenu\s*\{[\s\S]*gap: 8px;/);
  assert.match(css, /\.VPNavBarMenuLink\s*\{[\s\S]*padding-inline: 8px;/);
  assert.match(css, /\.VPNavBar > \.wrapper > \.container\s*\{[\s\S]*width: var\(--glade-shell-width\);[\s\S]*margin-inline: auto;[\s\S]*padding-inline: 0;/);
  assert.match(css, /\.VPNavBarTitle\s*\{[\s\S]*height: var\(--vp-nav-height\);[\s\S]*align-items: center;[\s\S]*margin-right: 24px;/);
  assert.match(css, /body:has\(\.VPHome\) \.VPNavBarTitle\s*\{[\s\S]*margin-left: 0;[\s\S]*margin-right: clamp\(40px, 6vw, 84px\);/);
  assert.match(css, /body:has\(\.VPHome\) \.VPNavBarSearch\s*\{[\s\S]*display: none;/);
  assert.match(css, /\.VPNavBar > \.wrapper > \.container > \.title\s*\{[\s\S]*align-items: center;[\s\S]*padding-left: 0;/);
  assert.match(css, /body:not\(:has\(\.VPHome\)\) \.VPNavBar > \.wrapper > \.container > \.title\s*\{[\s\S]*padding-left: 24px;/);
  assert.match(css, /\.VPNavBar > \.wrapper > \.container > \.content\s*\{[\s\S]*padding-right: 0;/);
  assert.match(css, /\.VPNavBarTitle \.title\s*\{[\s\S]*gap: 8px;[\s\S]*height: 38px !important;[\s\S]*border: 0;[\s\S]*background: transparent;/);
  assert.match(css, /\.VPNavBarTitle \.title\s*\{[\s\S]*font-size: 0\.9375rem;/);
  assert.match(css, /\.VPNavBarTitle \.logo\s*\{[\s\S]*width: 20px;[\s\S]*height: 20px;/);
  assert.match(css, /\.VPNavBar\.has-sidebar \.content,\s*\n\.VPNavBar:not\(\.home\)\s*\{[\s\S]*background-color: var\(--glade-nav-bg\) !important;[\s\S]*backdrop-filter: none;/);
  assert.match(css, /\.VPNavBar:not\(\.home\)\s*\{[\s\S]*border-bottom: 0;/);
  assert.match(css, /\.VPLocalNav\.has-sidebar\s*\{[\s\S]*border-top: 0;[\s\S]*border-bottom: 1px solid var\(--glade-border\);/);
  assert.doesNotMatch(theme, /DocsNavTitleSuffix/);
  assert.doesNotMatch(css, /\.docs-title-suffix\s*\{/);
  assert.match(css, /\.VPSidebar\s*\{[\s\S]*top: var\(--vp-nav-height\) !important;[\s\S]*height: calc\(100vh - var\(--vp-nav-height\)\) !important;[\s\S]*padding-top: 0 !important;[\s\S]*backdrop-filter: none;[\s\S]*box-shadow: none !important;/);
  assert.match(css, /\.VPSidebar \.nav\s*\{[\s\S]*padding-top: 28px;/);
  assert.match(css, /\.VPDoc \.aside-curtain\s*\{[\s\S]*display: none !important;/);
  assert.match(css, /body:not\(:has\(\.VPHome\)\) \.VPNavBar\.has-sidebar \.content-body\s*\{[\s\S]*justify-content: flex-start;[\s\S]*gap: 18px;/);
  assert.match(css, /body:not\(:has\(\.VPHome\)\) \.VPNavBar\.has-sidebar \.VPNavBarMenu\s*\{[\s\S]*margin-left: auto;/);
  assert.match(css, /\.VPNavBarSearch\s*\{[\s\S]*width: clamp\(220px, 26vw, 420px\);/);
  assert.match(css, /@media \(max-width: 980px\)\s*\{[\s\S]*\.VPNavBarSearch\s*\{[\s\S]*display: none !important;/);
  assert.match(css, /\.VPHome \.VPHero\s*\{[\s\S]*display: none !important;/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*grid-template-columns: minmax\(500px, 1fr\) minmax\(360px, 560px\);/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*gap: var\(--space-14\);/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*padding: 42px 0 20px;/);
  assert.match(css, /\.home-hero-shell\s*\{[\s\S]*width: var\(--glade-shell-width\);/);
  assert.match(css, /\.VPButton\.brand:hover\s*\{[\s\S]*color: var\(--text-inverse\) !important;/);
  assert.match(css, /\.vp-doc \.home-cta,\s*\n\.vp-doc \.home-cta:hover,\s*\n\.vp-doc \.home-cta:focus-visible\s*\{[\s\S]*text-decoration-line: none !important;/);
  assert.match(css, /\.home-hero-shell h1\s*\{[\s\S]*font-size: var\(--text-hero\);[\s\S]*letter-spacing: var\(--heading-track\);[\s\S]*line-height: 0\.96;/);
  assert.match(css, /\.vp-doc \.home-h2\s*\{[\s\S]*margin: 0;[\s\S]*border-top: 0;[\s\S]*padding-top: 0;/);
  assert.match(css, /\.home-loop-visual\s*\{[\s\S]*border: 1px solid var\(--glade-home-border\);/);
  assert.match(css, /@media \(max-width: 1120px\)\s*\{[\s\S]*\.home-hero-shell\s*\{[\s\S]*grid-template-columns: 1fr;/);
  assert.doesNotMatch(css, /\.VPHero \.name\s*\{[\s\S]*border-radius: 999px;/);
  assert.doesNotMatch(css, /\.VPHero \.text\s*\{[\s\S]*font-weight: 740/);
  assert.match(css, /\.vp-doc h1\s*\{[\s\S]*font-family: var\(--vp-font-family-base\);[\s\S]*font-style: normal;/);
  assert.match(config, /title: 'Glade - Local Apex Runtime for SFDX Projects'/);
  assert.match(config, /description: 'Local Apex checks and focused tests before the Salesforce validation gate\.'/);
  assert.match(config, /\['meta', \{ property: 'og:title', content: 'Glade — Local Apex runtime for SFDX projects' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:description', content: 'Run supported Apex checks before the Salesforce round trip\.' \}\]/);
  assert.match(config, /\['meta', \{ property: 'og:type', content: 'website' \}\]/);
  assert.match(config, /Local Apex runtime for SFDX projects/);
  assert.match(index, /<h1>Local Apex runtime for SFDX projects<\/h1>/);
  assert.match(config, /Local Apex Runtime for SFDX Projects/);
  assert.match(config, /siteTitle: 'Glade'/);
  assert.match(config, /Salesforce validation gate/);
  assert.match(config, /lastUpdated: false/);
  assert.match(config, /\['link', \{ rel: 'icon', type: 'image\/svg\+xml', href: '\/logo-mark\.svg' \}\]/);
  assert.match(config, /\{ text: 'Workflows', link: '\/guide\/workflows' \}/);
  assert.match(config, /\{ text: 'Product areas', link: '\/guide\/modules' \}/);
  assert.match(config, /\{ text: 'Reference', link: '\/reference\/cli' \}/);
  assert.match(config, /\{ text: 'What is Glade\?', link: '\/guide\/overview' \}/);
  assert.match(config, /text: 'What is Glade\?'/);
  assert.match(config, /text: 'First local check'/);
  assert.match(config, /text: 'Tester field guide'/);
  assert.match(config, /text: 'What runs locally'/);
  assert.doesNotMatch(config, /text: 'Capability map'/);
  assert.doesNotMatch(config, /text: 'Coverage workbench'/);
  assert.match(config, /text: 'Workflows',\n\s+collapsed: false,/);
  assert.match(config, /text: 'Product areas',\n\s+collapsed: false,/);
  assert.match(config, /text: 'Run Apex tests'/);
  assert.match(config, /text: 'Debug Apex'/);
  assert.match(config, /text: 'Affected tests'/);
  assert.match(config, /text: 'Local API routes'/);
  assert.match(config, /text: 'Preview LWC'/);
  assert.match(config, /text: 'Preview Visualforce'/);
  assert.match(config, /text: 'Use VS Code'/);
  assert.match(config, /text: 'Execute anonymous Apex and SOQL'/);
  assert.match(config, /text: 'Editor and workbench'/);
  assert.match(config, /text: 'Automation and JSON'/);
  assert.match(config, /text: 'Error codes'/);
  assert.match(config, /text: 'Reports and package artifacts'/);
  assert.match(config, /text: 'Plugins'/);
  assert.match(config, /text: 'First-party plugins'/);
  assert.match(config, /text: 'Plugin install and manage'/);
  assert.match(config, /text: 'Plugin lock files and CI'/);
  assert.match(config, /text: 'Maintainer'/);
  assert.match(config, /link: '\/maintainer\/release'/);
  assert.match(config, /link: '\/maintainer\/extend-runtime'/);
  assert.match(config, /link: '\/maintainer\/glade-tools'/);
  assert.doesNotMatch(config, /text: 'Build a plugin'/);
  assert.doesNotMatch(config, /text: 'Manifest reference'/);
  assert.doesNotMatch(config, /text: 'Marketplace'/);
  assert.doesNotMatch(config, /text: 'Publish'/);
  assert.doesNotMatch(config, /text: 'Compatibility \/ proof reports'/);
  assert.doesNotMatch(config, /Automation And|Error Codes And|Marketplace And|Install And|Built-In|First-Party|Build A Plugin|Plugin Lock Files And CI/);
  assert.ok(config.indexOf("text: 'What is Glade?'") < config.indexOf("text: 'Plugins'"));
  assert.doesNotMatch(config, /text: 'Project Status'/);
  assert.doesNotMatch(index, /A clearing for Salesforce work\./);
});

test("Cloudflare Pages build publishes the install route itself", () => {
  assert.match(packageJson.scripts.build, /vitepress build \./);
  assert.match(packageJson.scripts.build, /cp install\.sh \.vitepress\/dist\/install\.sh/);
  assert.doesNotMatch(pagesWorkflow, /deploy-pages|upload-pages-artifact|github-pages|configure-pages/);
  assert.match(siteReadme, /## Cloudflare Pages/);
  assert.match(siteReadme, /Root directory: site/);
  assert.match(siteReadme, /Project name: glade-sh/);
  assert.match(siteReadme, /Build command: npm run build/);
  assert.match(siteReadme, /Build output directory: \.vitepress\/dist/);
  assert.match(siteReadme, /Production branch: main/);
  assert.match(siteReadme, /npm ci/);
  assert.match(siteReadme, /wrangler pages deploy \.vitepress\/dist --project-name glade-sh --branch main/);
  assert.match(siteReadme, /pages deployment list --project-name glade-sh/);
  assert.match(siteReadme, /--environment production --json/);
  assert.match(siteReadme, /rev-parse --short=7 HEAD/);
  assert.match(siteReadme, /\.\[0\]\.Source/);
  assert.match(siteReadme, /guide\/local-testing\?v=\$cache_bust/);
  assert.match(siteReadme, /--cpu-profile/);
  assert.match(siteReadme, /--mem-profile/);
  assert.match(siteReadme, /--perf-json/);
  assert.match(siteReadme, /do not replace Salesforce validation/);
  assert.match(siteReadme, /Latest stable release:<span class="home-release-version">vX\.Y\.Z<\/span>/);
  assert.doesNotMatch(siteReadme, /GitHub Pages/);
});

test("mobile nav screen keeps a visible touch surface", () => {
  assert.match(css, /--vp-nav-screen-bg-color: var\(--glade-nav-bg\);/);
  assert.doesNotMatch(css, /\.VPNav,\s*\n\.VPNavBar\.has-sidebar \.content/);
  assert.match(css, /\.VPNavBar\.screen-open,\s*\n\.VPNavScreen\s*\{[\s\S]*background-color: var\(--glade-nav-bg\) !important;/);
  assert.match(css, /\.VPNavScreen\s*\{[\s\S]*z-index: var\(--vp-z-index-nav\);[\s\S]*backdrop-filter: blur\(16px\);[\s\S]*pointer-events: auto;/);
  assert.match(css, /\.VPNavScreen \.container\s*\{[\s\S]*background: transparent;/);
  assert.match(css, /@media \(max-width: 980px\)\s*\{[\s\S]*\.VPNavBarSearch\s*\{[\s\S]*display: none !important;/);
});

test("docs shell uses a dedicated docs header and stronger navigation states", () => {
  assert.doesNotMatch(theme, /DocsNavTitleSuffix/);
  assert.doesNotMatch(theme, /nav-bar-title-after/);
  assert.match(theme, /DocsEnhancer/);
  assert.doesNotMatch(docsEnhancer, /<template><\/template>/);
  assert.match(docsEnhancer, /class="docs-enhancer-root"/);
  assert.match(docsEnhancer, /hidden/);
  assert.match(docsEnhancer, /currentPath = window\.location\.pathname/);
  assert.match(docsEnhancer, /setAttribute\('data-glade-active', 'true'\)/);
  assert.match(docsEnhancer, /setAttribute\('aria-current', 'page'\)/);
  assert.match(docsEnhancer, /gladeHighlightAllCodeBlocks/);
  assert.match(docsEnhancer, /gladeInitHomeDemos/);
  assert.match(docsEnhancer, /dispatchEvent\(new CustomEvent\('glade:content-updated'\)\)/);
  assert.match(docsEnhancer, /data-command-filter/);
  assert.match(docsEnhancer, /card\.hidden = query\.length > 0/);
  assert.match(css, /\.VPSidebarItem\.is-active > \.item > \.link\s*\{[\s\S]*background: var\(--glade-muted\);/);
  assert.match(css, /\.VPSidebarItem\.is-active > \.item > \.link \.text\s*\{[\s\S]*color: var\(--glade-strong\) !important;/);
  assert.match(css, /\.home-run-button:hover\s*\{[\s\S]*color: var\(--glade-action-text\);/);
  assert.match(css, /\.VPDocAsideOutline\.has-outline:has\(\.VPDocOutlineItem\.root > li:only-child\)\s*\{[\s\S]*display: none;/);
  assert.match(css, /\.docs-intro\s*\{[\s\S]*border: 1px solid var\(--glade-border\);/);
  assert.match(css, /\.docs-command-card\[hidden\]\s*\{[\s\S]*display: none;/);
  assert.match(css, /\.vp-doc div\.language-bash code,\s*\n\.vp-doc div\.language-sh code,\s*\n\.vp-doc div\.language-zsh code\s*\{[\s\S]*white-space: pre-wrap;/);
  assert.match(css, /\.docs-command-filter input:focus-visible\s*\{[\s\S]*outline: 2px solid var\(--glade-focus-ring\);/);
});

test("support docs summarize the checked compatibility artifacts", () => {
  assert.match(overview, /^# What is Glade\?/m);
  assert.match(overview, /class="docs-intro"/);
  assert.match(overview, /class="docs-route-grid"/);
  assert.match(overview, /href="\/guide\/installation"[\s\S]*<strong>Install<\/strong>/);
  assert.match(overview, /href="\/guide\/tester-field-guide"[\s\S]*<strong>Tester field guide<\/strong>/);
  assert.match(overview, /href="\/guide\/ai-assisted-apex"[\s\S]*<strong>AI-assisted Apex<\/strong>/);
  assert.match(overview, /href="\/guide\/workbench"[\s\S]*<strong>Interactive capability map<\/strong>/);
  assert.match(overview, /## First local loop/);
  assert.match(overview, /Glade models the local paths it can prove/);
  assert.match(overview, /local assessment, cruft review, or refactor-proof reports/);
  assert.match(overview, /serves local Visualforce pages/);
  assert.match(overview, /Visualforce preview feature/);
  assert.match(overview, /LWC preview feature/);
  assert.match(overview, /Use Salesforce when/);
  assert.match(overview, /## Capability claims/);
  assert.doesNotMatch(overview, /## Support claims/);
  assert.doesNotMatch(overview, /full Visualforce rendering or PDF generation/);
  assert.match(quickstart, /^# Quickstart: Check and Test an SFDX Project/m);
  assert.match(quickstart, /class="docs-intro"/);
  assert.match(quickstart, /This path installs Glade, checks the project, and runs one focused test\./);
  assert.match(quickstart, /For VS Code, CI, and report workflows/);
  assert.doesNotMatch(quickstart, /in a few minutes|small evaluation with VS Code, AI/);
  assert.match(quickstart, /If `glade` is not found/);
  assert.match(quickstart, /zero diagnostics and exit code `0`/);
  assert.match(quickstart, /\[Tester field guide\]\(\/guide\/tester-field-guide\)/);
  assert.match(quickstart, /glade check --project \./);
  assert.match(quickstart, /glade test --project \. --class RefinementServiceTest/);
  assert.doesNotMatch(quickstart, /--filter/);
  assert.match(quickstart, /glade test changed --project \. --since origin\/main/);
  assert.match(quickstart, /exact hosted Visualforce\s+behavior/);
  assert.doesNotMatch(quickstart, /live auth, Visualforce rendering/);
  assert.match(supportMap, /^# What Glade runs locally/m);
  assert.match(supportMap, /class="docs-support-legend"/);
  assert.match(supportMap, /class="docs-support-legend-card docs-support-legend-card-supported"/);
  const legendStart = supportMap.indexOf('<div class="docs-support-legend"');
  const legendEnd = supportMap.indexOf("</div>", supportMap.indexOf("</div>", supportMap.indexOf("</div>", supportMap.indexOf("</div>", legendStart) + 1) + 1) + 1);
  assert.ok(legendStart > -1);
  assert.ok(legendEnd > legendStart);
  const legendBlock = supportMap.slice(legendStart, legendEnd);
  assert.doesNotMatch(legendBlock, /<p>/);
  assert.match(supportMap, /class="docs-status-chip docs-status-supported">Runs locally/);
  assert.match(supportMap, /Before you adopt Glade/);
  assert.match(supportMap, /Salesforce validation gate/);
  assert.doesNotMatch(supportMap, /Salesforce release gate/);
  assert.match(supportMap, /UnsupportedFeature/);
  assert.match(supportMap, /## Runs locally/);
  assert.match(supportMap, /## Runs with limits/);
  assert.match(supportMap, /Visualforce controller and page rendering[\s\S]*Preview feature/);
  assert.match(supportMap, /Local LWC workbench and routes[\s\S]*Preview feature/);
  assert.match(supportMap, /Local LWC data and services[\s\S]*Preview feature with local-data limits/);
  assert.match(supportMap, /Visualforce Lightning Out for LWCs[\s\S]*Preview feature with limits/);
  assert.match(supportMap, /Local LWC Shell/);
  assert.match(supportMap, /docs\/LWC_SUPPORT\.md/);
  assert.match(supportMap, /Project configuration and package contracts/);
  assert.match(supportMap, /namespace remaps/);
  assert.match(supportMap, /captured package artifacts/);
  assert.match(supportMap, /package shims/);
  assert.doesNotMatch(supportMap, /folded into this page/);
  assert.match(supportMap, /## Requires Salesforce/);
  assert.match(supportMap, /Counts come from the checked standard library capability report/);
  assert.match(supportMap, /\| String, Decimal, Boolean, Math \| Runs locally \| 32 supported \/ 32 tracked \|/);
  assert.match(supportMap, /\| ApexPages and PageReference \| Supported controller rows, hosted rendering gaps \| 15 supported, 2 unsupported \/ 17 tracked \|/);
  assert.match(supportMap, /\| UserInfo, URL, Label, and TrailblazerIdentity \| Broad local capability \| 24 supported \/ 24 tracked \|/);
  assert.match(supportMap, /\| Type, FeatureManagement, and Exception \| Supported local rows, hosted package gap \| 8 supported, 1 unsupported \/ 9 tracked \|/);
  assert.match(supportMap, /\| Local test harness and request context \| Supported local rows, hosted and malformed-input gaps \| 32 supported, 2 unsupported \/ 34 tracked \|/);
  assert.match(supportMap, /\| Hosted-service and platform boundary rows \| Requires Salesforce, plus stable diagnostics \| 1 supported diagnostic row, 2 unsupported \/ 3 tracked \|/);
  assert.match(supportMap, /## Capability claims/);
  assert.match(supportMap, /\| Capability features marked `supported` \| 31 \|/);
  assert.match(supportMap, /\| Capability features marked `partial` \| 0 \|/);
  assert.match(supportMap, /\| Standard-library rows marked `supported` \| 267 \|/);
  assert.match(supportMap, /\| Standard-library rows marked `unsupported` \| 19 \|/);
  assert.match(supportMap, /Approval list\s+processing/);
  assert.match(configuration, /namespaceRemaps: \[\]/);
  assert.match(configuration, /Namespace remaps/);
  assert.match(configuration, /BasePkg:stagepkg/);
  assert.doesNotMatch(supportMap, /Approval\.process is not supported/);
  assert.match(installation, /Recommended path: use the one-line installer/);
  assert.match(installation, /class="docs-install-grid"/);
  assert.match(installation, /Installs the current release to <code>~\/\.local\/bin<\/code>\./);
  assert.doesNotMatch(installation, /private repository release|GLADE_GITHUB_TOKEN/);
  assert.match(installation, /Use in CI or when policy requires pinned artifacts\./);
  assert.match(css, /\.docs-install-card\s*\{[\s\S]*padding: 16px;[\s\S]*border-radius: 12px;[\s\S]*min-height: 112px;/);
  assert.match(css, /\.docs-support-legend\s*\{[\s\S]*position: static;[\s\S]*display: flex;[\s\S]*flex-wrap: wrap;[\s\S]*gap: 4px;[\s\S]*padding: 4px 6px;/);
  assert.doesNotMatch(css, /\.docs-support-legend\s*\{[\s\S]*position: sticky;/);
  assert.match(css, /\.docs-support-legend-card\s*\{[\s\S]*display: inline-flex;[\s\S]*align-items: center;[\s\S]*padding: 0;/);
  assert.match(installation, /glade test --project \. --class RefinementServiceTest --method testRefinesFileRow --json/);
  assert.match(cliReference, /id="cli-command-filter"/);
  assert.match(cliReference, /class="docs-command-card"/);
  assert.match(cliReference, /--class RefinementServiceTest --method testRefinesFileRow/);
  assert.doesNotMatch(cliReference, /--filter/);
  assert.match(playground, /Use built-in examples when you want a safe scratch workspace/);
  assert.match(playground, /\| Group \| Example \| Command \|/);
  assert.doesNotMatch(config, /compatibility-dashboard/);
  assert.doesNotMatch(supportMap, /Maintainer Proof Reports/);
  assert.match(localApiServer, /^# Run local Salesforce API routes/m);
  assert.match(localApiServer, /record counts/);
  assert.match(localApiServer, /Tooling source metadata/);
  assert.match(localApiServer, /Tooling schema metadata/);
  assert.match(localApiServer, /Composite sObject insert/);
  assert.match(localApiServer, /Composite Batch and Tree local requests/);
  assert.match(localApiServer, /Composite Graph local requests/);
  assert.match(localApiServer, /supported local subrequests/);
  assert.doesNotMatch(localApiServer, /Composite Graph execution, Bulk API locator paging/);
  assert.match(localApiServer, /Layout and default metadata/);
  assert.match(localApiServer, /Metadata job status/);
  assert.match(localApiServer, /Bulk API v2 simple scalar query job create\/status\/whole-result CSV/);
  assert.match(localApiServer, /limits\/recordCount\?sObjects=Account/);
  assert.match(supportMap, /Composite Batch, Tree, and Graph local requests/);
  assert.match(supportMap, /Composite Graph local requests/);
  assert.doesNotMatch(supportMap, /Composite Graph execution, Streaming\/PubSub/);
  assert.match(repoCompatibility, /Composite Graph local requests/);
  assert.doesNotMatch(repoCompatibility, /Composite Graph execution, broader Bulk API locator paging/);
  assert.match(repoCompatibilityDashboard, /Composite Graph local requests/);
  assert.match(repoCompatibilityDashboard, /apex\.namespace-resolution/);
  assert.doesNotMatch(repoCompatibilityDashboard, /Composite Graph execution, and broader hosted REST namespaces/);
});

test("AI-assisted Apex guide gives agents a Glade TDD contract", () => {
  assert.match(aiAssistedApex, /^# AI-assisted Apex with Glade/m);
  assert.match(aiAssistedApex, /Paste the long prompt into a global skill,\s+repository instruction file, or\s+agent memory/);
  assert.match(aiAssistedApex, /Use this prompt for any Apex feature, bug fix, or refactor/);
  assert.match(aiAssistedApex, /Write the smallest failing Apex test first/);
  assert.match(aiAssistedApex, /Do not edit production Apex until a Glade test fails for the expected reason/);
  assert.match(aiAssistedApex, /mkdir -p reports/);
  assert.match(aiAssistedApex, /glade doctor/);
  assert.match(aiAssistedApex, /glade config validate --project \./);
  assert.match(aiAssistedApex, /glade check --project \. --format json --output reports\/glade-check\.json --no-progress/);
  assert.match(aiAssistedApex, /glade test --project \. --class <TestClass> --method <TestMethod> --json --no-progress/);
  assert.match(aiAssistedApex, /glade test changed --project \. --since origin\/main --json --no-progress/);
  assert.match(aiAssistedApex, /Quote the exact command and the failing diagnostic/);
  assert.match(aiAssistedApex, /Salesforce remains the validation gate/);
  assert.match(aiAssistedApex, /Use Glade from the SFDX project root/);
  assert.doesNotMatch(aiAssistedApex, /deploy-first|scratch org first|project-specific exception/i);
  assert.match(testerFieldGuide, /\[AI-assisted Apex\]\(\/guide\/ai-assisted-apex\)/);
});

test("preview surfaces are labeled in public and repo docs", () => {
  assert.match(workflowLwcPreview, /preview routes/i);
  assert.match(workflowLwcPreview, /glade dev lwc --project \. --open/);
  assert.match(moduleLwcPreview, /The LWC shell is a local preview surface/);
  assert.match(workflowVisualforcePreview, /\/apex\/<PageName>/);
  assert.match(workflowVisualforcePreview, /glade dev vf --project \./);
  assert.match(moduleVisualforcePreview, /The Visualforce server is a local preview surface/);
  assert.match(localTesting, /\[Preview LWC locally\]\(\/guide\/workflows\/lwc-preview\)/);
  assert.match(localTesting, /\[Preview Visualforce locally\]\(\/guide\/workflows\/visualforce-preview\)/);
  assert.match(lwcLocalShell, /"phase3BaseComponents"/);
  assert.match(supportMap, /`lightning\/refresh`/);
  assert.match(supportMap, /packaged SLDS 2 and classic SLDS assets/);
  assert.match(index, /Visualforce and LWC local shells remain preview features\./);
  assert.match(repoReadme, /Visualforce preview feature[\s\S]*glade dev vf/);
  assert.match(repoCompatibility, /Visualforce dev rendering \| preview feature/i);
  assert.match(repoLwcSupport, /Direct component shell \| Preview feature/i);
  assert.match(repoLwcSupport, /Visualforce Lightning Out \| Preview feature/i);
});

test("LWC local shell docs describe Workbench Console workflow", () => {
  assert.match(lwcLocalShell, /Workbench Console/);
  assert.match(lwcLocalShell, /route discovery/);
  assert.match(lwcLocalShell, /preview canvas/);
  assert.match(lwcLocalShell, /editable context/);
  assert.match(lwcLocalShell, /debug\s+panes[\s\S]*Apex[\s\S]*LDS[\s\S]*network calls[\s\S]*navigation\/events[\s\S]*runtime issues/);
  assert.match(lwcLocalShell, /mobile preview[\s\S]*main canvas/i);
  assert.match(lwcLocalShell, /does not reserve a side-by-side phone panel/i);
  assert.match(workflowLwcPreview, /Workbench Console/);
  assert.match(workflowLwcPreview, /debug\s+panes[\s\S]*network calls[\s\S]*navigation\/events/);
  assert.match(workflowLwcPreview, /mobile preview[\s\S]*main canvas/i);
  assert.match(workflowLwcPreview, /permanent side-by-side phone panel/i);
  assert.match(repoLwcSupport, /local shell UI is the Workbench Console/);
  assert.match(repoLwcSupport, /component, record page, builder, tab, and\s+app contexts/);
  assert.match(repoLwcSupport, /Apex, LDS, navigation,\s+PageReference, and runtime issues/);
});

test("enterprise workflow docs expose current report commands", () => {
  assert.match(enterpriseWorkflows, /^# Enterprise Workflows/m);
  assert.match(enterpriseWorkflows, /glade inspect graph --project \. --json/);
  assert.match(enterpriseWorkflows, /glade report assess --project \. --format html/);
  assert.match(enterpriseWorkflows, /glade report cruft --project \. --format html/);
  assert.match(enterpriseWorkflows, /glade report refactor-proof --project \. --since origin\/main/);
  assert.match(enterpriseWorkflows, /--fail-on-api-break/);
  assert.match(enterpriseWorkflows, /Compatibility and support-map generation remain plugin-owned/);
});

test("cli reference documents current code intelligence commands", () => {
  assert.match(cliReference, /## `glade update`/);
  assert.match(cliReference, /glade update --dry-run/);
  assert.match(cliReference, /GLADE_UPDATE_ALLOW_SHELL=1 glade update/);
  assert.match(cliReference, /glade inspect definition --project \. --symbol RefinementService/);
  assert.match(cliReference, /glade inspect definition --project \. --file force-app\/main\/default\/classes\/RefinementService\.cls --line 6 --column 13/);
  assert.match(cliReference, /glade inspect references --project \. --symbol RefinementService\.total --json/);
  assert.match(cliReference, /glade inspect references --project \. --symbol Account\.Name --include-declaration/);
  assert.match(cliReference, /glade refactor rename --project \. --symbol RefinementService --to FileRefinementService --dry-run --json/);
  assert.match(cliReference, /glade refactor rename --project \. --file force-app\/main\/default\/classes\/RefinementService\.cls --line 5 --column 14 --to totalNet --write/);
  assert.match(cliReference, /glade schema import describe --input reports\/org-describe\.json --output schema\/local\.schema\.json --project-cache \./);
  assert.match(cliReference, /writes schema symbols under `\.glade\/symbols`/);
  assert.match(cliReference, /href="#glade-tui"/);
  assert.match(cliReference, /## `glade tui`/);
  assert.match(cliReference, /glade tui --project \. --view tests/);
  assert.match(cliReference, /glade tui --project \. --db \.glade\/refinement-local\.sqlite --view data --target-org devhub --object Account/);
  assert.match(cliReference, /Parse, profile, explain, analyze for editors, and synthesize from Salesforce\s+debug logs\./);
  assert.match(cliReference, /glade debug editor --log apex\.log --project \. --json/);
  assert.match(cliReference, /glade debug repro --log apex\.log --project \. > ReproTest\.cls/);
  assert.match(cliReference, /glade debug replay --log apex\.log --project \. --json/);
  assert.match(cliReference, /Seed, import, query, describe, inspect, reset, and export persistent local org state/);
  assert.match(cliReference, /Seed, import, query, describe, inspect, reset, and export local org storage fixtures/);
  assert.match(moduleDebugProfile, /glade exec --project \. --trace reports\/trace\.json/);
  assert.match(moduleDebugProfile, /glade profile analyze reports\/trace\.json --format pprof/);
  assert.match(workflowDebugApex, /glade exec --project \. --trace reports\/trace\.json/);
  assert.match(workflowDebugApex, /glade profile analyze reports\/trace\.json --format pprof/);
  assert.doesNotMatch(moduleDebugProfile + workflowDebugApex, /glade profile analyze --log/);
  assert.match(storageSchema, /`glade db import sf`/);
  assert.match(storageSchema, /`glade db query`/);
  assert.match(storageSchema, /`glade db describe`/);
  assert.match(editor, /glade inspect definition --project \. --symbol RefinementService/);
  assert.match(editor, /glade inspect references --project \. --symbol Account\.Name --include-declaration/);
  assert.match(editor, /glade refactor rename --project \. --symbol RefinementService --to FileRefinementService --dry-run/);
  assert.match(cliReference, /\[Use Glade as an sf target\]\(\/guide\/glade-orgs\)/);
  assert.match(gladeOrgs, /glade org create refinement-local\n```/);
  assert(gladeOrgs.indexOf("glade org create refinement-local") < gladeOrgs.indexOf("--db .glade/orgs/refinement-local.sqlite"));
  assert.match(index, /glade org create refinement-local<\/code><\/pre>/);
  assert.match(gladeOrgs, /sf data import tree -p \.\/data\/fileRowsImport\.json -o refinement-local/);
  assert.match(gladeOrgs, /It is not a real scratch\s+org/);
  assert.match(gladeOrgs, /Bulk API v1 CSV insert and upsert baseline/);
});

test("ci docs create reports directory before report outputs", () => {
  for (const page of [automation, localTesting, affectedTests, workflowCi]) {
    const mkdirIndex = page.indexOf("mkdir -p reports");
    const junitIndex = page.indexOf("glade test --project . --junit reports/glade-junit.xml");
    assert.notEqual(mkdirIndex, -1);
    assert.notEqual(junitIndex, -1);
    assert.ok(mkdirIndex < junitIndex);
  }

  const mkdirIndex = workflowCi.indexOf("mkdir -p reports");
  const sarifIndex = workflowCi.indexOf("glade check --project . --format sarif --output reports/glade-check.sarif");
  assert.notEqual(sarifIndex, -1);
  assert.ok(mkdirIndex < sarifIndex);
});

test("public launch docs avoid stale public routes and registry promises", () => {
  assert.match(siteReadme, /https:\/\/glade\.sh\/guide\/support-map/);
  assert.doesNotMatch(siteReadme, /https:\/\/glade\.sh\/docs\/guide\//);
  assert.match(testerFieldGuide, /^# Tester field guide/m);
  assert.match(testerFieldGuide, /Use this guide for a first project evaluation with Salesforce engineers\./);
  assert.match(testerFieldGuide, /^## First project setup/m);
  assert.match(testerFieldGuide, /glade editor install vscode --force/);
  assert.match(testerFieldGuide, /AI coding agent/);
  assert.match(testerFieldGuide, /fetch-depth: 0/);
  assert.match(testerFieldGuide, /glade report refactor-proof --project \. --since origin\/main/);
  assert.match(testerFieldGuide, /glade dev vf --project \. --addr 127\.0\.0\.1:8080/);
  assert.match(testerFieldGuide, /glade dev lwc --project \. --open/);
  assert.match(testerFieldGuide, /Glade Home/);
  assert.match(testerFieldGuide, /Exec & SOQL scratch\s+buffers/);
  assert.match(testerFieldGuide, /exact hosted Visualforce\s+behavior/);
  assert.doesNotMatch(testerFieldGuide, /Visualforce rendering path/);
  assert.match(testerFieldGuide, /Useful first-run feedback includes:/);
  assert.match(testerFieldGuide, /default public plugin registry serves the three first-party packages/);
  assert.match(ciArtifacts, /fetch-depth: 0/);
  assert.match(installation, /^## Install VS Code Extension/m);
  assert.match(installation, /glade editor doctor vscode/);
  assert.match(installation, /glade editor install vscode --force/);
  assert.match(installation, /glade update --dry-run/);
  assert.match(installation, /glade update/);
  assert.match(installation, /share\/glade\/editor\/vscode-glade\.vsix/);
  assert.match(installation, /Glade Home/);
  assert.match(installation, /Exec & SOQL scratch buffers/);
  assert.match(installation, /\[Editor, LSP, and DAP\]\(\/guide\/editor\)/);
  assert.match(installation, /For a first project run, use the \[Tester field guide\]\(\/guide\/tester-field-guide\)\./);
  assert.match(editor, /^## VS Code Extension/m);
  assert.match(editor, /`Glade: Open Home` command/);
  assert.match(editor, /Exec & SOQL/);
  assert.match(editor, /Click \*\*Run local proof\*\*/);
  assert.match(editor, /^## Plugin actions and findings/m);
  assert.match(editor, /glade test --project PROJECT_ROOT --json --class CLASS_NAME --method METHOD_NAME/);
  assert.match(editor, /glade test changed --project PROJECT_ROOT --since origin\/main --json/);
  assert.match(editor, /glade test --project PROJECT_ROOT --daemon --watch/);
  assert.match(editor, /glade exec --project PROJECT_ROOT --db ACTIVE_DB --log-out reports\/exec\.log "insert new Account\(Name='local'\);"/);
  assert.match(editor, /glade dap --project PROJECT_ROOT --db ACTIVE_DB/);
  assert.doesNotMatch(editor, new RegExp("<" + "root>|<" + "Class>|<" + "Method>|<" + "active-db>"));
  assert.match(editor, /Glade Activity Bar/);
  assert.doesNotMatch(editor, /Click \*\*Run local check\*\*/);
  assert.match(ciArtifacts, /mkdir -p reports/);
  assert.match(plugins, /Most Glade work does not require plugins\./);
  assert.match(plugins, /Use plugins when you need a first-party extension that stays outside the base runtime\./);
  assert.match(plugins, /@glade\/orgpackage/);
  assert.match(plugins, /glade package capture \.\.\.[\s\S]*dispatches to `glade orgpackage capture \.\.\.`/);
  for (const pluginRoute of [
    "/guide/plugins/first-party",
    "/guide/plugins/install-manage",
    "/guide/plugins/lock-ci"
  ]) {
    assert.match(plugins, new RegExp(`href="${pluginRoute}"`));
    assert.match(config, new RegExp(`link: '${pluginRoute}'`));
  }
  for (const pluginRoute of [
    "/guide/plugins/build",
    "/guide/plugins/manifest",
    "/guide/plugins/marketplace",
    "/guide/plugins/publish"
  ]) {
    assert.doesNotMatch(plugins, new RegExp(`href="${pluginRoute}"`));
    assert.doesNotMatch(config, new RegExp(`link: '${pluginRoute}'`));
  }
  assert.match(plugins, /Maintainer support tools/);
  assert.doesNotMatch(plugins, /glade plugins link --exec \.\/glade-plugin-quality/);
  assert.match(firstPartyPlugins, /default public registry[\s\S]*https:\/\/plugins\.glade\.sh\/index\.json/);
  assert.match(firstPartyPlugins, /Maintainer support tools/);
  assert.doesNotMatch(firstPartyPlugins, /glade plugins install @glade\/compat/);
  assert.match(firstPartyPlugins, /@glade\/orgpackage/);
  assert.match(firstPartyPlugins, /glade package capture --target-org packaging/);
  assert.match(pluginInstallManage, /Direct archives and local links remain available for offline, private, and development use/);
  assert.doesNotMatch(pluginInstallManage, /glade plugins install @glade\/compat/);
  assert.match(pluginInstallManage, /glade plugins install @glade\/orgpackage/);
  assert.match(pluginLockCi, /^# Plugin lock files and CI/m);
  assert.match(pluginLockCi, /default public registry serves the three first-party packages/);
  assert.doesNotMatch(pluginLockCi, /glade plugins install @glade\/compat/);
  assert.match(pluginLockCi, /glade plugins install @glade\/orgpackage/);
  assert.match(maintainerIndex, /glade stays the product front door/);
  assert.match(extendRuntime, /Write the failing fixture or product test first/);
  assert.match(extendRuntime, /go test \.\/internal\/vm \.\/internal\/apextest/);
  assert.match(gladeToolsMaintainer, /go run \.\/cmd\/glade-plugin-compat manifest --json/);
  assert.match(gladeToolsMaintainer, /scripts\/build-plugin-archives\.sh X\.Y\.Z/);
  assert.match(gladeToolsMaintainer, /@glade\/compat` is the published maintainer package/);
  assert.doesNotMatch(gladeToolsMaintainer, /\/maintainer\/tools/);
  assert.match(pluginRuntime, /Plugins are executable processes/);
  assert.doesNotMatch(config, /link: '\/maintainer\/tools\/'/);
  assert.doesNotMatch(packageJson.scripts.test, /sync:tools-docs/);
  assert.doesNotMatch(packageJson.scripts.build, /sync:tools-docs/);
  assert.equal(packageJson.scripts["sync:tools-docs"], undefined);
  assert.match(packageJson.scripts.test, /check:routes/);
  assert.match(packageJson.scripts.build, /check:routes/);
  assert.match(packageJson.scripts.test, /help:check/);
  assert.match(packageJson.scripts.build, /help:check/);
  assert.match(checkDocRoutesScript, /Missing docs route source files/);
  assert.doesNotMatch(lwcLocalShell, /cd \.\.\/glade-tools/);
  assert.doesNotMatch(localTesting, /cd \.\.\/glade-tools/);
  assert.match(lwcLocalShell, /maintainer\/glade-tools/);
  assert.doesNotMatch(allPublicGuideText, /cd \.\.\/glade-tools/);
  assert.doesNotMatch(allPublicGuideText, /oaer-probe-max/);
  assert.doesNotMatch(allPublicGuideText, /Salesforce Docs Scraper/);
  assert.match(lwcLocalShell, /^## Data and services/m);
  assert.match(enterpriseWorkflows, /^## Cruft and dead code/m);
  for (const staleHeading of [
    new RegExp("Data " + "And Services"),
    new RegExp("Plugin Actions " + "And Findings"),
    new RegExp("Cruft " + "And Dead Code")
  ]) {
    assert.doesNotMatch(siteCopy, staleHeading);
  }
});

test("site palette uses glade green and operational states", () => {
  assert.doesNotMatch(css, /#37d9ff|#a7f2ff|rgba\(55,\s*217,\s*255/);
  assert.doesNotMatch(css, /#73b7bf|#93b17d|#2e7882|#5d754f/i);
  assert.match(css, /--bg: #070b0d;/);
  assert.match(css, /--surface: #10191e;/);
  assert.match(css, /--surface-raised: #14232a;/);
  assert.match(css, /--line: #26363d;/);
  assert.match(css, /--glade: #9be870;/);
  assert.match(css, /--glade-strong: #b7ff8a;/);
  assert.match(css, /--warning: #f5c95f;/);
  assert.match(css, /--danger: #ff6b61;/);
  assert.match(css, /--info: #7db7ff;/);
  assert.match(css, /--glade-state-supported: var\(--success\);/);
  assert.match(css, /--glade-state-partial: var\(--warning\);/);
  assert.match(css, /--glade-state-failed: var\(--danger\);/);
  assert.match(css, /--glade-state-info: var\(--info\);/);
  assert.match(css, /--glade-accent: var\(--glade\);/);
  assert.match(css, /--glade-moss: var\(--glade\);/);
  assert.match(logoMark, /stroke="#9BE870"/);
  assert.match(logoMark, /stroke="#B7FF8A"/);
  assert.match(logoMarkOpen, /stroke="#9BE870"/);
  assert.match(css, /--glade-focus-ring: var\(--focus\);/);
  assert.match(css, /--glade-status-warning: var\(--warning\);/);
  assert.match(css, /:focus-visible\s*\{[\s\S]*outline: 2px solid var\(--glade-focus-ring\);/);
  assert.match(css, /\.home-install-strip code\s*\{/);
  assert.match(css, /\.home-loop-visual\s*\{/);
  assert.doesNotMatch(css, /--glade-font-mona-sans|--glade-font-ibm-sans|--glade-font-ibm-mono|--glade-font-atkinson|--glade-font-monaspace/);
  assert.match(css, /--glade-font-accent: var\(--vp-font-family-base\);/);
  assert.doesNotMatch(css, /\.brand-lab|\.color-lab|--lab-line/);
});

test("contour background is a single field instead of a visible tile", () => {
  assert.match(css, /body::before\s*\{[\s\S]*background-repeat: no-repeat;/);
  assert.match(css, /body::before\s*\{[\s\S]*background-size: max\(1450px, 120vw\) auto;/);
  assert.doesNotMatch(css, /background-size: 900px 520px;/);
});
