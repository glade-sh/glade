import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));

test("shell installer waits for the selected custom toolchain lock across different XDG homes", async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-xdg-"));
  const host = join(root, "host"), payload = join(root, "payload"), xdg = join(root, "xdg"), home = join(root, "home");
  const version = "v9.9.9", os = platform() === "darwin" ? "darwin" : "linux", cpu = arch() === "arm64" ? "arm64" : "amd64";
  const archive = `glade_${version}_${os}_${cpu}.tar.gz`;
  await mkdir(join(host, version), { recursive: true });
  await mkdir(join(payload, "share/glade/third_party/lwc"), { recursive: true });
  await writeFile(join(payload, "glade"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  await writeFile(join(payload, "share/glade/third_party/lwc/compile.mjs"), "new\n");
  const packed = spawnSync("tar", ["-czf", join(host, version, archive), "-C", payload, "."], { encoding: "utf8" });
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(host, version, archive))).digest("hex");
  await writeFile(join(host, version, "SHA256SUMS.txt"), `${sha}  ./${archive}\n`);
  await writeFile(join(host, version, "release-manifest.json"), JSON.stringify({version}));
  const commands = join(root, "commands");
  await mkdir(commands);
  await writeFile(join(commands, "gh"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  const realPython = spawnSync("which", ["python3"], { encoding: "utf8" }).stdout.trim();
  await writeFile(join(commands, "python3"), `#!/bin/sh\ntouch "$LOCK_HELPER_STARTED"\nexec "${realPython}" "$@"\n`, { mode: 0o755 });
  const env = { ...process.env, PATH: `${commands}:${process.env.PATH}`, LOCK_HELPER_STARTED: join(root, "lock-helper-started"), HOME: home, XDG_DATA_HOME: xdg, GLADE_VERSION: version, GLADE_DOWNLOAD_BASE: pathToFileURL(host).href, GLADE_INSTALL_DIR: join(root, "bin") };
  const share = join(root, "custom");
  env.GLADE_HOME = share;
  await mkdir(share, { recursive: true });
  const holder = spawn("python3", ["-u", "-c", "import fcntl,sys; f=open(sys.argv[1], 'w'); fcntl.lockf(f, fcntl.LOCK_EX); print('ready', flush=True); sys.stdin.read(1)", join(share, ".install.lock")], { stdio: ["pipe", "pipe", "pipe"] });
  await new Promise((resolve, reject) => { holder.stdout.once("data", resolve); holder.once("error", reject); });
  const installer = spawn("sh", [join(repo, "site/install.sh")], { env, stdio: "ignore" });
  let finished = false;
  const completion = new Promise((resolve, reject) => { installer.once("exit", code => { finished = true; resolve(code); }); installer.once("error", reject); });
  try {
    const deadline = Date.now() + 10000;
    while (!existsSync(env.LOCK_HELPER_STARTED) && !finished && Date.now() < deadline) {
      await new Promise(resolve => setTimeout(resolve, 25));
    }
    assert.ok(existsSync(env.LOCK_HELPER_STARTED), "installer did not start the shared lock helper");
    await new Promise(resolve => setTimeout(resolve, 1500));
    assert.equal(finished, false, "installer finished while an exclusive fcntl lock was held");
    assert.equal(existsSync(join(root, "bin/glade")), false, "installer wrote binary while lock was held");
  } finally {
    holder.stdin.end("x");
    await completion;
  }
});
