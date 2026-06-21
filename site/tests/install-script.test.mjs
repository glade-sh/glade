import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { chmod, mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repoRoot = fileURLToPath(new URL("../../", import.meta.url));
const installScript = join(repoRoot, "site", "install.sh");

test("install script upgrades the binary and replaces only packaged toolchain directories", async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-install-e2e-"));
  const host = join(root, "host");
  const payload = join(root, "payload");
  const installDir = join(root, "bin");
  const gladeHome = join(root, "home");
  const version = "v9.9.9";
  const osName = releaseOS();
  const archName = releaseArch();
  const archive = `glade_${version}_${osName}_${archName}.tar.gz`;

  await mkdir(join(payload, "share", "glade", "third_party", "lwc"), { recursive: true });
  await mkdir(join(payload, "share", "glade", "lwcruntime", "src", "shell"), { recursive: true });
  await mkdir(join(payload, "share", "glade", "lwcruntime", "src", "slds"), { recursive: true });
  await writeFile(join(payload, "glade"), "#!/usr/bin/env sh\nif [ \"$1\" = version ]; then echo 'glade v9.9.9'; exit 0; fi\nexit 0\n", { mode: 0o755 });
  await chmod(join(payload, "glade"), 0o755);
  await writeFile(join(payload, "share", "glade", "third_party", "lwc", "compile.mjs"), "export default 'new toolchain';\n");
  await writeFile(join(payload, "share", "glade", "lwcruntime", "src", "shell", "app.mjs"), "export default 'new shell';\n");
  await writeFile(join(payload, "share", "glade", "lwcruntime", "src", "slds", "slds-loader.mjs"), "export default 'new slds';\n");

  const versionDir = join(host, version);
  await mkdir(versionDir, { recursive: true });
  run("tar", ["-czf", join(versionDir, archive), "-C", payload, "."]);
  const archiveBytes = await readFile(join(versionDir, archive));
  const archiveSHA = createHash("sha256").update(archiveBytes).digest("hex");
  await writeFile(join(versionDir, "SHA256SUMS.txt"), `${archiveSHA}  ./${archive}\n`);

  const manifest = {
    schemaVersion: 2,
    channel: "stable",
    version,
    assets: [
      {
        os: osName,
        arch: archName,
        url: `${pathToFileURL(versionDir).href}/${archive}`,
        sha256: archiveSHA
      }
    ]
  };
  await mkdir(join(host, "latest"), { recursive: true });
  await writeFile(join(host, "latest", "release-manifest.json"), JSON.stringify(manifest, null, 2));
  await writeFile(join(versionDir, "release-manifest.json"), JSON.stringify(manifest, null, 2));
  await writeFile(join(host, "index.json"), JSON.stringify({ schemaVersion: 1, latest: version }, null, 2));

  await mkdir(join(gladeHome, "plugins"), { recursive: true });
  await mkdir(join(gladeHome, "third_party"), { recursive: true });
  await mkdir(join(gladeHome, "lwcruntime"), { recursive: true });
  await writeFile(join(gladeHome, "plugins", "keep.txt"), "user plugin\n");
  await writeFile(join(gladeHome, "third_party", "stale.txt"), "old third party\n");
  await writeFile(join(gladeHome, "lwcruntime", "stale.txt"), "old runtime\n");

  const result = spawnSync("sh", [installScript], {
    cwd: repoRoot,
    env: {
      ...process.env,
      GLADE_DOWNLOAD_BASE: pathToFileURL(host).href,
      GLADE_HOME: gladeHome,
      GLADE_INSTALL_DIR: installDir
    },
    encoding: "utf8"
  });
  assert.equal(result.status, 0, result.stderr + result.stdout);
  assert.match(result.stdout, /glade v9\.9\.9/);
  assert.ok(existsSync(join(installDir, "glade")));
  assert.equal(await readFile(join(gladeHome, "plugins", "keep.txt"), "utf8"), "user plugin\n");
  assert.equal(existsSync(join(gladeHome, "third_party", "stale.txt")), false);
  assert.equal(existsSync(join(gladeHome, "lwcruntime", "stale.txt")), false);
  assert.ok(existsSync(join(gladeHome, "third_party", "lwc", "compile.mjs")));
  assert.ok(existsSync(join(gladeHome, "lwcruntime", "src", "shell", "app.mjs")));
});

function releaseOS() {
  if (platform() === "darwin") return "darwin";
  if (platform() === "linux") return "linux";
  throw new Error(`unsupported test platform: ${platform()}`);
}

function releaseArch() {
  if (arch() === "x64") return "amd64";
  if (arch() === "arm64") return "arm64";
  throw new Error(`unsupported test architecture: ${arch()}`);
}

function run(name, args) {
  const result = spawnSync(name, args, { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr + result.stdout);
}
