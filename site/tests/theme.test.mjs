import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const css = await readFile(new URL("../.vitepress/theme/custom.css", import.meta.url), "utf8");
const index = await readFile(new URL("../docs-src/index.md", import.meta.url), "utf8");
const theme = await readFile(new URL("../.vitepress/theme/index.ts", import.meta.url), "utf8");
const config = await readFile(new URL("../.vitepress/config.ts", import.meta.url), "utf8");
const highlight = await readFile(new URL("../docs-src/public/js/highlight.js", import.meta.url), "utf8");
const homeScript = await readFile(new URL("../docs-src/public/js/home.js", import.meta.url), "utf8");
const brandGuide = await readFile(new URL("../docs-src/guide/brand-guide.md", import.meta.url), "utf8");
const supportMap = await readFile(new URL("../docs-src/guide/support-map.md", import.meta.url), "utf8");
const compatibilityDashboard = await readFile(new URL("../docs-src/guide/compatibility-dashboard.md", import.meta.url), "utf8");
const logoMark = await readFile(new URL("../docs-src/public/logo-mark.svg", import.meta.url), "utf8");
const logoMarkOpen = await readFile(new URL("../docs-src/public/logo-mark-open.svg", import.meta.url), "utf8");

test("theme defines complete light and dark color tokens", () => {
  assert.match(css, /html:not\(\.dark\)\s*\{[\s\S]*--vp-c-bg:/);
  assert.match(css, /\.dark\s*\{[\s\S]*--vp-c-bg:/);
  assert.match(css, /--glade-bg-grid-line:/);
  assert.match(css, /--glade-code-bg:/);
});

test("home feature icons use registered Lucide components", () => {
  assert.match(theme, /@lucide\/vue/);
  assert.equal((index.match(/class="home-feature-icon"/g) || []).length, 4);
  assert.equal((index.match(/<Icon(?:SearchCheck|FlaskConical|SquareTerminal|ServerCog)\b/g) || []).length, 4);
  assert.doesNotMatch(index, /&#\d+;/);
});

test("home page follows the brand review conversion pass", () => {
  assert.match(index, /tagline: Build, check, test, and debug Apex workflows locally - from one Go binary\./);
  assert.match(index, /text: Install Glade/);
  assert.match(index, /text: Open Playground/);
  assert.match(index, /text: Read Docs/);
  assert.match(index, /Single Go binary · local SQLite state · macOS\/Linux · no deploy loop for fast checks/);
  assert.match(index, /<span class="home-terminal-prompt">\$<\/span>/);
  assert.match(index, /View install script/);
  assert.match(index, /Try the runtime before installing\./);
  assert.match(index, /<button class="home-run-example"/);
  assert.match(index, /data-example-id="account"/);
  assert.match(index, /data-example-id="soql"/);
  assert.match(index, /data-example-id="rollback"/);
  assert.match(index, /data-example-active="account"/);
  assert.match(index, /data-output-key="status"[\s\S]*Not run/);
  assert.match(index, /data-output-key="timing"[\s\S]*--/);
  assert.match(index, /data-output-key="log"[\s\S]*Run Example to see output/);
  assert.doesNotMatch(index, /local\.sqlite · 1 Account inserted/);
  assert.match(index, /Ready to try it locally\?/);
  assert.ok(index.indexOf("class=\"home-install\"") < index.indexOf("class=\"home-features\""));
});

test("home script delegates controls after VitePress hydration", () => {
  assert.match(homeScript, /document\.addEventListener\("click"/);
  assert.ok(homeScript.includes('target.closest("[data-run-example]")'));
  assert.ok(homeScript.includes('target.closest("[data-example-id]")'));
  assert.ok(homeScript.includes('target.closest("[data-copy-target]")'));
  assert.match(homeScript, /var examples = \{/);
  assert.match(homeScript, /setActiveExample\(exampleButton\.getAttribute\("data-example-id"\)\)/);
  assert.match(homeScript, /setOutput\(example\.idle\)/);
  assert.match(homeScript, /setOutput\(example\.result\)/);
  assert.doesNotMatch(homeScript, /if \(!root\) return/);
});

test("home code samples keep tight lines and do not wrap commands", () => {
  assert.match(highlight, /\.join\(""\)/);
  assert.match(highlight, /window\.gladeHighlightCodeBlock = highlightCodeBlock/);
  assert.doesNotMatch(highlight, /\.join\("\\n"\)/);
  assert.equal((index.match(/class="home-command-line"/g) || []).length, 10);
  assert.match(css, /\.home-command-line\s*\{[\s\S]*display: block;[\s\S]*white-space: nowrap;/);
  assert.match(css, /\.home-command-line \+ \.home-command-line\s*\{[\s\S]*padding-left: 1\.25rem;/);
  assert.doesNotMatch(css, /\.home-feature code\s*\{[\s\S]*overflow-wrap: anywhere;[\s\S]*\}/);
  assert.match(css, /\.home-code-block\s*\{[\s\S]*line-height: 1\.4;/);
  assert.match(css, /\.home-code-block code\s*\{[\s\S]*white-space: pre;/);
});

test("docs code blocks and tables fill their content lane cleanly", () => {
  assert.match(css, /\.vp-doc div\[class\*='language-'\] pre\s*\{[\s\S]*padding: 22px 24px;/);
  assert.match(css, /\.vp-doc table\s*\{[\s\S]*width: 100%;/);
  assert.match(css, /\.vp-doc th,\s*\n\.vp-doc td\s*\{[\s\S]*padding: 14px 18px;/);
  assert.match(css, /@media \(max-width: 640px\)\s*\{[\s\S]*\.vp-doc table\s*\{[\s\S]*display: block;[\s\S]*overflow-x: auto;/);
});

test("theme uses the forest clearing design direction", () => {
  assert.match(config, /Newsreader/);
  assert.match(config, /Mona\+Sans/);
  assert.match(config, /JetBrains\+Mono/);
  assert.match(css, /--vp-font-family-base: 'Mona Sans'/);
  assert.match(css, /--vp-font-family-mono: 'Monaspace Neon'[\s\S]*'JetBrains Mono'/);
  assert.match(css, /--glade-font-display: 'Newsreader'/);
  assert.match(css, /--glade-moss:/);
  assert.match(css, /--glade-lichen:/);
  assert.match(css, /--glade-bark:/);
  assert.match(css, /--glade-contour-opacity:/);
  assert.match(css, /body::before\s*\{[\s\S]*glade-contour-opacity/);
  assert.match(css, /\.VPHero \.name\s*\{[\s\S]*margin-bottom: clamp\(12px, 1\.2vw, 18px\) !important;/);
  assert.match(css, /\.VPHero \.text\s*\{[\s\S]*font-family: var\(--glade-font-display\);[\s\S]*font-style: italic;/);
  assert.match(css, /\.vp-doc h1\s*\{[\s\S]*font-family: var\(--glade-font-display\);[\s\S]*font-style: italic;/);
  assert.match(config, /The local workbench for Apex\./);
  assert.match(index, /text: "The local workbench for Apex\."/);
  assert.match(config, /Local Workbench for Salesforce Apex/);
  assert.match(config, /siteTitle: 'Glade'/);
  assert.match(config, /local Apex workbench for checking, testing, debugging, and exercising Salesforce-shaped APIs/);
  assert.doesNotMatch(index, /A clearing for Salesforce work\./);
});

test("mobile nav screen keeps a visible touch surface", () => {
  assert.match(css, /--vp-nav-screen-bg-color: var\(--glade-nav-bg\);/);
  assert.doesNotMatch(css, /\.VPNav,\s*\n\.VPNavBar\.has-sidebar \.content/);
  assert.match(css, /\.VPNavBar\.screen-open,\s*\n\.VPNavScreen\s*\{[\s\S]*background-color: var\(--glade-nav-bg\) !important;/);
  assert.match(css, /\.VPNavScreen\s*\{[\s\S]*z-index: var\(--vp-z-index-nav\);[\s\S]*backdrop-filter: blur\(16px\);[\s\S]*pointer-events: auto;/);
  assert.match(css, /\.VPNavScreen \.container\s*\{[\s\S]*background: transparent;/);
});

test("support docs summarize the checked compatibility artifacts", () => {
  assert.match(supportMap, /^# Apex and Salesforce Support/m);
  assert.match(supportMap, /## Works Well/);
  assert.match(supportMap, /## Works With Limits/);
  assert.match(supportMap, /## Not Supported Today/);
  assert.match(supportMap, /Counts come from the checked standard library coverage report/);
  assert.match(supportMap, /\| String, Decimal, Boolean, Math \| Wide local support \| 29 supported, 3 partial \/ 32 tracked \|/);
  assert.match(supportMap, /\| UserInfo, URL, and Label \| Wide local support \| 19 supported, 2 unsupported \/ 21 tracked \|/);
  assert.match(supportMap, /\| Type, FeatureManagement, Exception, and diagnostics \| Works with limits \| 6 supported, 3 partial \/ 9 tracked \|/);
  assert.match(supportMap, /\| Service-only platform APIs \| Not supported \| 35 unsupported \/ 35 tracked \|/);
  assert.match(compatibilityDashboard, /^# Developer Reports/m);
  assert.match(compatibilityDashboard, /\| Readiness \| ready \|/);
  assert.match(compatibilityDashboard, /\| Required complete \| 21\/21 \|/);
  assert.match(compatibilityDashboard, /\| Required incomplete \| 0 \|/);
  assert.match(compatibilityDashboard, /\| Required supported capabilities \| 21 \|/);
  assert.match(compatibilityDashboard, /\| Tracked post-MVP partial capabilities \| 9 \|/);
});

test("forest palette uses deep tarn blue with lichen support", () => {
  assert.doesNotMatch(css, /#37d9ff|#a7f2ff|rgba\(55,\s*217,\s*255/);
  assert.doesNotMatch(css, /#73b7bf|#93b17d|#2e7882|#5d754f/i);
  assert.match(css, /--glade-accent: #435f7c;/);
  assert.match(css, /--glade-accent: #7897b8;/);
  assert.match(css, /--glade-moss: #6e7650;/);
  assert.match(css, /--glade-moss: #b7c68f;/);
  assert.match(logoMark, /stroke="#7897B8"/);
  assert.match(logoMark, /stroke="#B7C68F"/);
  assert.match(logoMarkOpen, /stroke="#7897B8"/);
});

test("contour background is a single field instead of a visible tile", () => {
  assert.match(css, /body::before\s*\{[\s\S]*background-repeat: no-repeat;/);
  assert.match(css, /body::before\s*\{[\s\S]*background-size: max\(1450px, 120vw\) auto;/);
  assert.doesNotMatch(css, /background-size: 900px 520px;/);
});

test("brand guide page documents the settled identity", () => {
  assert.match(config, /Brand Guide/);
  assert.doesNotMatch(config, /Brand Lab|Color Lab/);
  assert.match(config, /logo-mark\.svg/);
  assert.match(config, /Newsreader/);
  assert.match(config, /Mona\+Sans/);
  assert.match(config, /IBM\+Plex\+Sans/);
  assert.match(config, /IBM\+Plex\+Mono/);
  assert.match(config, /Atkinson\+Hyperlegible\+Next/);
  assert.match(config, /Literata/);
  assert.match(config, /Source\+Serif\+4/);
  assert.match(brandGuide, /class="brand-guide"/);
  assert.match(brandGuide, /logo-mark-open\.svg/);
  assert.match(brandGuide, /The local workbench for Apex\./);
  assert.match(brandGuide, /#7897B8/);
  assert.match(brandGuide, /#B7C68F/);
  assert.match(brandGuide, /Newsreader Italic/);
  assert.match(brandGuide, /Mona Sans/);
  assert.match(brandGuide, /Monaspace Neon \/ JetBrains Mono/);
  assert.match(brandGuide, /38;2;120;151;184/);
  assert.match(brandGuide, /38;2;183;198;143/);
  assert.match(brandGuide, /Give the mark room to read as terrain, not texture/);
  assert.match(brandGuide, /glade-mark\.svg/);
  assert.match(brandGuide, /--text-primary: #EDF3F6/);
  assert.match(brandGuide, /--status-warning: #D8B36C/);
  assert.match(brandGuide, /Hero display/);
  assert.match(brandGuide, /--container-page: 1120px/);
  assert.match(brandGuide, /Primary button/);
  assert.match(brandGuide, /The grid is structure\. The contour lines are terrain\./);
  assert.match(brandGuide, /Warning and danger colors exist for product clarity/);
  assert.equal((brandGuide.match(/class="brand-color-card/g) || []).length, 9);
  assert.ok((brandGuide.match(/class="brand-rule-card/g) || []).length >= 15);
  assert.match(css, /\.brand-guide\s*\{/);
  assert.match(css, /\.brand-guide-logo-lockup\s*\{/);
  assert.match(css, /\.brand-logo-grid\s*\{/);
  assert.match(css, /\.brand-logo-card\s*\{/);
  assert.match(css, /\.brand-color-grid\s*\{/);
  assert.match(css, /\.brand-rule-grid\s*\{/);
  assert.match(css, /--glade-focus-ring: #b6cadf;/);
  assert.match(css, /--glade-status-warning: #d8b36c;/);
  assert.match(css, /:focus-visible\s*\{[\s\S]*outline: 2px solid var\(--glade-focus-ring\);/);
  assert.match(css, /\.home-run-example\s*\{/);
  assert.match(css, /\.home-terminal-prompt\s*\{/);
  assert.match(css, /--glade-font-mona-sans:/);
  assert.match(css, /--glade-font-ibm-sans:/);
  assert.match(css, /--glade-font-ibm-mono:/);
  assert.match(css, /--glade-font-atkinson:/);
  assert.match(css, /--glade-font-monaspace:/);
  assert.match(css, /\.brand-stack-recommended\s*\{/);
  assert.match(css, /\.brand-font-newsreader\s*\{/);
  assert.match(css, /\.brand-font-source-serif\s*\{/);
  assert.doesNotMatch(css, /\.brand-lab|\.color-lab|--lab-line/);
});
