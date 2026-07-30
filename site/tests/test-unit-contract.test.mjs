import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const siteRoot = join(dirname(fileURLToPath(import.meta.url)), "..");
const packageJSON = JSON.parse(await readFile(join(siteRoot, "package.json"), "utf8"));

test("npm test remains the fast verification and unit-test entry point", () => {
  assert.equal(packageJSON.scripts.test, "npm run verify && npm run test:unit");
  assert.match(packageJSON.scripts["test:release"], /scripts\/release-check\.test\.mjs/);
});

test("release-orchestration checks do not run inside the release unit proof", () => {
  assert.equal(
    existsSync(join(siteRoot, "tests", "release-check.test.mjs")),
    false,
    "release:check must not re-enter itself through test:unit"
  );
});
