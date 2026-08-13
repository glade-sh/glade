import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const home = await readFile(new URL("../docs-src/index.md", import.meta.url), "utf8");
const homeScript = await readFile(new URL("../docs-src/public/js/home.js", import.meta.url), "utf8");
const quickstart = await readFile(new URL("../docs-src/guide/quickstart.md", import.meta.url), "utf8");
const helpFirstLocalCheck = await readFile(new URL("../docs-src/help/first-local-check.md", import.meta.url), "utf8");
const siteInstallation = await readFile(new URL("../docs-src/guide/installation.md", import.meta.url), "utf8");
const siteSecurityTrust = await readFile(new URL("../docs-src/guide/security-trust.md", import.meta.url), "utf8");
const repoInstallation = await readFile(new URL("../../docs/INSTALL.md", import.meta.url), "utf8");
const repoSecurityTrust = await readFile(new URL("../../docs/SECURITY_TRUST.md", import.meta.url), "utf8");
const securityPolicy = await readFile(new URL("../../SECURITY.md", import.meta.url), "utf8");

test("homepage install copy preserves executable command line breaks", () => {
  assert.match(
    home,
    /<code id="install-cmd"[^>]*>curl -fsSL https:\/\/glade\.sh\/install\.sh \| sh\nglade version<\/code>/
  );
  assert.match(homeScript, /return target\.getAttribute\("data-copy-text"\) \|\| target\.textContent\.trim\(\)/);
});

test("first-run docs initialize a project before doctor and avoid fixture-only test names", () => {
  for (const firstRunDoc of [quickstart, helpFirstLocalCheck, repoInstallation]) {
    const commandBlocks = [...firstRunDoc.matchAll(/```bash\n([\s\S]*?)```/g)].map((match) => match[1]);
    const initBlock = commandBlocks.findIndex((block) => block.includes("glade init --project . --yes"));
    const doctorBlock = commandBlocks.findIndex((block) => block.includes("glade doctor"));
    assert.ok(initBlock > -1, "first-run docs should initialize the project");
    assert.ok(doctorBlock === -1 || doctorBlock >= initBlock, "first-run docs should run doctor only after initialization");
    if (doctorBlock === initBlock) {
      assert.ok(commandBlocks[initBlock].indexOf("glade init --project . --yes") < commandBlocks[initBlock].indexOf("glade doctor"));
    }
    assert.doesNotMatch(firstRunDoc, /RefinementServiceTest/);
  }
  assert.match(quickstart, /glade test --project \.\n/);
  assert.match(quickstart, /--class <YourTestClass>/);
  assert.match(siteInstallation, /first local check/);
  assert.doesNotMatch(siteInstallation, /```bash\nglade doctor\n/);
});

test("installation choices link to canonical task guides", () => {
  for (const route of ["security-trust#release-proof", "editor", "workflows/ci", "build-from-source"]) {
    assert.match(siteInstallation, new RegExp(`href="/guide/${route}"`));
  }
});

test("manual verification snippets resolve the manifest and archive name without ambient variables", () => {
  for (const verificationDoc of [siteSecurityTrust, repoInstallation, repoSecurityTrust, securityPolicy]) {
    assert.match(verificationDoc, /GLADE_MANIFEST_URL=https:\/\/downloads\.glade\.sh\/latest\/release-manifest\.json/);
    assert.match(verificationDoc, /GLADE_ARCHIVE="glade_\$\{GLADE_VERSION\}_\$\{GLADE_OS\}_\$\{GLADE_ARCH\}\.tar\.gz"/);
    assert.doesNotMatch(verificationDoc, /GLADE_RELEASE_URL|GLADE_CHECKSUMS_URL/);
  }
  assert.match(siteInstallation, /security and release trust guide[^\n]*canonical/);
});

test("public trust documentation explains plugin execution boundaries", () => {
  assert.match(siteSecurityTrust, /Plugins are executables/);
  assert.match(siteSecurityTrust, /current OS user/);
  assert.match(siteSecurityTrust, /minimal environment/);
  assert.match(siteSecurityTrust, /not separately signed/);
});
