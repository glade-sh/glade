# Help Article Walkthroughs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a small set of guided Glade help articles with real VS Code screenshots, Ghostty CLI screenshots, and repeatable capture checks.

**Architecture:** Keep reference docs in the existing `/guide/...` pages and add focused guided walkthroughs under `/help/...`. Use a dedicated capture fixture and scripts so screenshots come from a clean VS Code profile with only Glade and, when needed, Salesforce extensions installed. Store screenshots as static site assets and make site tests prove each article has its required images, commands, and links.

**Tech Stack:** VitePress, Markdown, Node test runner, shell scripts, macOS Ghostty, VS Code CLI, `glade`, optional Salesforce VS Code extension pack, `screencapture`, and checked static image assets.

---

## Current State

Glade has reference-style pages under `site/docs-src/guide/`.

- `site/docs-src/guide/quickstart.md` covers first install and command flow.
- `site/docs-src/guide/local-testing.md` covers local Apex test behavior.
- `site/docs-src/guide/editor.md` covers the VS Code extension, LSP, and DAP.
- `site/docs-src/guide/tester-field-guide.md` covers first project evaluation.
- `site/.vitepress/config.ts` owns nav and sidebar entries.
- `site/tests/theme.test.mjs` checks site content, route shape, and copy contracts.

The help articles should not replace those pages. They should show the path with screenshots and exact expected outputs.

## Article Set

Create six articles.

1. `/help/first-local-check`
   - Install Glade, run `glade doctor`, initialize an SFDX project, run `glade check`.
   - Screenshot source: Ghostty.
2. `/help/run-one-apex-test`
   - Run one class from CLI, CodeLens, and Test Explorer.
   - Screenshot source: Ghostty and clean VS Code.
3. `/help/debug-apex-vscode`
   - Set a gutter breakpoint, start `Debug Local Test`, step, inspect variables, open Glade output.
   - Screenshot source: clean VS Code.
4. `/help/anonymous-apex-scratch`
   - Open the anonymous Apex scratch editor, run selected code, debug selected code, inspect active DB behavior.
   - Screenshot source: clean VS Code.
5. `/help/local-data-environments`
   - Show `dev`, clone to `feature`, switch, seed, inspect, reset, export.
   - Screenshot source: Ghostty and clean VS Code.
6. `/help/changed-tests-before-pr`
   - Run `glade test changed --since origin/main`, rerun failures, write JSON and JUnit under `reports/`.
   - Screenshot source: Ghostty.

Leave `glade debug explain` and `glade debug repro` as a later article unless the first six land clean. Six is enough timber for the first wall.

## File Structure

- Create `site/docs-src/help/index.md`
  - Landing page for the guided help article set.

- Create `site/docs-src/help/first-local-check.md`
  - Guided CLI walkthrough with Ghostty screenshots.

- Create `site/docs-src/help/run-one-apex-test.md`
  - Mixed CLI and VS Code walkthrough.

- Create `site/docs-src/help/debug-apex-vscode.md`
  - VS Code breakpoint and DAP walkthrough.

- Create `site/docs-src/help/anonymous-apex-scratch.md`
  - VS Code anonymous Apex scratch walkthrough.

- Create `site/docs-src/help/local-data-environments.md`
  - Local environment walkthrough.

- Create `site/docs-src/help/changed-tests-before-pr.md`
  - PR-ready changed-test walkthrough.

- Create `site/docs-src/public/help/screenshots/.gitkeep`
  - Keeps the screenshot asset directory present before capture.

- Create `site/docs-src/public/help/screenshots/README.md`
  - Documents screenshot naming, crop rules, and recapture expectations.

- Create `site/scripts/help-fixture/setup.mjs`
  - Creates a disposable SFDX fixture project for screenshots.

- Create `site/scripts/help-fixture/README.md`
  - Explains fixture source, generated files, and reset rules.

- Create `site/scripts/capture-help-screenshots.sh`
  - Launches clean VS Code and Ghostty capture sessions.
  - Uses isolated VS Code `--user-data-dir` and `--extensions-dir`.
  - Installs only Glade and optional Salesforce extensions.

- Create `site/scripts/check-help-screenshots.mjs`
  - Verifies every required screenshot exists, is large enough, and is referenced by exactly one help article.

- Modify `site/.vitepress/config.ts`
  - Add a `Help` nav link and sidebar section for `/help/...`.

- Modify `site/package.json`
  - Add `help:fixture`, `help:capture`, and `help:check` scripts.
  - Run `help:check` from `npm test`.

- Modify `site/tests/theme.test.mjs`
  - Add content tests for routes, article count, screenshot references, Ghostty wording, and clean VS Code extension wording.

