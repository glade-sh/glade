import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmod, cp, mkdir, mkdtemp, readFile, rename, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

import { runReleaseCheck } from "./release-check.mjs";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(siteRoot, "..");

test("release check runs each site proof once and writes a source-bound summary", async () => {
  const fixture = await createFixture();
  try {
    const result = await runReleaseCheck({ siteRoot: fixture.siteRoot, summaryPath: fixture.summaryPath, quiet: true });
    assert.deepEqual(result.proofs.map((proof) => [proof.name, proof.invocations, proof.exitCode]), [
      ["verify", 1, 0],
      ["test:unit", 1, 0],
      ["build:site", 1, 0]
    ]);
    const summary = JSON.parse(await readFile(fixture.summaryPath, "utf8"));
    assert.equal(summary.source.digest, result.source.digest);
    assert.equal(summary.source.afterDigest, result.source.digest);
    assert.match(summary.source.digest, /^[a-f0-9]{64}$/);
    assert.equal(summary.siteRoot, fixture.siteRoot);
  } finally {
    await fixture.cleanup();
  }
});

test("npm release:check invokes each real proof once", async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-site-release-count-"));
  try {
    const bin = join(root, "bin");
    const log = join(root, "proofs.log");
    const wrapper = join(bin, "npm");
    const npmPath = realNpmPath();
    await mkdir(bin);
    await writeFile(wrapper, "#!/usr/bin/env sh\nif [ \"$1\" = run ]; then printf '%s\\n' \"$2\" >> \"$GLADE_RELEASE_PROOF_LOG\"; fi\nexec \"$REAL_NPM\" \"$@\"\n");
    await chmod(wrapper, 0o755);
    const result = spawnSync(npmPath, ["run", "release:check"], {
      cwd: siteRoot,
      env: {
        ...process.env,
        PATH: `${bin}:${process.env.PATH}`,
        GLADE_RELEASE_PROOF_LOG: log,
        REAL_NPM: npmPath
      },
      encoding: "utf8"
    });
    assert.equal(result.status, 0, result.stdout + result.stderr);
    const proofNames = new Set(["verify", "test:unit", "build:site"]);
    const invocations = (await readFile(log, "utf8")).trim().split("\n");
    assert.deepEqual(invocations.filter((name) => proofNames.has(name)), ["verify", "test:unit", "build:site"]);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("release check rejects a proof that mutates tracked input without a success summary", async () => {
  const fixture = await createFixture();
  try {
    const packagePath = join(fixture.siteRoot, "package.json");
    const packageJSON = JSON.parse(await readFile(packagePath, "utf8"));
    packageJSON.scripts["test:unit"] = "node scripts/mutate-release-input.mjs";
    await writeFile(packagePath, `${JSON.stringify(packageJSON, null, 2)}\n`);
    await writeFile(
      join(fixture.siteRoot, "scripts", "mutate-release-input.mjs"),
      "import { appendFile } from 'node:fs/promises'; await appendFile('docs-src/index.md', '\\nrelease-check mutation\\n');\n"
    );
    await assert.rejects(
      () => runReleaseCheck({ siteRoot: fixture.siteRoot, summaryPath: fixture.summaryPath, quiet: true }),
      /site release inputs changed while checks were running/
    );
    await assert.rejects(readFile(fixture.summaryPath, "utf8"), { code: "ENOENT" });
  } finally {
    await fixture.cleanup();
  }
});

for (const scenario of [
  ["a bad route", async ({ siteRoot }) => {
    const configPath = join(siteRoot, ".vitepress", "config.ts");
    await writeFile(configPath, `${await readFile(configPath, "utf8")}\nconst releaseCheckBadRoute = { link: '/missing-release-check-route' };\n`);
  }, "verify"],
  ["stale editor support", async ({ siteRoot }) => {
    const output = join(siteRoot, "docs-src", "public", "data", "editor-support.json");
    await writeFile(output, `${await readFile(output, "utf8")}stale\n`);
  }, "verify"],
  ["a missing help screenshot", async ({ siteRoot }) => {
    const screenshot = join(siteRoot, "docs-src", "public", "help", "screenshots", "first-local-check-01-doctor.png");
    await rename(screenshot, `${screenshot}.removed`);
  }, "verify"],
  ["a unit-test failure", async ({ siteRoot }) => {
    const packagePath = join(siteRoot, "package.json");
    const packageJSON = JSON.parse(await readFile(packagePath, "utf8"));
    packageJSON.scripts["test:unit"] = "node --test tests/release-check-intentional-failure.test.mjs";
    await writeFile(packagePath, `${JSON.stringify(packageJSON, null, 2)}\n`);
    await writeFile(join(siteRoot, "tests", "release-check-intentional-failure.test.mjs"), "import assert from 'node:assert/strict'; import { test } from 'node:test'; test('intentional failure', () => assert.equal(1, 2));\n");
  }, "test:unit"],
  ["a VitePress build failure", async ({ siteRoot }) => {
    const configPath = join(siteRoot, ".vitepress", "config.ts");
    await writeFile(configPath, `${await readFile(configPath, "utf8")}\nthrow new Error('intentional release-check build failure');\n`);
  }, "build:site"]
]) {
  const [name, tamper, expectedProof] = scenario;
  test(`release check fails for ${name}`, async () => {
    const fixture = await createFixture();
    try {
      await tamper(fixture);
      await assert.rejects(
        () => runReleaseCheck({ siteRoot: fixture.siteRoot, summaryPath: fixture.summaryPath, quiet: true }),
        new RegExp(`site release proof failed: ${expectedProof}`)
      );
    } finally {
      await fixture.cleanup();
    }
  });
}

function realNpmPath() {
  const result = spawnSync("which", ["npm"], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  return result.stdout.trim();
}

async function createFixture() {
  const root = await mkdtemp(join(tmpdir(), "glade-site-release-check-"));
  const fixtureSite = join(root, "site");
  const fixtureDocs = join(root, "docs");
  await cp(siteRoot, fixtureSite, {
    recursive: true,
    filter(source) {
      return !source.includes(`${siteRoot}/node_modules`) && !source.includes(`${siteRoot}/.vitepress/dist`) && !source.includes(`${siteRoot}/.vitepress/cache`);
    }
  });
  await cp(join(repoRoot, "docs", "STDLIB_COVERAGE.md"), join(fixtureDocs, "STDLIB_COVERAGE.md"));
  await symlink(join(siteRoot, "node_modules"), join(fixtureSite, "node_modules"));
  await replaceFixturePackage(fixtureSite);
  return { root, siteRoot: fixtureSite, summaryPath: join(root, "release-check-summary.json"), cleanup: () => rm(root, { recursive: true, force: true }) };
}

async function replaceFixturePackage(fixtureSite) {
  const packagePath = join(fixtureSite, "package.json");
  const packageJSON = JSON.parse(await readFile(packagePath, "utf8"));
  packageJSON.scripts = {
    ...packageJSON.scripts,
    verify: "npm run check:routes && npm run generate:editor-support -- --check && npm run help:check",
    "test:unit": "node --test tests/release-check-fixture.test.mjs",
    "build:site": "vitepress build . && cp install.sh .vitepress/dist/install.sh"
  };
  await writeFile(packagePath, `${JSON.stringify(packageJSON, null, 2)}\n`);
  await writeFile(join(fixtureSite, "tests", "release-check-fixture.test.mjs"), "import { test } from 'node:test'; test('fixture passes', () => {});\n");
}
