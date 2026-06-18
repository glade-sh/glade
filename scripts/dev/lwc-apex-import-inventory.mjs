#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.argv[2];
if (!root) {
  throw new Error("usage: node scripts/dev/lwc-apex-import-inventory.mjs <project>");
}

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if ([".git", ".sf", ".sfdx", "node_modules"].includes(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

const imports = new Map();
for (const file of walk(root).filter((file) => /\/lwc\/.*\.js$/.test(file))) {
  const text = fs.readFileSync(file, "utf8");
  for (const match of text.matchAll(/@salesforce\/apex\/([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)/g)) {
    const key = `${match[1]}.${match[2]}`;
    imports.set(key, (imports.get(key) || 0) + 1);
  }
}

console.log(JSON.stringify([...imports.entries()].sort((a, b) => b[1] - a[1]).map(([name, count]) => ({ name, count })), null, 2));
