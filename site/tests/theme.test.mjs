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
const config = await readFile(new URL("../.vitepress/config.ts", import.meta.url), "utf8");
const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
const siteReadme = await readFile(new URL("../README.md", import.meta.url), "utf8");
const pagesWorkflow = await readFile(new URL("../../.github/workflows/pages.yml", import.meta.url), "utf8").catch(() => "");
const repoReadme = await readFile(new URL("../../README.md", import.meta.url), "utf8");
const repoCompatibility = await readFile(new URL("../../docs/COMPATIBILITY.md", import.meta.url), "utf8");
const repoLwcSupport = await readFile(new URL("../../docs/LWC_SUPPORT.md", import.meta.url), "utf8");
const highlight = await readFile(new URL("../docs-src/public/js/highlight.js", import.meta.url), "utf8");
const homeScript = await readFile(new URL("../docs-src/public/js/home.js", import.meta.url), "utf8");
const ciArtifacts = await readFile(new URL("../docs-src/guide/ci-artifacts.md", import.meta.url), "utf8");
const automation = await readFile(new URL("../docs-src/guide/automation.md", import.meta.url), "utf8");
const installation = await readFile(new URL("../docs-src/guide/installation.md", import.meta.url), "utf8");
const overview = await readFile(new URL("../docs-src/guide/overview.md", import.meta.url), "utf8");
const quickstart = await readFile(new URL("../docs-src/guide/quickstart.md", import.meta.url), "utf8");
const cliReference = await readFile(new URL("../docs-src/guide/cli-reference.md", import.meta.url), "utf8");
const localTesting = await readFile(new URL("../docs-src/guide/local-testing.md", import.meta.url), "utf8");
const affectedTests = await readFile(new URL("../docs-src/guide/affected-tests.md", import.meta.url), "utf8");
const playground = await readFile(new URL("../docs-src/guide/playground.md", import.meta.url), "utf8");
const workbench = await readFile(new URL("../docs-src/guide/workbench.md", import.meta.url), "utf8").catch(() => "");
const testerFieldGuide = await readFile(new URL("../docs-src/guide/tester-field-guide.md", import.meta.url), "utf8");
const editor = await readFile(new URL("../docs-src/guide/editor.md", import.meta.url), "utf8");
const supportMap = await readFile(new URL("../docs-src/guide/support-map.md", import.meta.url), "utf8");
const localApiServer = await readFile(new URL("../docs-src/guide/local-api-server.md", import.meta.url), "utf8");
const gladeOrgs = await readFile(new URL("../docs-src/guide/glade-orgs.md", import.meta.url), "utf8");
const lwcLocalShell = await readFile(new URL("../docs-src/guide/lwc-local-shell.md", import.meta.url), "utf8");
const enterpriseWorkflows = await readFile(new URL("../docs-src/guide/enterprise-workflows.md", import.meta.url), "utf8");
const plugins = await readFile(new URL("../docs-src/guide/plugins.md", import.meta.url), "utf8");
const firstPartyPlugins = await readFile(new URL("../docs-src/guide/plugins/first-party.md", import.meta.url), "utf8");
const pluginMarketplace = await readFile(new URL("../docs-src/guide/plugins/marketplace.md", import.meta.url), "utf8");
const pluginInstallManage = await readFile(new URL("../docs-src/guide/plugins/install-manage.md", import.meta.url), "utf8");
const pluginLockCi = await readFile(new URL("../docs-src/guide/plugins/lock-ci.md", import.meta.url), "utf8");
const logoMark = await readFile(new URL("../docs-src/public/logo-mark.svg", import.meta.url), "utf8");
const logoMarkOpen = await readFile(new URL("../docs-src/public/logo-mark-open.svg", import.meta.url), "utf8");

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
const siteCopy = [
  index,
  config,
  workbench,
  codeMirrorWorkbench,
  homeScript,
  ...siteSourceFiles.map(([, contents]) => contents)
].join("\n");

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
  assert.match(index, /href="\/guide\/quickstart"[^>]*data-demo-link[^>]*>Run your first local check<\/a>/);
  assert.match(index, /href="\/guide\/support-map"[\s\S]*>What runs locally<\/a>/);
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
  assert.doesNotMatch(index, /<h3>Test<\/h3>[\s\S]*AccountServiceTest/);
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
  assert.match(index, /glade server --project \. --db \.glade\/local-org\.sqlite --addr 127\.0\.0\.1:8080/);
  assert.match(index, /glade db seed --db \.glade\/local-org\.sqlite --project \. seed\.json/);
  assert.match(index, /aria-label="Optional plugins"/);
  assert.match(index, /<h2 class="home-h2">Optional plugins<\/h2>/);
  assert.match(index, /The base runtime stays focused on local Apex workflows\. Add plugins only when a project needs capability reports, advisory scans, or custom local checks\./);
  assert.match(index, /Base Glade workflows do not require plugins\. Registry commands are preview until a registry, archive URL, or linked plugin is configured\./);
  assert.match(index, /glade plugins list/);
  assert.match(index, /glade plugins link --exec \.\/glade-plugin-quality/);
  assert.match(index, /glade plugins install @glade\/compat/);
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
  assert.match(config, /\{ text: 'Capabilities', link: '\/guide\/support-map' \}/);
  assert.match(config, /\{ text: 'VS Code', link: '\/guide\/editor' \}/);
  assert.match(config, /\{ text: 'sf target orgs', link: '\/guide\/glade-orgs' \}/);
  assert.match(config, /\{ text: 'Docs', link: '\/guide\/overview' \}/);
  assert.match(config, /\{ text: 'Install', link: '\/guide\/installation' \}/);
  assert.doesNotMatch(config, /\{ text: 'Coverage', link: '\/guide\/workbench' \}/);
  assert.doesNotMatch(config, /\{ text: 'Capability map', link: '\/guide\/support-map' \}/);
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
  assert.match(config, /\{ text: 'Capabilities', link: '\/guide\/support-map' \}/);
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
  assert.match(cliReference, /glade orgpackage capture --target-org packaging --namespace pkg/);
  assert.equal((cliReference.match(/auto-connect through `\.glade\/test\/serve\.sock` unless `--no-serve` is set\./g) || []).length, 1);
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
  assert.match(css, /--vp-sidebar-width: 240px;/);
  assert.match(css, /--vp-layout-max-width: 1280px;/);
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
  assert.match(config, /\{ text: 'Docs', link: '\/guide\/overview' \}/);
  assert.match(config, /text: 'What is Glade\?'/);
  assert.match(config, /text: 'First local check'/);
  assert.match(config, /text: 'What runs locally'/);
  assert.doesNotMatch(config, /text: 'Capability map'/);
  assert.doesNotMatch(config, /text: 'Coverage workbench'/);
  assert.match(config, /text: 'Workflows',\n\s+collapsed: true,/);
  assert.match(config, /text: 'Check source'/);
  assert.match(config, /text: 'Run tests'/);
  assert.match(config, /text: 'Affected tests'/);
  assert.match(config, /text: 'Local API routes'/);
  assert.match(config, /text: 'VS Code'/);
  assert.match(config, /text: 'Automation and JSON'/);
  assert.match(config, /text: 'Error codes and `glade explain`'/);
  assert.match(config, /text: 'Reports and package artifacts'/);
  assert.match(config, /text: 'Plugins'/);
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
  assert.match(siteReadme, /Build command: npm run build/);
  assert.match(siteReadme, /Build output directory: \.vitepress\/dist/);
  assert.match(siteReadme, /Production branch: main/);
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
  assert.match(quickstart, /glade test --project \. --class AccountServiceTest/);
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
  assert.match(supportMap, /Local LWC shell and Visualforce Lightning Out[\s\S]*Preview feature/);
  assert.match(supportMap, /## Requires Salesforce/);
  assert.match(supportMap, /Counts come from the checked standard library capability report/);
  assert.match(supportMap, /\| String, Decimal, Boolean, Math \| Runs locally \| 32 supported \/ 32 tracked \|/);
  assert.match(supportMap, /\| ApexPages and PageReference \| Supported controller rows, hosted rendering gaps \| 15 supported, 2 unsupported \/ 17 tracked \|/);
  assert.match(supportMap, /\| UserInfo, URL, Label, and TrailblazerIdentity \| Broad local capability \| 24 supported \/ 24 tracked \|/);
  assert.match(supportMap, /\| Type, FeatureManagement, and Exception \| Supported local rows, hosted package gap \| 8 supported, 1 unsupported \/ 9 tracked \|/);
  assert.match(supportMap, /\| Hosted-service and platform boundary rows \| Requires Salesforce, plus stable diagnostics \| 1 supported diagnostic row, 2 unsupported \/ 3 tracked \|/);
  assert.match(supportMap, /## Capability claims/);
  assert.match(supportMap, /\| Capability features marked `supported` \| 30 \|/);
  assert.match(supportMap, /\| Capability features marked `partial` \| 0 \|/);
  assert.match(supportMap, /\| Standard-library rows marked `unsupported` \| 19 \|/);
  assert.doesNotMatch(supportMap, /Approval\.process is not supported/);
  assert.match(installation, /Recommended path: use the one-line installer/);
  assert.match(installation, /class="docs-install-grid"/);
  assert.match(installation, /Installs the current release to <code>~\/\.local\/bin<\/code>\./);
  assert.match(installation, /Use in CI or when policy requires pinned artifacts\./);
  assert.match(css, /\.docs-install-card\s*\{[\s\S]*padding: 16px;[\s\S]*border-radius: 12px;[\s\S]*min-height: 112px;/);
  assert.match(css, /\.docs-support-legend\s*\{[\s\S]*position: static;[\s\S]*display: flex;[\s\S]*flex-wrap: wrap;[\s\S]*gap: 4px;[\s\S]*padding: 4px 6px;/);
  assert.doesNotMatch(css, /\.docs-support-legend\s*\{[\s\S]*position: sticky;/);
  assert.match(css, /\.docs-support-legend-card\s*\{[\s\S]*display: inline-flex;[\s\S]*align-items: center;[\s\S]*padding: 0;/);
  assert.match(installation, /glade test --project \. --class AccountServiceTest --method testCreatesAccount --json/);
  assert.match(cliReference, /id="cli-command-filter"/);
  assert.match(cliReference, /class="docs-command-card"/);
  assert.match(cliReference, /--class AccountServiceTest --method testCreatesAccount/);
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
  assert.match(localApiServer, /Layout and default metadata/);
  assert.match(localApiServer, /Metadata job status/);
  assert.match(localApiServer, /Bulk API v2 simple scalar query job create\/status\/whole-result CSV/);
  assert.match(localApiServer, /limits\/recordCount\?sObjects=Account/);
});

test("preview surfaces are labeled in public and repo docs", () => {
  assert.match(localTesting, /## LWC dev shell[\s\S]*preview feature/i);
  assert.match(localTesting, /\/lwc\/preview\/utility\/<UtilityBar>/);
  assert.match(localTesting, /\/lwc\/preview\/flow\/<FlowApiName>/);
  assert.match(localTesting, /\/lwc\/preview\/community\/<site>\/<page>/);
  assert.match(lwcLocalShell, /"phase3BaseComponents"/);
  assert.match(supportMap, /`lightning\/refresh`/);
  assert.match(supportMap, /packaged SLDS 2 and classic SLDS assets/);
  assert.match(localTesting, /## Visualforce dev server[\s\S]*preview feature/i);
  assert.match(index, /Visualforce and LWC local shells remain preview features\./);
  assert.match(repoReadme, /Visualforce preview feature[\s\S]*glade dev vf/);
  assert.match(repoCompatibility, /Visualforce dev rendering \| preview feature/i);
  assert.match(repoLwcSupport, /Direct component shell \| Preview feature/i);
  assert.match(repoLwcSupport, /Visualforce Lightning Out \| Preview feature/i);
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
  assert.match(cliReference, /glade inspect definition --project \. --symbol InvoiceService/);
  assert.match(cliReference, /glade inspect definition --project \. --file force-app\/main\/default\/classes\/InvoiceService\.cls --line 6 --column 13/);
  assert.match(cliReference, /glade inspect references --project \. --symbol InvoiceService\.total --json/);
  assert.match(cliReference, /glade inspect references --project \. --symbol Account\.Name --include-declaration/);
  assert.match(cliReference, /glade refactor rename --project \. --symbol InvoiceService --to BillingService --dry-run --json/);
  assert.match(cliReference, /glade refactor rename --project \. --file force-app\/main\/default\/classes\/InvoiceService\.cls --line 5 --column 14 --to totalNet --write/);
  assert.match(cliReference, /glade schema import describe --input reports\/org-describe\.json --output schema\/local\.schema\.json --project-cache \./);
  assert.match(cliReference, /writes schema symbols under `\.glade\/symbols`/);
  assert.match(editor, /glade inspect definition --project \. --symbol InvoiceService/);
  assert.match(editor, /glade inspect references --project \. --symbol Account\.Name --include-declaration/);
  assert.match(editor, /glade refactor rename --project \. --symbol InvoiceService --to BillingService --dry-run/);
  assert.match(cliReference, /\[Use Glade as an sf target\]\(\/guide\/glade-orgs\)/);
  assert.match(gladeOrgs, /glade org create my-glade-org\n```/);
  assert(gladeOrgs.indexOf("glade org create my-glade-org") < gladeOrgs.indexOf("--db .glade/orgs/my-glade-org.sqlite"));
  assert.match(index, /glade org create my-glade-org<\/code><\/pre>/);
  assert.match(gladeOrgs, /sf data import tree -p \.\/data\/insertOrder\.json -o my-glade-org/);
  assert.match(gladeOrgs, /It is not a real scratch\s+org/);
  assert.match(gladeOrgs, /Bulk API v1 CSV insert and upsert baseline/);
});

test("ci docs create reports directory before report outputs", () => {
  for (const page of [automation, localTesting, affectedTests]) {
    const mkdirIndex = page.indexOf("mkdir -p reports");
    const junitIndex = page.indexOf("glade test --project . --junit reports/glade-junit.xml");
    assert.notEqual(mkdirIndex, -1);
    assert.notEqual(junitIndex, -1);
    assert.ok(mkdirIndex < junitIndex);
  }
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
  assert.match(testerFieldGuide, /default public plugin registry is preview/);
  assert.match(ciArtifacts, /fetch-depth: 0/);
  assert.match(installation, /^## Install VS Code Extension/m);
  assert.match(installation, /glade editor doctor vscode/);
  assert.match(installation, /glade editor install vscode --force/);
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
  assert.match(plugins, /Registry-backed[\s\S]*installs are preview until a registry, archive URL, or local plugin is[\s\S]*configured\./);
  assert.match(plugins, /plugins only when you need compatibility fixtures, capability reports,[\s\S]*compatibility dashboards, or project-specific checks\./);
  assert.match(plugins, /glade plugins link --exec \.\/glade-plugin-quality/);
  assert.match(firstPartyPlugins, /install commands below[\s\S]*canonical coordinates once the registry/);
  assert.match(firstPartyPlugins, /runtime capability reports/);
  assert.match(pluginMarketplace, /The marketplace model is preview until the production registry is live/);
  assert.match(pluginInstallManage, /Direct archives and local links are the[\s\S]*fallback paths/);
  assert.match(pluginLockCi, /^# Plugin lock files and CI/m);
  assert.match(pluginLockCi, /The default public plugin registry is not live yet/);
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
