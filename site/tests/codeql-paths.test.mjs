import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const workflow = await readFile(new URL("../../.github/workflows/security.yml", import.meta.url), "utf8");

for (const [route, step] of [
  ["small pull request", "Initialize pull request CodeQL"],
  ["full branch and large pull request", "Initialize full-branch CodeQL"],
]) {
  test(`CodeQL ${route} config declares the generated Go path policy`, () => {
    const start = workflow.indexOf(`      - name: ${step}\n`);
    assert.notEqual(start, -1, `missing ${step}`);
    const block = workflow.slice(start).split(/\n      - name: /)[0];
    assert.match(block, /          config: \|\n            paths-ignore:\n              - '\*\*\/\*_generated\.go'\n/);
    assert.match(block, /            queries:\n              - uses: security-extended\n/);
    assert.match(block, /            query-filters:\n/);
  });
}
