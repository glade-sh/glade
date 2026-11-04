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

test("installer never truncates an active recovery fence", async () => {
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
  await writeFile(join(commands, "sitecustomize.py"), `import os\n_orig = os.ftruncate\ndef checked(fd, length):\n try:\n  marker = os.pread(fd, 1, 0)\n except OSError:\n  marker = b""\n if marker == b"S" and length == 0:\n  raise OSError("active recovery fence was truncated")\n return _orig(fd, length)\nos.ftruncate = checked\n`);
  await writeFile(join(commands, "python3"), `#!/bin/sh\nexec "${realPython}" "$@"\n`, { mode: 0o755 });
  const env = { ...process.env, PATH: `${commands}:${process.env.PATH}`, LOCK_HELPER_STARTED: join(root, "lock-helper-started"), HOME: home, XDG_DATA_HOME: xdg, GLADE_VERSION: version, GLADE_DOWNLOAD_BASE: pathToFileURL(host).href, GLADE_INSTALL_DIR: join(root, "bin") };
  env.PYTHONPATH = commands;
  delete env.GLADE_HOME;
  const result = spawnSync("sh", [join(repo, "site/install.sh")], { env, encoding: "utf8", timeout: 20000 });
  assert.equal(result.status, 0, result.stderr);
});

for (const owner of [process.pid, 99999999]) {
  test(`binary owner fence blocks a different toolchain root (${owner === process.pid ? "live" : "stale"})`, async () => {
    const root = await mkdtemp(join(tmpdir(), "glade-owner-"));
    const bin = join(root, "bin"), xdg = join(root, "xdg"), commands = join(root, "commands");
    await mkdir(bin, { recursive: true }); await mkdir(commands);
    await writeFile(join(bin, ".glade-install.owner"), `${join(root, "other-root")}\n${owner}`);
    await writeFile(join(commands, "curl"), "#!/bin/sh\nexit 22\n", {mode: 0o755});
    const env = {...process.env, PATH: `${commands}:${process.env.PATH}`, HOME:root, XDG_DATA_HOME:xdg, GLADE_INSTALL_DIR:bin, GLADE_VERSION:"v9.9.9"};
    delete env.GLADE_HOME; delete env.GLADE_ROOT;
    const result = spawnSync("sh", [join(repo, "site/install.sh")], {env, encoding:"utf8", timeout:10000});
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /previous installer|original toolchain/);
  });
}
