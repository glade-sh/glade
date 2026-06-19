const assert = require("assert");
const fs = require("fs");
const path = require("path");

const source = fs.readFileSync(path.resolve(__dirname, "../src/extension.ts"), "utf8");

function commandBlock(command, nextCommand) {
  const start = source.indexOf(`vscode.commands.registerCommand("${command}"`);
  assert(start >= 0, `${command} registration must exist`);
  const end = nextCommand
    ? source.indexOf(`vscode.commands.registerCommand("${nextCommand}"`, start)
    : source.indexOf("vscode.workspace.onDidChangeConfiguration", start);
  assert(end > start, `${command} block must have an end marker`);
  return source.slice(start, end);
}

const runProof = commandBlock("glade.runLocalProof", "glade.debugTestItem");
const runProofCatch = runProof.slice(runProof.indexOf("catch (error)"));
assert(runProofCatch.includes("startHereState.setLastRun"), "failed local proof must update Home run state");
assert(runProofCatch.includes("updateHome();"), "failed local proof must refresh Home");

const seed = commandBlock("glade.seedLocalOrg", "glade.resetLocalOrg");
assert(seed.includes("startHereState.setLocalOrgSummary(undefined);"), "seed must clear stale local DB summary");
assert(seed.includes("updateHome();"), "seed must refresh Home");

const reset = commandBlock("glade.resetLocalOrg", "glade.exportLocalOrg");
assert(reset.includes("startHereState.setLocalOrgSummary(undefined);"), "reset must clear stale local DB summary");
assert(reset.includes("updateHome();"), "reset must refresh Home");

const refreshProjectStart = source.indexOf("async function refreshProject");
const refreshProjectEnd = source.indexOf("async function projectOrWarn", refreshProjectStart);
const refreshProject = source.slice(refreshProjectStart, refreshProjectEnd);
assert(refreshProject.includes("salesforceTarget = undefined;"), "project refresh must invalidate Salesforce target state");
