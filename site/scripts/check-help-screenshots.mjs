#!/usr/bin/env node
import { readdir, readFile, stat } from "node:fs/promises";
import { resolve } from "node:path";

const siteRoot = resolve(new URL("..", import.meta.url).pathname);
const screenshotRoot = resolve(siteRoot, "docs-src/public/help/screenshots");
const helpRoot = resolve(siteRoot, "docs-src/help");
const captureReadmePath = resolve(screenshotRoot, "README.md");
const minWidth = 900;
const minHeight = 500;
const maxWidth = 2200;
const maxHeight = 1500;

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
  "local-data-environments-02-terminal.png",
  "changed-tests-before-pr-01-changed-tests.png",
  "changed-tests-before-pr-02-reports.png",
  "glade-org-sf-data-import-01-create-start.png",
  "glade-org-sf-data-import-02-auth-list.png",
  "glade-org-sf-data-import-03-import-query.png",
  "profile-apex-debug-log-01-profile.png",
  "profile-apex-debug-log-02-json.png",
  "ci-setup-01-workflow.png",
  "ci-setup-02-artifacts.png"
];

const articleNames = [
  "index.md",
  "first-local-check.md",
  "run-one-apex-test.md",
  "debug-apex-vscode.md",
  "anonymous-apex-scratch.md",
  "local-data-environments.md",
  "changed-tests-before-pr.md",
  "glade-org-sf-data-import.md",
  "profile-apex-debug-log.md",
  "ci-setup.md"
];

const articleText = (await Promise.all(articleNames.map(async (name) => {
  return readFile(resolve(helpRoot, name), "utf8");
}))).join("\n");
const captureReadme = await readFile(captureReadmePath, "utf8");

const pngFiles = new Set((await readdir(screenshotRoot)).filter((name) => name.endsWith(".png")));

for (const name of required) {
  if (!pngFiles.has(name)) {
    throw new Error(`missing required screenshot: ${name}`);
  }
  const filePath = resolve(screenshotRoot, name);
  const info = await stat(filePath);
  if (info.size < 20_000) {
    throw new Error(`screenshot is too small to be real UI evidence: ${name}`);
  }
  const dimensions = await pngDimensions(filePath);
  if (dimensions.width < minWidth || dimensions.height < minHeight) {
    throw new Error(`screenshot is too small: ${name} ${dimensions.width}x${dimensions.height}`);
  }
  if (dimensions.width > maxWidth || dimensions.height > maxHeight) {
    throw new Error(`screenshot is too large for the docs lane: ${name} ${dimensions.width}x${dimensions.height}`);
  }
  const refs = articleText.match(new RegExp(`/help/screenshots/${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`, "g")) || [];
  if (refs.length !== 1) {
    throw new Error(`screenshot must be referenced exactly once: ${name} refs=${refs.length}`);
  }
  if (!captureReadme.includes(`### ${name}`)) {
    throw new Error(`missing capture recipe for screenshot: ${name}`);
  }
}

const extra = [...pngFiles].filter((name) => !required.includes(name));
if (extra.length) {
  throw new Error(`unexpected screenshot file(s): ${extra.join(", ")}`);
}

console.log(`checked ${required.length} help screenshots; target range ${minWidth}x${minHeight} to ${maxWidth}x${maxHeight}`);

async function pngDimensions(filePath) {
  const handle = await readFile(filePath);
  const signature = handle.subarray(0, 8).toString("hex");
  if (signature !== "89504e470d0a1a0a") {
    throw new Error(`not a PNG file: ${filePath}`);
  }
  const chunkType = handle.subarray(12, 16).toString("ascii");
  if (chunkType !== "IHDR") {
    throw new Error(`missing PNG IHDR chunk: ${filePath}`);
  }
  return {
    width: handle.readUInt32BE(16),
    height: handle.readUInt32BE(20)
  };
}
