const assert = require("assert");
const crypto = require("crypto");
const { execFileSync } = require("child_process");

const archive = process.argv[2];
assert(archive, "usage: node test/vsix-package.test.js <archive.vsix>");

const entries = new Set(execFileSync("unzip", ["-Z1", archive], { encoding: "utf8" }).trim().split(/\r?\n/));
for (const entry of [
  "extension/LICENSE.txt",
  "extension/NOTICE",
  "extension/out/THIRD_PARTY_NOTICES.txt",
  "extension/out/bundled-dependencies.json",
]) {
  assert(entries.has(entry), `VSIX is missing ${entry}`);
}

const license = execFileSync("unzip", ["-p", archive, "extension/LICENSE.txt"]);
assert.strictEqual(
  crypto.createHash("sha256").update(license).digest("hex"),
  "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30",
  "VSIX license is not the canonical Apache-2.0 text",
);
