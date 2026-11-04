import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));
for (const phase of ["download", "verify"]) test(`installer releases the writer lock during ${phase}`, async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-network-lock-"));
  const host = join(root, "host"), payload = join(root, "payload"), commands = join(root, "commands"), share = join(root, "xdg", "glade");
  const version = "v9.9.9", os = platform() === "darwin" ? "darwin" : "linux", cpu = arch() === "arm64" ? "arm64" : "amd64";
  const archive = `glade_${version}_${os}_${cpu}.tar.gz`;
  await mkdir(join(host, version), {recursive:true}); await mkdir(payload); await mkdir(commands); await mkdir(share, {recursive:true});
  await writeFile(join(share, ".install.lock"), "");
  await writeFile(join(share, "old-generation.txt"), "old generation");
  await writeFile(join(payload, "glade"), "#!/bin/sh\nexit 0\n", {mode:0o755});
  const packed = spawnSync("tar", ["-czf", join(host, version, archive), "-C", payload, "."], {encoding:"utf8"});
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(host, version, archive))).digest("hex");
  await writeFile(join(host, version, "SHA256SUMS.txt"), `${sha}  ./${archive}\n`);
  await writeFile(join(host, version, "release-manifest.json"), JSON.stringify({version}));
  const curl = spawnSync("which", ["curl"], {encoding:"utf8"}).stdout.trim();
  const pause = 'touch "$PAUSE_READY"; while [ ! -e "$PAUSE_RELEASE" ]; do sleep 0.05; done';
  await writeFile(join(commands, "curl"), `#!/bin/sh\ncase "$*" in *${archive}*) if [ "$PHASE" = download ]; then ${pause}; fi;; esac\nexec "${curl}" "$@"\n`, {mode:0o755});
  await writeFile(join(commands, "gh"), `#!/bin/sh\nif [ "$1" = auth ]; then exit 0; fi\nif [ "$PHASE" = verify ]; then ${pause}; fi\nexit 0\n`, {mode:0o755});
  const env = {...process.env, PATH:`${commands}:${process.env.PATH}`, HOME:join(root,"home"), XDG_DATA_HOME:join(root,"xdg"), GLADE_VERSION:version, GLADE_DOWNLOAD_BASE:pathToFileURL(host).href, GLADE_INSTALL_DIR:join(root,"bin"), PHASE:phase, PAUSE_READY:join(root,"ready"), PAUSE_RELEASE:join(root,"release")};
  delete env.GLADE_HOME; delete env.GLADE_ROOT; delete env.GH_TOKEN; delete env.GITHUB_TOKEN; delete env.GLADE_GITHUB_TOKEN;
  const child = spawn("sh", [join(repo, "site/install.sh")], {env, stdio:"ignore"});
  const ended = new Promise(resolve => child.once("exit", (code, signal) => resolve({code, signal})));
  try {
    let ready = false;
    for (let i=0; i<200; i++) { try { await readFile(env.PAUSE_READY); ready=true; break; } catch { await new Promise(r => setTimeout(r,50)); } }
    assert.equal(ready, true, "installer did not reach paused phase");
    const probe = spawnSync("python3", ["-c", `import fcntl,sys\nf=open(sys.argv[1], 'r')\ntry: fcntl.lockf(f, fcntl.LOCK_SH|fcntl.LOCK_NB); print(open(sys.argv[2]).read())\nexcept BlockingIOError: print('held')`, join(share,".install.lock"), join(share,"old-generation.txt")], {encoding:"utf8"});
    assert.equal(probe.status, 0, probe.stderr);
    assert.equal(probe.stdout.trim(), "old generation", `reader could not use the old generation during ${phase}`);
  } finally {
    await writeFile(env.PAUSE_RELEASE, "");
    const result = await ended;
    assert.equal(result.code, 0, `installer failed: ${JSON.stringify(result)}`);
  }
});
