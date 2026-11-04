import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, realpath, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));
const installer = process.env.GLADE_TEST_INSTALLER || join(repo, "site/install.sh");
test("shared executable remains fenced across different roots after helper death", async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-fence-"));
  const host = join(root, "host"), payload = join(root, "payload"), xdg = join(root, "xdg"), bin = join(root, "bin"), commands = join(root, "commands");
  const version = "v9.9.9", os = platform() === "darwin" ? "darwin" : "linux", cpu = arch() === "arm64" ? "arm64" : "amd64";
  const archive = `glade_${version}_${os}_${cpu}.tar.gz`;
  await mkdir(join(host, version), { recursive: true });
  await mkdir(join(payload, "share/glade/third_party/lwc"), { recursive: true });
  await mkdir(join(xdg, "glade/third_party/lwc"), { recursive: true });
  await mkdir(bin); await mkdir(commands);
  await writeFile(join(bin, "glade"), "old executable\n");
  await writeFile(join(xdg, "glade/third_party/lwc/old.mjs"), "old asset\n");
  await writeFile(join(payload, "glade"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  await writeFile(join(payload, "share/glade/third_party/lwc/compile.mjs"), "new\n");
  const packed = spawnSync("tar", ["-czf", join(host, version, archive), "-C", payload, "."], { encoding: "utf8" });
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(host, version, archive))).digest("hex");
  await writeFile(join(host, version, "SHA256SUMS.txt"), `${sha}  ./${archive}\n`);
  await writeFile(join(host, version, "release-manifest.json"), JSON.stringify({version}));
  await writeFile(join(commands, "gh"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  const python = spawnSync("which", ["python3"], { encoding: "utf8" }).stdout.trim();
  await writeFile(join(commands, "python3"), `#!/bin/sh\nif [ "$1" = -c ]; then printf '%s' "$$" > "$HELPER_PID"; fi\nexec "${python}" "$@"\n`, { mode: 0o755 });
  await writeFile(join(commands, "mv"), '#!/bin/sh\nif [ "$2" = "$FAIL_DEST" ] && [ ! -e "$TRIGGER" ]; then touch "$TRIGGER"; kill -KILL "$(cat "$HELPER_PID")"; touch "$PAUSED"; while [ ! -e "$RELEASE" ]; do sleep 0.1; done; fi\nexec /bin/mv "$@"\n', { mode: 0o755 });
  const paused = join(root, "paused"), release = join(root, "release");
  const env = { ...process.env, PATH: `${commands}:${process.env.PATH}`, HOME: root, XDG_DATA_HOME: xdg, GLADE_INSTALL_DIR: bin, GLADE_VERSION: version, GLADE_DOWNLOAD_BASE: pathToFileURL(host).href, HELPER_PID: join(root, "helper"), TRIGGER: join(root, "trigger"), PAUSED: paused, RELEASE: release, FAIL_DEST: join(await realpath(join(xdg, "glade")), "third_party") };
  delete env.GLADE_HOME;
  const first = spawn("sh", [installer], { env, stdio: "ignore" });
  const completion = new Promise(resolve => first.once("exit", (code, signal) => resolve({code, signal})));
  try {
    const deadline = Date.now() + 15000;
    while (!existsSync(paused) && Date.now() < deadline) await new Promise(resolve => setTimeout(resolve, 25));
    assert.ok(existsSync(paused), "first installer never reached the interrupted swap");
    const secondEnv = { ...env, XDG_DATA_HOME: join(root, "other-xdg") };
    const second = spawnSync("sh", [installer], { env: secondEnv, encoding: "utf8", timeout: 10000 });
    assert.notEqual(second.status, 0, second.stdout);
    assert.match(second.stderr, /previous installer is still active/);
  } finally {
    await writeFile(release, "release");
    const result = await completion;
    assert.notEqual(result.code, 0);
  }
  assert.equal(await readFile(join(bin, "glade"), "utf8"), "old executable\n");
  assert.equal(await readFile(join(xdg, "glade/third_party/lwc/old.mjs"), "utf8"), "old asset\n");
});
