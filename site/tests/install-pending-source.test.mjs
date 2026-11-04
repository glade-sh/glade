import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));

test("installer refuses to cross an interrupted source-toolchain transaction", async () => {
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
  await writeFile(join(commands, "gh"), `#!/bin/sh
case "$*" in
  "auth status --hostname github.com") exit 0 ;;
  *"attestation verify "*" -R glade-sh/glade "*"--signer-workflow glade-sh/glade/.github/workflows/release.yml"*"--source-ref refs/tags/v9.9.9"*) printf '%s\n' "$*" >> "$ATTESTATION_LOG" ;;
  *) exit 1 ;;
esac
`, { mode: 0o755 });
  await mkdir(join(xdg, "glade/.toolchain-stage-fixture"), { recursive: true });
  await writeFile(join(xdg, "glade/.toolchain-stage-fixture/started"), "");
  await writeFile(join(xdg, "glade/.install.lock"), "P");
  const env = { ...process.env, PATH: `${commands}:${process.env.PATH}`, ATTESTATION_LOG: join(root, "attestations"), HOME: home, XDG_DATA_HOME: xdg, GLADE_VERSION: version, GLADE_DOWNLOAD_BASE: pathToFileURL(host).href, GLADE_INSTALL_DIR: join(root, "bin") };
  delete env.GLADE_HOME;
  const result = spawnSync("sh", [join(repo, "site/install.sh")], { env, encoding: "utf8" });
  assert.notEqual(result.status, 0, result.stdout);
  assert.equal(existsSync(join(root, "bin/glade")), false);

});
