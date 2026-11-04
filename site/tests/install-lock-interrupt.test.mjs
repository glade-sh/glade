import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));
test("interrupting an installer waiting on the lock exits promptly", async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-lock-interrupt-"));
  const xdg = join(root, "xdg"), share = join(xdg, "glade"), commands = join(root, "commands"), started = join(root, "started");
  await mkdir(share, { recursive: true });
  await mkdir(commands);
  const python = spawnSync("which", ["python3"], { encoding: "utf8" }).stdout.trim();
  await writeFile(join(commands, "python3"), `#!/bin/sh\nprintf '%s' "$$" > "$LOCK_STARTED"\nexec "${python}" "$@"\n`, { mode: 0o755 });
  const holder = spawn(python, ["-u", "-c", "import fcntl,sys; f=open(sys.argv[1], 'w'); fcntl.lockf(f, fcntl.LOCK_EX); print('ready', flush=True); sys.stdin.read(1)", join(share, ".install.lock")], { stdio: ["pipe", "pipe", "pipe"] });
  await new Promise((resolve, reject) => { holder.stdout.once("data", resolve); holder.once("error", reject); });
  const env = { ...process.env, PATH: `${commands}:${process.env.PATH}`, LOCK_STARTED: started, HOME: root, XDG_DATA_HOME: xdg, GLADE_INSTALL_DIR: join(root, "bin") };
  delete env.GLADE_HOME;
  const installer = spawn("sh", [join(repo, "site/install.sh")], { env, stdio: "ignore" });
  let finished = false;
  const completion = new Promise(resolve => installer.once("exit", () => { finished = true; resolve(); }));
  try {
    const deadline = Date.now() + 10000;
    while (!existsSync(started) && !finished && Date.now() < deadline) await new Promise(resolve => setTimeout(resolve, 25));
    assert.ok(existsSync(started), "installer did not begin lock acquisition");
    installer.kill("SIGTERM");
    await Promise.race([completion, new Promise(resolve => setTimeout(resolve, 2000))]);
    assert.equal(finished, true, "installer did not exit while lock was held");
  } finally {
    holder.stdin.end("x");
    if (!finished && existsSync(started)) {
      const helperPID = Number(await readFile(started, "utf8"));
      if (Number.isInteger(helperPID) && helperPID > 1) {
        try { process.kill(helperPID, "SIGTERM"); } catch (err) { if (err.code !== "ESRCH") throw err; }
      }
    }
    await completion;
  }
});