## Screenshot Rules

- Ghostty screenshots must show a real Ghostty window.
- VS Code screenshots must come from a dedicated clean profile.
- VS Code launch must use both:

```bash
code --user-data-dir "$capture_root/vscode-user" \
  --extensions-dir "$capture_root/vscode-extensions" \
  "$fixture_root"
```

- The capture profile may install:
  - the local Glade VSIX from `contrib/vscode-glade/dist/vscode-glade-*.vsix`
  - Salesforce extensions only when an article needs familiar Salesforce language support
- The capture profile must not use the operator's default VS Code profile.
- Screenshots must not show private org names, private paths, tokens, email addresses, or unrelated extensions.
- Name screenshots with route and step numbers:

```text
site/docs-src/public/help/screenshots/debug-apex-vscode-01-breakpoint.png
site/docs-src/public/help/screenshots/debug-apex-vscode-02-debug-toolbar.png
site/docs-src/public/help/screenshots/debug-apex-vscode-03-variables.png
```

## Article Template

Each article uses this structure:

```markdown
# Title

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>One sentence stating the job.</p>
  <ul>
    <li>Outcome one.</li>
    <li>Outcome two.</li>
    <li>Outcome three.</li>
  </ul>
</div>

## Before you start

- Glade is installed and `glade doctor` passes.
- You are in an SFDX project.
- The screenshot path uses a disposable fixture.

## Steps

### 1. Do the first action

```bash
command here
```

Expected result.

![Short alt text](/help/screenshots/article-name-01-step-name.png)

## Common wrong turn

Name the problem. Show the exact fix.

## Next

- [Next article](/help/next-article)
- [Reference guide](/guide/existing-guide)
```

---

### Task 1: Pin the help article contract with failing tests

**Files:**
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add help article file reads**

Add these reads after the existing guide reads:

```js
const helpIndex = await readFile(new URL("../docs-src/help/index.md", import.meta.url), "utf8").catch(() => "");
const helpFirstLocalCheck = await readFile(new URL("../docs-src/help/first-local-check.md", import.meta.url), "utf8").catch(() => "");
const helpRunOneApexTest = await readFile(new URL("../docs-src/help/run-one-apex-test.md", import.meta.url), "utf8").catch(() => "");
const helpDebugApexVsCode = await readFile(new URL("../docs-src/help/debug-apex-vscode.md", import.meta.url), "utf8").catch(() => "");
const helpAnonymousApexScratch = await readFile(new URL("../docs-src/help/anonymous-apex-scratch.md", import.meta.url), "utf8").catch(() => "");
const helpLocalDataEnvironments = await readFile(new URL("../docs-src/help/local-data-environments.md", import.meta.url), "utf8").catch(() => "");
const helpChangedTestsBeforePr = await readFile(new URL("../docs-src/help/changed-tests-before-pr.md", import.meta.url), "utf8").catch(() => "");
const captureHelpScreenshotsScript = await readFile(new URL("../scripts/capture-help-screenshots.sh", import.meta.url), "utf8").catch(() => "");
const checkHelpScreenshotsScript = await readFile(new URL("../scripts/check-help-screenshots.mjs", import.meta.url), "utf8").catch(() => "");
```

- [ ] **Step 2: Add the route and article-count test**

Add this test near the other docs route tests:

```js
test("guided help articles are a small screenshot-backed set", () => {
  const articles = [
    helpFirstLocalCheck,
    helpRunOneApexTest,
    helpDebugApexVsCode,
    helpAnonymousApexScratch,
    helpLocalDataEnvironments,
    helpChangedTestsBeforePr
  ];

  assert.match(config, /\{ text: 'Help', link: '\/help\/' \}/);
  assert.match(config, /text: 'Guided help'/);
  assert.match(helpIndex, /^# Guided Help/m);
  assert.equal(articles.length, 6);

  for (const article of articles) {
    assert.match(article, /class="docs-intro"/);
    assert.match(article, /## Before you start/);
    assert.match(article, /## Common wrong turn/);
    assert.match(article, /## Next/);
    assert.match(article, /!\[[^\]]+\]\(\/help\/screenshots\/[a-z0-9-]+\.png\)/);
  }
});
```

- [ ] **Step 3: Add the screenshot discipline test**

Add this test after the article-count test:

