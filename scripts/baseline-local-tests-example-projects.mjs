#!/usr/bin/env node

import { createWriteStream, readFileSync, writeFileSync } from "node:fs";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, join, relative } from "node:path";
import { spawn } from "node:child_process";

const repoRoot = new URL("..", import.meta.url).pathname.replace(/\/$/, "");
const timeoutSeconds = Number.parseInt(process.env.OAER_BASELINE_TIMEOUT_SECONDS || "30", 10);
const timeoutMs = timeoutSeconds * 1000;
const outputPath = process.argv[2] || "docs/fixtures/local-tests-example-projects.json";

const projects = [
  "example-projects/NPSP-rel-3.237",
  "example-projects/src-nmb-nc-develop",
  "example-projects/src-nmb-nu-develop",
  "example-projects/src-nmb-nutpl-develop",
  "example-projects/sf-cred-pkg-develop",
  "example-projects/nams-workspace",
];

function normalizeMessage(message) {
  return String(message || "")
    .replace(/"[^"]+"/g, "\"<symbol>\"")
    .replace(/\b\d+\b/g, "<n>")
    .replace(/\s+/g, " ")
    .slice(0, 180);
}

function addGroup(groups, key, sample) {
  const entry = groups.get(key) || { count: 0, sample };
  entry.count += 1;
  groups.set(key, entry);
}

function topGroups(groups, limit = 8) {
  return [...groups.entries()]
    .map(([key, value]) => ({ key, count: value.count, ...value.sample }))
    .sort((a, b) => b.count - a.count || a.key.localeCompare(b.key))
    .slice(0, limit);
}

function summarizeReport(project, report, elapsedMs, command) {
  const outcomeGroups = new Map();
  for (const outcome of report.outcomes || []) {
    const file = outcome.file ? relative(repoRoot, outcome.file) : "";
    const key = [
      outcome.outcome || "unknown",
      outcome.phase || "",
      outcome.capabilityId || "",
      basename(file),
      outcome.line || "",
      normalizeMessage(outcome.error),
    ].join("|");
    addGroup(outcomeGroups, key, {
      outcome: outcome.outcome || "unknown",
      phase: outcome.phase || "",
      capabilityId: outcome.capabilityId || "",
      file,
      line: outcome.line || 0,
      error: outcome.error || "",
    });
  }

  const diagnosticGroups = new Map();
  for (const diagnostic of report.diagnostics || []) {
    const file = diagnostic.file ? relative(repoRoot, diagnostic.file) : "";
    const key = [
      diagnostic.code || "unknown",
      basename(file),
      normalizeMessage(diagnostic.message),
    ].join("|");
    addGroup(diagnosticGroups, key, {
      code: diagnostic.code || "",
      file,
      message: diagnostic.message || "",
    });
  }

  return {
    project,
    command,
    timeoutSeconds,
    timedOut: false,
    exitCode: 0,
    elapsedMs,
    ready: Boolean(report.ready),
    durationMs: report.durationMs || 0,
    summary: report.summary || {},
    topOutcomeBlockers: topGroups(outcomeGroups),
    topDiagnostics: topGroups(diagnosticGroups),
  };
}

function runProject(project) {
	const tempDir = mkdtempSync(join(tmpdir(), "oaer-local-tests-"));
	const stdoutPath = join(tempDir, "stdout.json");
	const stderrPath = join(tempDir, "stderr.txt");
	const command = `go run ./cmd/oaer compat local-tests --project ${project} --json --timeout ${timeoutMs} --top-failures 8`;
	const started = Date.now();

	return new Promise((resolve) => {
		const stdout = createWriteStream(stdoutPath);
		const stderr = createWriteStream(stderrPath);
		const child = spawn("go", [
			"run",
			"./cmd/oaer",
			"compat",
			"local-tests",
			"--project",
			project,
			"--json",
			"--timeout",
			String(timeoutMs),
			"--top-failures",
			"8",
		], { cwd: repoRoot, stdio: ["ignore", "pipe", "pipe"] });

    child.stdout.pipe(stdout);
    child.stderr.pipe(stderr);

    child.on("close", (code) => {
      stdout.end();
      stderr.end();
      const elapsedMs = Date.now() - started;
      const stderrText = readFileSync(stderrPath, "utf8").trim();
			const timedOut = code === 0 && stderrText.includes("context deadline exceeded");
      try {
        if (timedOut) {
          resolve({
            project,
            command,
            timeoutSeconds,
            timedOut: true,
            exitCode: code,
            elapsedMs,
            ready: false,
            durationMs: 0,
            summary: {
              total: 0,
              pass: 0,
              fail: 0,
              unsupported: 0,
              loadError: 0,
              compileError: 0,
              internalError: 0,
              assertFail: 0,
              timeout: 1,
            },
            topOutcomeBlockers: [{
              key: "timeout",
              count: 1,
              outcome: "timeout",
              phase: "run",
              error: `command exceeded ${timeoutSeconds}s`,
            }],
            topDiagnostics: stderrText ? [{
              key: "stderr",
              count: 1,
              code: "stderr",
              message: stderrText.slice(0, 240),
            }] : [],
          });
          return;
        }

        const report = JSON.parse(readFileSync(stdoutPath, "utf8"));
        const summary = summarizeReport(project, report, elapsedMs, command);
        summary.exitCode = code ?? 0;
        if (stderrText) {
          summary.stderr = stderrText.slice(0, 240);
        }
        resolve(summary);
      } catch (error) {
        resolve({
          project,
          command,
          timeoutSeconds,
          timedOut,
          exitCode: code ?? 1,
          elapsedMs,
          ready: false,
          durationMs: 0,
          summary: {},
          topOutcomeBlockers: [{
            key: "parse_error",
            count: 1,
            outcome: "internal_error",
            phase: "baseline",
            error: error.message,
          }],
          topDiagnostics: stderrText ? [{
            key: "stderr",
            count: 1,
            code: "stderr",
            message: stderrText.slice(0, 240),
          }] : [],
        });
      } finally {
        rmSync(tempDir, { recursive: true, force: true });
      }
    });
  });
}

const results = [];
for (const project of projects) {
  results.push(await runProject(project));
}

const artifact = {
  target: "enterprise example-project local Apex runtime baseline",
  generatedAt: new Date().toISOString(),
	timeoutSupport: {
		compatLocalTestsTimeoutFlag: true,
		command: `go run ./cmd/oaer compat local-tests --project <project> --json --timeout ${timeoutMs} --top-failures 8`,
	},
  projects: results,
};

writeFileSync(join(repoRoot, outputPath), `${JSON.stringify(artifact, null, 2)}\n`);
console.log(JSON.stringify({
  outputPath,
  projects: results.map((result) => ({
    project: result.project,
    timedOut: result.timedOut,
    elapsedMs: result.elapsedMs,
    summary: result.summary,
    topBlocker: result.topOutcomeBlockers[0] || null,
  })),
}, null, 2));
