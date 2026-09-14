import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const home = await readFile(new URL("../.vitepress/theme/home/GladeHome.vue", import.meta.url), "utf8");
const homeScript = await readFile(new URL("../docs-src/public/js/home.js", import.meta.url), "utf8");
const quickstart = await readFile(new URL("../docs-src/guide/quickstart.md", import.meta.url), "utf8");
const helpFirstLocalCheck = await readFile(new URL("../docs-src/help/first-local-check.md", import.meta.url), "utf8");
const siteInstallation = await readFile(new URL("../docs-src/guide/installation.md", import.meta.url), "utf8");
const siteSecurityTrust = await readFile(new URL("../docs-src/guide/security-trust.md", import.meta.url), "utf8");
const repoInstallation = await readFile(new URL("../../docs/INSTALL.md", import.meta.url), "utf8");
const repoSecurityTrust = await readFile(new URL("../../docs/SECURITY_TRUST.md", import.meta.url), "utf8");
const securityPolicy = await readFile(new URL("../../SECURITY.md", import.meta.url), "utf8");
const releaseVerifier = await readFile(new URL("../../scripts/verify-release-download.sh", import.meta.url), "utf8");

test('homepage install copy preserves executable command line breaks', () => {
  assert.match(home, /INSTALL_COMMAND/);
  assert.match(home, /navigator.clipboard.writeText\(INSTALL_COMMAND\)/);
  assert.match(home, /installCommand/);
  assert.match(homeScript, /return target\.getAttribute\("data-copy-text"\) \|\| target\.textContent\.trim\(\)/);
});

test("first-run docs establish sample or existing project context before doctor", () => {
  const sampleRoute = quickstart.slice(quickstart.indexOf("## Try the sample"), quickstart.indexOf("## Use my Salesforce DX project"));
  const existingProjectRoute = quickstart.slice(quickstart.indexOf("## Use my Salesforce DX project"));
  for (const firstRunDoc of [existingProjectRoute, helpFirstLocalCheck, repoInstallation]) {
    const commandBlocks = [...firstRunDoc.matchAll(/```bash\n([\s\S]*?)```/g)].map((match) => match[1]);
    const initBlock = commandBlocks.findIndex((block) => block.includes("glade init --project . --yes"));
    const doctorBlock = commandBlocks.findIndex((block) => block.includes("glade doctor"));
    assert.ok(initBlock > -1, "first-run docs should initialize the project");
    assert.ok(doctorBlock === -1 || doctorBlock >= initBlock, "first-run docs should run doctor only after initialization");
    if (doctorBlock === initBlock) {
      assert.ok(commandBlocks[initBlock].indexOf("glade init --project . --yes") < commandBlocks[initBlock].indexOf("glade doctor"));
    }
  }
  assert.match(quickstart, /glade playground .*--example refinement-service --open/);
  assert.match(sampleRoute, /glade init --project \.glade\/playground\/workspaces\/default --yes/);
  assert.ok(sampleRoute.indexOf("glade init --project") < sampleRoute.indexOf("glade doctor --project"));
  assert.match(quickstart, /glade doctor --project \.glade\/playground\/workspaces\/default/);
  assert.match(quickstart, /--class RefinementServiceTest/);
  assert.match(quickstart, /substitute your actual class name/);
  assert.match(quickstart, /zero tests/);
  assert.match(siteInstallation, /first local check/);
  assert.doesNotMatch(siteInstallation, /```bash\nglade doctor\n/);
});

test("installation choices link to canonical task guides", () => {
  for (const route of ["security-trust#release-proof", "editor", "workflows/ci", "build-from-source"]) {
    assert.match(siteInstallation, new RegExp(`href="/guide/${route}"`));
  }
});

test("installation docs disclose both destinations and the observed footprint", () => {
  for (const installationDoc of [siteInstallation, repoInstallation]) {
    assert.match(installationDoc, /GLADE_INSTALL_DIR/);
    assert.match(installationDoc, /GLADE_HOME/);
    assert.match(installationDoc, /binary, including parser support/);
    assert.match(installationDoc, /LWC runtime\/toolchain and bundled editor assets/);
    assert.match(installationDoc, /about 229 MB on disk/i);
  }
});

test("manual verification uses one fail-closed, verification-only helper", () => {
  for (const verificationDoc of [siteSecurityTrust, securityPolicy]) {
    assert.match(verificationDoc, /release-verifier:start/);
    assert.match(verificationDoc, /manifest must contain a stable vMAJOR\.MINOR\.PATCH version/);
    assert.match(verificationDoc, /Not extracted, installed, or executed/);
  }
  for (const pointerDoc of [repoInstallation, repoSecurityTrust]) {
    assert.match(pointerDoc, /sh scripts\/verify-release-download\.sh/);
  }
  assert.match(releaseVerifier, /^set -eu$/m);
  assert.doesNotMatch(releaseVerifier, /\btar\s+-|\.\/glade\s+version/);
  assert.match(siteInstallation, /security and release trust guide[^\n]*canonical/);
});

test("public trust documentation explains plugin execution boundaries", () => {
  assert.match(siteSecurityTrust, /Plugins are executables/);
  assert.match(siteSecurityTrust, /current OS user/);
  assert.match(siteSecurityTrust, /minimal environment/);
  assert.match(siteSecurityTrust, /not separately signed/);
});