```js
test("guided help screenshot capture uses Ghostty and clean VS Code profiles", () => {
  assert.match(captureHelpScreenshotsScript, /Ghostty/);
  assert.match(captureHelpScreenshotsScript, /--user-data-dir/);
  assert.match(captureHelpScreenshotsScript, /--extensions-dir/);
  assert.match(captureHelpScreenshotsScript, /vscode-glade-\$?\{?npm_package_version|vscode-glade-\*\.vsix|vscode-glade-\.\*\.vsix/);
  assert.match(captureHelpScreenshotsScript, /salesforce/);
  assert.match(captureHelpScreenshotsScript, /code --list-extensions/);
  assert.match(captureHelpScreenshotsScript, /screencapture/);
  assert.match(checkHelpScreenshotsScript, /help\/screenshots/);
  assert.match(checkHelpScreenshotsScript, /minWidth/);
  assert.match(checkHelpScreenshotsScript, /minHeight/);

  assert.match(helpFirstLocalCheck, /Ghostty/);
  assert.match(helpChangedTestsBeforePr, /Ghostty/);
  assert.match(helpDebugApexVsCode, /clean VS Code profile/);
  assert.match(helpAnonymousApexScratch, /clean VS Code profile/);
  assert.match(helpLocalDataEnvironments, /only Glade and optional Salesforce extensions/);
});
```

- [ ] **Step 4: Run the failing site test**

Run:

```bash
npm --prefix site test
```

Expected: FAIL. The help files, scripts, and config links do not exist yet.

- [ ] **Step 5: Commit if working in small commits**

```bash
git add site/tests/theme.test.mjs
git commit -m "test: pin guided help article contract"
```

### Task 2: Add the fixture and screenshot capture scripts

**Files:**
- Create: `site/scripts/help-fixture/setup.mjs`
- Create: `site/scripts/help-fixture/README.md`
- Create: `site/scripts/capture-help-screenshots.sh`
- Create: `site/scripts/check-help-screenshots.mjs`
- Modify: `site/package.json`

- [ ] **Step 1: Create the fixture setup script**

Create `site/scripts/help-fixture/setup.mjs`:

```js
#!/usr/bin/env node
import { mkdir, rm, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const repoRoot = resolve(siteRoot, "..");
const outRoot = resolve(repoRoot, ".glade/help-fixture");

await rm(outRoot, { recursive: true, force: true });
await mkdir(resolve(outRoot, "force-app/main/default/classes"), { recursive: true });
await mkdir(resolve(outRoot, "reports"), { recursive: true });

await writeFile(resolve(outRoot, "sfdx-project.json"), JSON.stringify({
  packageDirectories: [{ path: "force-app", default: true }],
  name: "glade-help-fixture",
  namespace: "",
  sourceApiVersion: "64.0"
}, null, 2) + "\n");

await writeFile(resolve(outRoot, "force-app/main/default/classes/AccountService.cls"), `public with sharing class AccountService {
    public static Account makeAccount(String name) {
        Account account = new Account(Name = name);
        insert account;
        return account;
    }

    public static Integer accountCount() {
        return [SELECT Id FROM Account].size();
    }
}
`);

await writeFile(resolve(outRoot, "force-app/main/default/classes/AccountServiceTest.cls"), `@IsTest
private class AccountServiceTest {
    @IsTest
    static void makesAccount() {
        Account account = AccountService.makeAccount('Trail account');
        System.assertNotEquals(null, account.Id);
        System.assertEquals(1, AccountService.accountCount());
    }
}
`);

await writeFile(resolve(outRoot, "seed.json"), JSON.stringify({
  records: [
    { attributes: { type: "Account" }, Name: "Seed account" }
  ]
}, null, 2) + "\n");

console.log(outRoot);
```

- [ ] **Step 2: Create the fixture README**

Create `site/scripts/help-fixture/README.md`:

```markdown
# Help Fixture

`setup.mjs` writes a disposable SFDX project to `.glade/help-fixture`.

Use it for screenshots only. Do not edit the generated fixture by hand.

```bash
npm --prefix site run help:fixture
```

The fixture contains:

- `sfdx-project.json`
- `AccountService.cls`
- `AccountServiceTest.cls`
- `seed.json`
- `reports/`
```

- [ ] **Step 3: Create the capture script**

