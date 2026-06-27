import assert from "node:assert/strict";
import { execFile as execFileCallback } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { promisify } from "node:util";
import test from "node:test";

const execFile = promisify(execFileCallback);

test("help project creates a clean git baseline for screenshots", async () => {
  const tempRoot = await mkdtemp(resolve(tmpdir(), "glade-help-project-"));
  const projectRoot = resolve(tempRoot, "macrodata-apex");
  try {
    const setup = await execFile("node", ["scripts/help-project/setup.mjs"], {
      cwd: new URL("..", import.meta.url),
      env: { ...process.env, HELP_PROJECT_ROOT: projectRoot }
    });

    assert.equal(setup.stdout.trim(), projectRoot);

    const head = await execFile("git", ["-C", projectRoot, "rev-parse", "--verify", "HEAD"]);
    assert.match(head.stdout.trim(), /^[0-9a-f]{40}$/);

    const status = await execFile("git", [
      "-C",
      projectRoot,
      "status",
      "--short",
      "--",
      "force-app/main/default/classes/RefinementService.cls"
    ]);
    assert.equal(status.stdout, "");
  } finally {
    await rm(tempRoot, { recursive: true, force: true });
  }
});
