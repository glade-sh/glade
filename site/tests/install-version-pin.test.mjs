import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { chmod, mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));

test("a pinned version rejects a manifest naming another version", async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-pin-"));
  const host = join(root, "host");
  const payload = join(root, "payload");
  const bin = join(root, "commands");
  const requested = "v9.0.0", redirected = "v8.0.0";
  const os = platform() === "darwin" ? "darwin" : "linux";
  const cpu = arch() === "arm64" ? "arm64" : "amd64";
  const archive = `glade_${redirected}_${os}_${cpu}.tar.gz`;
  await Promise.all([mkdir(join(host, requested), { recursive: true }), mkdir(join(host, redirected), { recursive: true }), mkdir(payload), mkdir(bin)]);
  await writeFile(join(payload, "glade"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  const packed = spawnSync("tar", ["-czf", join(host, redirected, archive), "-C", payload, "."], { encoding: "utf8" });
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(host, redirected, archive))).digest("hex");
  await writeFile(join(host, redirected, "SHA256SUMS.txt"), `${sha}  ./${archive}\n`);
  await writeFile(join(host, requested, "release-manifest.json"), JSON.stringify({version: redirected}));
  const curl = join(bin, "curl");
  await writeFile(curl, '#!/bin/sh\ncase "$*" in *file://*) exec /usr/bin/curl "$@";; *) exit 22;; esac\n');
  await chmod(curl, 0o755);
  const result = spawnSync("sh", [join(repo, "site/install.sh")], { encoding: "utf8", env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, HOME: root, XDG_DATA_HOME: join(root, "xdg"), GLADE_VERSION: requested, GLADE_DOWNLOAD_BASE: pathToFileURL(host).href, GLADE_INSTALL_DIR: join(root, "install"), GLADE_HOME: join(root, "home") } });
  assert.notEqual(result.status, 0, `installed mismatched version: ${result.stdout}`);
});