Create `site/scripts/capture-help-screenshots.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SITE="$ROOT/site"
CAPTURE_ROOT="$ROOT/.glade/help-capture"
FIXTURE_ROOT="$ROOT/.glade/help-fixture"
SCREENSHOT_DIR="$SITE/docs-src/public/help/screenshots"
VSCODE_USER="$CAPTURE_ROOT/vscode-user"
VSCODE_EXTENSIONS="$CAPTURE_ROOT/vscode-extensions"

mkdir -p "$CAPTURE_ROOT" "$SCREENSHOT_DIR"

fixture_path="$(npm --prefix "$SITE" run --silent help:fixture)"
if [[ "$fixture_path" != "$FIXTURE_ROOT" ]]; then
  echo "unexpected fixture path: $fixture_path" >&2
  exit 1
fi

npm --prefix "$ROOT/contrib/vscode-glade" run package
rm -rf "$VSCODE_USER" "$VSCODE_EXTENSIONS"
mkdir -p "$VSCODE_USER" "$VSCODE_EXTENSIONS"

glade_vsix="$(find "$ROOT/contrib/vscode-glade/dist" -name 'vscode-glade-*.vsix' -print -quit)"
if [[ -z "$glade_vsix" ]]; then
  echo "missing Glade VSIX" >&2
  exit 1
fi

code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --install-extension "$glade_vsix"

if [[ "${INSTALL_SALESFORCE_EXTENSIONS:-0}" == "1" ]]; then
  code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --install-extension salesforce.salesforcedx-vscode
fi

echo "Installed extensions in capture profile:"
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --list-extensions

echo
echo "Opening clean VS Code profile. Only Glade and optional Salesforce extensions should appear."
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" "$FIXTURE_ROOT"

echo
echo "Opening Ghostty for CLI screenshots."
open -a Ghostty "$FIXTURE_ROOT"

cat <<EOF

Capture checklist:
  1. Use VS Code for /help screenshots that need editor UI.
  2. Use Ghostty for CLI screenshots.
  3. Save each screenshot with screencapture into:
     $SCREENSHOT_DIR

Example:
  screencapture -i "$SCREENSHOT_DIR/debug-apex-vscode-01-breakpoint.png"

Run after capture:
  npm --prefix site run help:check
EOF
```

- [ ] **Step 4: Create the screenshot checker**

Create `site/scripts/check-help-screenshots.mjs`:

