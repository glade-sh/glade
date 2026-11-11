import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { repoRoot } from "./helpers.mjs";
import { getIconPath } from "../src/lightning/iconUtils.mjs";

const shellDir = path.join(repoRoot, "lwcruntime/src/shell");
const source = fs.readFileSync(path.join(shellDir, "modal-overlay.mjs"), "utf8");

// Hosts serve shell modules from /lightning/runtime/shell/ alone, so the modal
// overlay may import only siblings from that directory.
test("modal overlay imports stay inside the shell directory", () => {
  const specifiers = [...source.matchAll(/^import\s[^;]*?from\s+"([^"]+)"/gm)].map(match => match[1]);
  assert.ok(specifiers.length > 0);
  for (const specifier of specifiers) {
    if (!specifier.startsWith(".")) continue;
    assert.equal(path.dirname(path.resolve(shellDir, specifier)), shellDir, specifier);
  }
});

test("modal close icon uses the utility close sprite", () => {
  const match = source.match(/setAttribute\("href",\s*([^)]+)\)/);
  assert.ok(match, "close icon href");
  const name = match[1].trim();
  const literal = source.match(new RegExp(`const ${name} = "([^"]+)"`));
  assert.ok(literal, "href is a shell-local constant");
  assert.equal(literal[1], getIconPath("utility:close"));
});
