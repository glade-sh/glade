import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const home = await readFile(new URL("../.vitepress/theme/home/GladeHome.vue", import.meta.url), "utf8");
const homeScript = await readFile(new URL("../docs-src/public/js/home.js", import.meta.url), "utf8");
const quickstart = await readFile(new URL("../docs-src/guide/quickstart.md", import.meta.url), "utf8");
const examplesGuide = await readFile(new URL("../docs-src/guide/examples.md", import.meta.url), "utf8");
const helpFirstLocalCheck = await readFile(new URL("../docs-src/help/first-local-check.md", import.meta.url), "utf8");
const siteInstallation = await readFile(new URL("../docs-src/guide/installation.md", import.meta.url), "utf8");
const supportMap = await readFile(new URL("../docs-src/guide/support-map.md", import.meta.url), "utf8");
const localTesting = await readFile(new URL("../docs-src/guide/local-testing.md", import.meta.url), "utf8");
const lwcShell = await readFile(new URL("../docs-src/guide/lwc-local-shell.md", import.meta.url), "utf8");
const localAPIRoutes = await readFile(new URL("../docs-src/reference/local-api-routes.md", import.meta.url), "utf8");
const cliReference = await readFile(new URL("../docs-src/reference/cli.md", import.meta.url), "utf8");
const jsonSchema = await readFile(new URL("../docs-src/reference/json-schema.md", import.meta.url), "utf8");
const siteSecurityTrust = await readFile(new URL("../docs-src/guide/security-trust.md", import.meta.url), "utf8");
const repoInstallation = await readFile(new URL("../../docs/INSTALL.md", import.meta.url), "utf8");
const repoSecurityTrust = await readFile(new URL("../../docs/SECURITY_TRUST.md", import.meta.url), "utf8");
const securityPolicy = await readFile(new URL("../../SECURITY.md", import.meta.url), "utf8");

test('homepage install copy preserves executable command line breaks', () => {
  assert.match(home, /INSTALL_COMMAND/);
  assert.match(home, /navigator.clipboard.writeText\(INSTALL_COMMAND\)/);
  assert.match(home, /installCommand/);
  assert.match(homeScript, /return target\.getAttribute\("data-copy-text"\) \|\| target\.textContent\.trim\(\)/);
});

test("first-run docs initialize a project before doctor and execute the bundled sample test", () => {
	for (const firstRunDoc of [quickstart, helpFirstLocalCheck, repoInstallation]) {
		const commandBlocks = [...firstRunDoc.matchAll(/```bash\n([\s\S]*?)```/g)].map((match) => match[1]);
		const initBlocks = commandBlocks.map((block, index) => ({ block, index })).filter(({ block }) => block.includes("glade init --project"));
		const doctorBlocks = commandBlocks.map((block, index) => ({ block, index })).filter(({ block }) => block.includes("glade doctor"));
		assert.ok(initBlocks.length > 0, "first-run docs should initialize the project");
		for (const doctor of doctorBlocks) {
			const priorInit = initBlocks.find((init) => init.index < doctor.index);
			const sameBlockInit = initBlocks.find((init) => init.index === doctor.index);
			assert.ok(priorInit || sameBlockInit, "first-run docs should run doctor only after initialization");
			if (sameBlockInit) assert.ok(sameBlockInit.block.indexOf("glade init --project") < sameBlockInit.block.indexOf("glade doctor"));
		}
	}
	assert.match(quickstart, /Route A: Try the sample/);
	assert.match(quickstart, /Route B: Use my Salesforce DX project/);
	assert.match(quickstart, /--class SampleTest --method adds/);
	assert.match(quickstart, /--example refinement-service --open/);
	for (const stableExampleDoc of [quickstart, examplesGuide]) {
		assert.match(stableExampleDoc, /GLADE_EXAMPLE_DIR="\$\(mktemp -d\)"/);
		assert.match(stableExampleDoc, /--data-root "\$GLADE_EXAMPLE_DIR\/playground"/);
	}
	assert.doesNotMatch(quickstart, /--example refinement-service --once/);
	assert.match(quickstart, /`total` and `passed` are `1`/);
	assert.match(quickstart, /--class <YourTestClass>/);
	assert.match(quickstart, /API versions are separate contracts/);
	assert.match(quickstart, /Salesforce was not contacted|did not log in to Salesforce/);
	assert.match(quickstart, /Report a reproducible issue/);
	assert.match(quickstart, /printf '%s\\n' "\$GLADE_SAMPLE_DIR"/);
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

test("first-run surfaces keep API axes and Salesforce proof separate", () => {
	for (const doc of [quickstart, supportMap]) {
		for (const version of ["60.0", "65.0", "66.0", "67.0"]) assert.match(doc, new RegExp(version.replace(".", "\\.")));
		assert.match(doc, /historical versions are preserved|historical\s+source versions are preserved/i);
		assert.match(doc, /Do not (?:raise|upgrade|change)[\s\S]*?metadata/i);
	}
	assert.match(lwcShell, /exact checked `<apiVersion>` of `65\.0`, `66\.0`,\s+or `67\.0`/);
	assert.match(lwcShell, /does not contact Salesforce/);
	assert.match(localAPIRoutes, /accepts `60\.0`, `65\.0`, `66\.0`, and `67\.0`/);
	assert.match(localAPIRoutes, /not evidence that Salesforce was contacted/);
});

test("public references separate Unreleased first-run contracts from the stable binary", () => {
	assert.match(cliReference, /`--once` example materializer[\s\S]*Unreleased source/);
	assert.match(cliReference, /v0\.2\.15 stable binary does not provide that behavior/);
	assert.match(cliReference, /Unreleased doctor `1\.1` schema/);
	assert.match(jsonSchema, /v0\.2\.15 stable binary emits schema `1\.0`/);
	assert.doesNotMatch(quickstart, /--example refinement-service --once/);
	assert.match(localTesting, /\[`SampleTest\.adds` in the quickstart\]/);
	assert.match(cliReference, /self-contained `SampleTest\.adds` run/);
	for (const reference of [localTesting, cliReference]) {
		assert.doesNotMatch(reference, /self-contained[\s\S]{0,100}RefinementServiceTest\.createsAndLabelsFileRow/);
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
