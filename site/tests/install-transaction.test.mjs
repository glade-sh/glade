import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, realpath, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));
for (const failure of ["stage", "asset swap", "binary swap", "binary validation", "interruption", "helper failure"]) test(`failed ${failure} preserves the existing installation`, async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-transaction-"));
  const host = join(root, "host"), payload = join(root, "payload"), xdg = join(root, "xdg"), home = join(root, "home");
  const version = "v9.9.9", os = platform() === "darwin" ? "darwin" : "linux", cpu = arch() === "arm64" ? "arm64" : "amd64";
  const archive = `glade_${version}_${os}_${cpu}.tar.gz`;
  await mkdir(join(host, version), { recursive: true });
  await mkdir(join(payload, "share/glade/third_party/lwc"), { recursive: true });
  await writeFile(join(payload, "glade"), failure === "binary validation" ? "#!/bin/sh\nexit 1\n" : "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  await writeFile(join(payload, "share/glade/third_party/lwc/compile.mjs"), "new\n");
  const packed = spawnSync("tar", ["-czf", join(host, version, archive), "-C", payload, "."], { encoding: "utf8" });
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(host, version, archive))).digest("hex");
  await writeFile(join(host, version, "SHA256SUMS.txt"), `${sha}  ./${archive}\n`);
  await writeFile(join(host, version, "release-manifest.json"), JSON.stringify({version}));
  const commands = join(root, "commands"), installDir = join(root, "bin"), oldAsset = join(xdg, "glade/third_party/lwc/old.mjs");
  await mkdir(commands);
  await mkdir(installDir);
  await mkdir(join(xdg, "glade/third_party/lwc"), { recursive: true });
  await writeFile(join(installDir, "glade"), "old executable\n");
  await writeFile(oldAsset, "old asset\n");
  await writeFile(join(commands, "gh"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  if (failure === "stage") await writeFile(join(commands, "cp"), "#!/bin/sh\nexit 1\n", { mode: 0o755 });
  if (failure === "asset swap" || failure === "binary swap") await writeFile(join(commands, "mv"), '#!/bin/sh\nif [ "$2" = "$FAIL_DEST" ] && [ ! -e "$FAIL_MARK" ]; then touch "$FAIL_MARK"; exit 1; fi\nexec /bin/mv "$@"\n', { mode: 0o755 });
  if (failure === "helper failure") {
    const python = spawnSync("which", ["python3"], { encoding: "utf8" }).stdout.trim();
    await writeFile(join(commands, "python3"), `#!/bin/sh\nif [ "$1" = -c ]; then printf '%s' "$$" > "$HELPER_PID"; fi\nexec "${python}" "$@"\n`, { mode: 0o755 });
    await writeFile(join(commands, "mv"), '#!/bin/sh\nif [ "$2" = "$FAIL_DEST" ]; then kill -KILL "$(cat "$HELPER_PID")"; fi\nexec /bin/mv "$@"\n', { mode: 0o755 });
  }
  if (failure === "interruption") await writeFile(join(commands, "mv"), '#!/bin/sh\n/bin/mv "$@" || exit $?\nif [ "$2" = "$FAIL_DEST" ]; then kill -KILL "$PPID"; fi\n', { mode: 0o755 });
  const env = { ...process.env, PATH: `${commands}:${process.env.PATH}`, HOME: home, XDG_DATA_HOME: xdg, GLADE_VERSION: version, GLADE_DOWNLOAD_BASE: pathToFileURL(host).href, GLADE_INSTALL_DIR: installDir, FAIL_DEST: failure === "asset swap" || failure === "interruption" || failure === "helper failure" ? join(await realpath(join(xdg, "glade")), "third_party") : join(await realpath(installDir), "glade"), FAIL_MARK: join(root, "injected"), HELPER_PID: join(root, "helper-pid") };
  delete env.GLADE_HOME;
  const result = spawnSync("sh", [join(repo, "site/install.sh")], { env, encoding: "utf8", timeout: 10000 });
  assert.notEqual(result.status, 0, result.stdout);
  if (failure === "interruption") {
    assert.equal(result.signal, "SIGKILL", result.stderr + result.stdout);
    await writeFile(join(commands, "curl"), "#!/bin/sh\nexit 22\n", { mode: 0o755 });
    await writeFile(join(commands, "mv"), "#!/bin/sh\nexec /bin/mv \"$@\"\n", { mode: 0o755 });
    const recovery = spawnSync("sh", [join(repo, "site/install.sh")], { env, encoding: "utf8", timeout: 10000 });
    assert.notEqual(recovery.status, 0, recovery.stdout);
  }
  assert.equal(await readFile(join(installDir, "glade"), "utf8"), "old executable\n");
  assert.equal(await readFile(oldAsset, "utf8"), "old asset\n");
});