```js
#!/usr/bin/env node
import { readdir, readFile, stat } from "node:fs/promises";
import { resolve } from "node:path";

const siteRoot = resolve(new URL("..", import.meta.url).pathname);
const screenshotRoot = resolve(siteRoot, "docs-src/public/help/screenshots");
const helpRoot = resolve(siteRoot, "docs-src/help");
const minWidth = 900;
const minHeight = 500;

const required = [
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
  "local-data-environments-02-ghostty.png",
  "changed-tests-before-pr-01-changed-tests.png",
  "changed-tests-before-pr-02-reports.png"
];

const articleNames = [
  "index.md",
  "first-local-check.md",
  "run-one-apex-test.md",
  "debug-apex-vscode.md",
  "anonymous-apex-scratch.md",
  "local-data-environments.md",
  "changed-tests-before-pr.md"
];

const articleText = (await Promise.all(articleNames.map(async (name) => {
  return readFile(resolve(helpRoot, name), "utf8");
}))).join("\n");

const pngFiles = new Set((await readdir(screenshotRoot)).filter((name) => name.endsWith(".png")));

for (const name of required) {
  if (!pngFiles.has(name)) {
    throw new Error(`missing required screenshot: ${name}`);
  }
  const info = await stat(resolve(screenshotRoot, name));
  if (info.size < 20_000) {
    throw new Error(`screenshot is too small to be real UI evidence: ${name}`);
  }
  const refs = articleText.match(new RegExp(`/help/screenshots/${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`, "g")) || [];
  if (refs.length !== 1) {
    throw new Error(`screenshot must be referenced exactly once: ${name} refs=${refs.length}`);
  }
}

const extra = [...pngFiles].filter((name) => !required.includes(name));
if (extra.length) {
  throw new Error(`unexpected screenshot file(s): ${extra.join(", ")}`);
}

console.log(`checked ${required.length} help screenshots; minimum display target ${minWidth}x${minHeight}`);
```

- [ ] **Step 5: Wire package scripts**

Modify `site/package.json` scripts to include:

```json
{
  "scripts": {
    "generate:editor-support": "node scripts/build-editor-support.mjs",
    "help:fixture": "node scripts/help-fixture/setup.mjs",
    "help:capture": "bash scripts/capture-help-screenshots.sh",
    "help:check": "node scripts/check-help-screenshots.mjs",
    "dev": "vitepress dev .",
    "build": "npm run generate:editor-support -- --check && npm run help:check && vitepress build . && cp install.sh .vitepress/dist/install.sh",
    "test": "npm run generate:editor-support -- --check && npm run help:check && node --test tests/*.test.mjs",
    "preview": "vitepress preview ."
  }
}
```

- [ ] **Step 6: Run the fixture script**

Run:

```bash
npm --prefix site run help:fixture
```

Expected: prints an absolute path ending in `.glade/help-fixture`.

- [ ] **Step 7: Run tests and verify the next failure**

Run:

```bash
npm --prefix site test
```

Expected: FAIL. Help article markdown and screenshots are still missing.

- [ ] **Step 8: Commit if working in small commits**

```bash
git add site/package.json site/scripts
git commit -m "chore: add help article capture harness"
```

### Task 3: Add the help routes and landing page

**Files:**
- Create: `site/docs-src/help/index.md`
- Create: `site/docs-src/public/help/screenshots/.gitkeep`
- Create: `site/docs-src/public/help/screenshots/README.md`
- Modify: `site/.vitepress/config.ts`

- [ ] **Step 1: Create the screenshot directory marker**

Create `site/docs-src/public/help/screenshots/.gitkeep` as an empty file.

- [ ] **Step 2: Create the screenshot README**

Create `site/docs-src/public/help/screenshots/README.md`:

```markdown
# Help Screenshots

These images are real UI captures for `/help/...` articles.

Rules:

- Capture CLI images in Ghostty.
- Capture editor images in a clean VS Code profile.
- Launch VS Code with `--user-data-dir` and `--extensions-dir`.
- Install only Glade and optional Salesforce extensions.
- Do not show private orgs, private paths, tokens, email addresses, or unrelated extensions.
- Run `npm --prefix site run help:check` after capture.
```

- [ ] **Step 3: Create the help landing page**

Create `site/docs-src/help/index.md`:

```markdown
# Guided Help

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Use these walkthroughs when you want the exact path, screenshots, and expected output for common Glade jobs.</p>
  <ul>
    <li>Start with a local check.</li>
    <li>Run and debug one Apex test.</li>
    <li>Use local data and changed-test loops before Salesforce.</li>
  </ul>
</div>

These articles complement the reference docs. They show the work in Ghostty and a clean VS Code profile.

## Start here

- [Install Glade and run the first local check](/help/first-local-check)
- [Run one Apex test locally](/help/run-one-apex-test)
- [Debug Apex in VS Code with breakpoints](/help/debug-apex-vscode)
- [Use anonymous Apex scratch in VS Code](/help/anonymous-apex-scratch)
- [Work with local data environments](/help/local-data-environments)
- [Run changed tests before a PR](/help/changed-tests-before-pr)

## Reference docs

- [What is Glade?](/guide/overview)
- [Quickstart](/guide/quickstart)
- [VS Code Extension, LSP, and DAP](/guide/editor)
- [Run Apex Tests Locally](/guide/local-testing)
- [What Glade runs locally](/guide/support-map)
```

- [ ] **Step 4: Add Help to VitePress nav and sidebar**

Modify `site/.vitepress/config.ts`:

```ts
nav: [
  { text: 'Capabilities', link: '/guide/support-map' },
  { text: 'VS Code', link: '/guide/editor' },
  { text: 'Playground', link: '/guide/playground' },
  { text: 'Help', link: '/help/' },
  { text: 'Docs', link: '/guide/overview' },
  { text: 'Install', link: '/guide/installation' },
  { text: 'GitHub', link: 'https://github.com/glade-sh/glade' }
],
```

Add this sidebar group after `Start`:

```ts
{
  text: 'Guided help',
  collapsed: false,
  items: [
    { text: 'Help overview', link: '/help/' },
    { text: 'First local check', link: '/help/first-local-check' },
    { text: 'Run one Apex test', link: '/help/run-one-apex-test' },
    { text: 'Debug with breakpoints', link: '/help/debug-apex-vscode' },
    { text: 'Anonymous Apex scratch', link: '/help/anonymous-apex-scratch' },
    { text: 'Local data environments', link: '/help/local-data-environments' },
    { text: 'Changed tests before a PR', link: '/help/changed-tests-before-pr' }
  ]
},
```

- [ ] **Step 5: Run tests and verify article failures remain**

Run:

```bash
npm --prefix site test
```

Expected: FAIL. The six article pages and screenshot files are still missing.

- [ ] **Step 6: Commit if working in small commits**

```bash
git add site/.vitepress/config.ts site/docs-src/help site/docs-src/public/help
git commit -m "docs: add guided help landing route"
```

### Task 4: Draft the six guided articles

**Files:**
- Create: `site/docs-src/help/first-local-check.md`
- Create: `site/docs-src/help/run-one-apex-test.md`
- Create: `site/docs-src/help/debug-apex-vscode.md`
- Create: `site/docs-src/help/anonymous-apex-scratch.md`
- Create: `site/docs-src/help/local-data-environments.md`
- Create: `site/docs-src/help/changed-tests-before-pr.md`

- [ ] **Step 1: Create first local check article**

Create `site/docs-src/help/first-local-check.md`:

```markdown
# Install Glade and Run the First Local Check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Install Glade, prove the binary, and run the first local check from Ghostty.</p>
  <ul>
    <li>Run `glade doctor`.</li>
    <li>Initialize an SFDX project.</li>
    <li>Read the first `glade check` result.</li>
  </ul>
</div>

## Before you start

- You have Ghostty installed.
- You have an SFDX project or the generated help fixture.
- Your shell can find `glade` on `PATH`.

## Steps

### 1. Prove the install

```bash
glade version
glade doctor
```

Expected: `glade doctor` ends with `Ready.`

![Ghostty showing glade doctor ready output](/help/screenshots/first-local-check-01-doctor.png)

### 2. Initialize and check the project

```bash
glade init --project . --yes
glade config validate --project .
glade check --project .
```

Expected: `glade check` exits `0` for clean source or exits `1` with file and line diagnostics.

![Ghostty showing glade check output](/help/screenshots/first-local-check-02-check.png)

## Common wrong turn

`glade: command not found` means the install directory is not on `PATH`. Add `~/.local/bin` to `PATH`, restart Ghostty, and run `glade doctor` again.

## Next

- [Run one Apex test locally](/help/run-one-apex-test)
- [Quickstart reference](/guide/quickstart)
```

- [ ] **Step 2: Create one-test article**

Create `site/docs-src/help/run-one-apex-test.md`:

```markdown
# Run One Apex Test Locally

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Run one local Apex test from Ghostty, CodeLens, and VS Code Test Explorer.</p>
  <ul>
    <li>Run one class from the CLI.</li>
    <li>Run the same test from the editor.</li>
    <li>Rerun after a failure.</li>
  </ul>
</div>

## Before you start

- `glade doctor` passes.
- The project has at least one Apex test class.
- VS Code uses a clean VS Code profile with only Glade and optional Salesforce extensions.

## Steps

### 1. Run the test class in Ghostty

```bash
glade test --project . --class AccountServiceTest --json
```

Expected: JSON output with one selected class and a pass or failure result.

![Ghostty showing one local Apex test run](/help/screenshots/run-one-apex-test-01-cli.png)

### 2. Run the test from CodeLens

Open `AccountServiceTest.cls`. Click `Run Local Test` above the method or class.

![VS Code showing Glade local test CodeLens](/help/screenshots/run-one-apex-test-02-codelens.png)

### 3. Use Test Explorer

Open VS Code Testing. Expand `Glade Apex`, select the test, and run it.

![VS Code Test Explorer showing Glade Apex test](/help/screenshots/run-one-apex-test-03-test-explorer.png)

## Common wrong turn

If the test tree is empty, confirm the folder contains `sfdx-project.json` and run `Glade: Refresh`.

## Next

- [Debug Apex in VS Code with breakpoints](/help/debug-apex-vscode)
- [Run Apex Tests Locally](/guide/local-testing)
```

- [ ] **Step 3: Create breakpoint-debug article**

Create `site/docs-src/help/debug-apex-vscode.md`:

```markdown
# Debug Apex in VS Code With Breakpoints

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Set a normal Apex gutter breakpoint and debug a local test through Glade DAP.</p>
  <ul>
    <li>Set a breakpoint in a test path.</li>
    <li>Start `Debug Local Test`.</li>
    <li>Step and inspect variables.</li>
  </ul>
</div>

## Before you start

- The Glade VS Code extension is installed in a clean VS Code profile.
- No unrelated VS Code extensions are installed in the capture profile.
- A local data environment is selected.

## Steps

### 1. Set a breakpoint

Open `AccountService.cls` or `AccountServiceTest.cls`. Click the editor gutter beside the line you want to stop on.

![VS Code showing an Apex gutter breakpoint](/help/screenshots/debug-apex-vscode-01-breakpoint.png)

### 2. Start the local debug session

Click `Debug Local Test` from CodeLens or use the Glade Debug view action.

Expected: VS Code opens a normal debug toolbar and Glade starts a DAP session.

![VS Code showing the Glade debug toolbar](/help/screenshots/debug-apex-vscode-02-debug-toolbar.png)

### 3. Inspect variables and output

Use Step Over, Variables, Call Stack, and the Glade output channel.

![VS Code showing local Apex variables during debug](/help/screenshots/debug-apex-vscode-03-variables.png)

## Common wrong turn

If debugging starts but does not stop, check that the breakpoint is in a supported `.cls` or `.trigger` file and that the test path executes that line.

## Next

- [Use anonymous Apex scratch in VS Code](/help/anonymous-apex-scratch)
- [VS Code Extension, LSP, and DAP](/guide/editor)
```

- [ ] **Step 4: Create anonymous Apex article**

Create `site/docs-src/help/anonymous-apex-scratch.md`:

```markdown
# Use Anonymous Apex Scratch in VS Code

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Open an untitled Apex scratch buffer, run it locally, and debug selected Apex.</p>
  <ul>
    <li>Open the scratch editor.</li>
    <li>Run selected code or the whole buffer.</li>
    <li>Use the active local DB.</li>
  </ul>
</div>

## Before you start

- VS Code is running with a clean VS Code profile.
- Only Glade and optional Salesforce extensions are installed.
- The active Glade local data environment is the one you want to write to.

## Steps

### 1. Open the scratch buffer

Run `Glade: Open Anonymous Apex Scratch`.

![VS Code showing an anonymous Apex scratch buffer](/help/screenshots/anonymous-apex-scratch-01-buffer.png)

### 2. Run or debug the Apex

Use `Cmd+Enter` on macOS or click the editor title play button. Select a smaller block when you want to run only part of the buffer.

Expected: Glade runs local anonymous Apex against the active DB.

![VS Code showing anonymous Apex run output](/help/screenshots/anonymous-apex-scratch-02-run.png)

## Common wrong turn

If the command says no SFDX project is open, open the project root folder, not a single `.cls` file.

## Next

- [Work with local data environments](/help/local-data-environments)
- [VS Code Extension, LSP, and DAP](/guide/editor)
```

- [ ] **Step 5: Create local environments article**

Create `site/docs-src/help/local-data-environments.md`:

```markdown
# Work With Local Data Environments

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Create, switch, seed, inspect, reset, and export local SQLite-backed Glade environments.</p>
  <ul>
    <li>Use `dev` for the default local DB.</li>
    <li>Clone an environment for a branch.</li>
    <li>Seed and inspect local data.</li>
  </ul>
</div>

## Before you start

- Glade is initialized in an SFDX project.
- VS Code uses only Glade and optional Salesforce extensions.
- These environments are local SQLite files. They do not copy Salesforce org data unless you import or seed data yourself.

## Steps

### 1. Inspect the active environment in VS Code

Open the Glade Activity Bar. Use Data Environments and Local Org to see the active DB.

![VS Code showing Glade local data environments](/help/screenshots/local-data-environments-01-sidebar.png)

### 2. Seed and inspect from Ghostty

```bash
glade db seed --db .glade/envs/dev.sqlite --project . seed.json
glade db inspect --db .glade/envs/dev.sqlite --project .
```

Expected: Glade reports local rows in the SQLite-backed environment.

![Ghostty showing local data seed and inspect output](/help/screenshots/local-data-environments-02-ghostty.png)

## Common wrong turn

`Glade: Clone Local Data Environment` copies local SQLite state. It does not contact Salesforce or refresh data from an org.

## Next

- [Run changed tests before a PR](/help/changed-tests-before-pr)
- [VS Code Extension, LSP, and DAP](/guide/editor)
```

- [ ] **Step 6: Create changed-tests article**

Create `site/docs-src/help/changed-tests-before-pr.md`:

```markdown
# Run Changed Tests Before a PR

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Run the local tests Glade can connect to your branch diff, then save machine-readable output.</p>
  <ul>
    <li>Run changed tests against `origin/main`.</li>
    <li>Rerun failures.</li>
    <li>Write reports for review or CI.</li>
  </ul>
</div>

## Before you start

- Your branch has access to `origin/main`.
- `reports/` exists before commands write files there.
- Screenshots for this article are captured in Ghostty.

## Steps

### 1. Run changed tests

```bash
glade test changed --project . --since origin/main --json --no-progress
```

Expected: Glade selects the smallest test set it can prove from the diff.

![Ghostty showing changed local tests](/help/screenshots/changed-tests-before-pr-01-changed-tests.png)

### 2. Save review artifacts

```bash
mkdir -p reports
glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
glade test --project . --junit reports/glade-junit.xml
```

Expected: `reports/` contains JSON and JUnit output for PR review or CI upload.

![Ghostty showing Glade report files](/help/screenshots/changed-tests-before-pr-02-reports.png)

## Common wrong turn

If changed-test selection finds no merge base, fetch full history first. In GitHub Actions, use `fetch-depth: 0`.

## Next

- [Add Glade to CI](/guide/ci-artifacts)
- [Affected tests](/guide/affected-tests)
```

- [ ] **Step 7: Run the site tests**

Run:

```bash
npm --prefix site test
```

Expected: FAIL until real screenshots exist. Markdown contract tests should now pass.

- [ ] **Step 8: Commit if working in small commits**

```bash
git add site/docs-src/help
git commit -m "docs: draft guided help walkthroughs"
```

### Task 5: Capture and check real screenshots

**Files:**
- Add: `site/docs-src/public/help/screenshots/*.png`

- [ ] **Step 1: Start the capture harness**

Run:

```bash
npm --prefix site run help:capture
```

Expected:

- the help fixture is recreated under `.glade/help-fixture`
- the Glade VSIX is packaged
- VS Code opens with isolated `--user-data-dir` and `--extensions-dir`
- `code --list-extensions` shows only Glade and optional Salesforce extensions
- Ghostty opens in the fixture project

- [ ] **Step 2: Capture Ghostty screenshots**

In Ghostty, run the commands from:

- `/help/first-local-check`
- `/help/run-one-apex-test`
- `/help/local-data-environments`
- `/help/changed-tests-before-pr`

Capture with:

```bash
screencapture -i site/docs-src/public/help/screenshots/first-local-check-01-doctor.png
screencapture -i site/docs-src/public/help/screenshots/first-local-check-02-check.png
screencapture -i site/docs-src/public/help/screenshots/run-one-apex-test-01-cli.png
screencapture -i site/docs-src/public/help/screenshots/local-data-environments-02-ghostty.png
screencapture -i site/docs-src/public/help/screenshots/changed-tests-before-pr-01-changed-tests.png
screencapture -i site/docs-src/public/help/screenshots/changed-tests-before-pr-02-reports.png
```

Expected: each image shows Ghostty, the command, and the relevant result.

- [ ] **Step 3: Capture VS Code screenshots**

In the clean VS Code profile, capture:

```bash
screencapture -i site/docs-src/public/help/screenshots/run-one-apex-test-02-codelens.png
screencapture -i site/docs-src/public/help/screenshots/run-one-apex-test-03-test-explorer.png
screencapture -i site/docs-src/public/help/screenshots/debug-apex-vscode-01-breakpoint.png
screencapture -i site/docs-src/public/help/screenshots/debug-apex-vscode-02-debug-toolbar.png
screencapture -i site/docs-src/public/help/screenshots/debug-apex-vscode-03-variables.png
screencapture -i site/docs-src/public/help/screenshots/anonymous-apex-scratch-01-buffer.png
screencapture -i site/docs-src/public/help/screenshots/anonymous-apex-scratch-02-run.png
screencapture -i site/docs-src/public/help/screenshots/local-data-environments-01-sidebar.png
```

Expected: each image shows only the Glade workflow surface. The Extensions view, if visible, shows only Glade and optional Salesforce extensions.

- [ ] **Step 4: Check the screenshot set**

Run:

```bash
npm --prefix site run help:check
```

Expected: PASS with `checked 14 help screenshots`.

- [ ] **Step 5: Commit if working in small commits**

```bash
git add site/docs-src/public/help/screenshots
git commit -m "docs: add real help walkthrough screenshots"
```

### Task 6: Render and verify the help pages

**Files:**
- Modify only if verification finds route, copy, or image issues.

- [ ] **Step 1: Run full site tests**

Run:

```bash
npm --prefix site test
```

Expected: PASS.

- [ ] **Step 2: Build the site**

Run:

```bash
npm --prefix site run build
```

Expected: PASS. VitePress writes `site/.vitepress/dist`.

- [ ] **Step 3: Preview the site**

Run:

```bash
npm --prefix site run preview -- --host 127.0.0.1 --port 4173
```

Expected: VitePress prints a local preview URL.

- [ ] **Step 4: Check every help route in the browser**

Open these routes:

```text
http://127.0.0.1:4173/help/
http://127.0.0.1:4173/help/first-local-check
http://127.0.0.1:4173/help/run-one-apex-test
http://127.0.0.1:4173/help/debug-apex-vscode
http://127.0.0.1:4173/help/anonymous-apex-scratch
http://127.0.0.1:4173/help/local-data-environments
http://127.0.0.1:4173/help/changed-tests-before-pr
```

Expected:

- no 404 pages
- screenshots load
- images are readable on desktop width
- no private paths, tokens, emails, or unrelated extensions appear
- Help nav and sidebar links work

- [ ] **Step 5: Run final whitespace check**

Run:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 6: Commit if working in small commits**

```bash
git add site
git commit -m "docs: publish guided help walkthroughs"
```

## Self-Review

- Spec coverage: The plan creates a handful of guided help articles, uses real VS Code screenshots, uses Ghostty for CLI screenshots, and isolates VS Code extensions to Glade plus optional Salesforce.
- Placeholder scan: No red-flag marker words or unspecified article slots remain.
- Boundary check: Work stays in public docs and site assets. No maintenance commands move into base `glade`.
- Risk check: The capture script uses real desktop apps and needs macOS UI access. If it runs headless, it should fail before claiming screenshot coverage.
