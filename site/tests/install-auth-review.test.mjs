import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));
for (const mode of ["anonymous", "url-bundle", "explicit-token", "logged-in", "missing-bundle"]) test(`installer verifies releases with ${mode}`, async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-auth-"));
  const host = join(root, "host"), payload = join(root, "payload"), commands = join(root, "commands");
  const version = "v9.9.9", os = platform() === "darwin" ? "darwin" : "linux", cpu = arch() === "arm64" ? "arm64" : "amd64";
  const archive = `glade_${version}_${os}_${cpu}.tar.gz`;
  await mkdir(join(host, version), { recursive: true }); await mkdir(commands); await mkdir(payload);
  await writeFile(join(payload, "glade"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  const packed = spawnSync("tar", ["-czf", join(host, version, archive), "-C", payload, "."], { encoding: "utf8" });
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(host, version, archive))).digest("hex");
  await writeFile(join(host, version, "SHA256SUMS.txt"), `${sha}  ./${archive}\n`);
  await writeFile(join(host, version, "release-manifest.json"), JSON.stringify({version}));
  const curl = spawnSync("which", ["curl"], {encoding:"utf8"}).stdout.trim();
  await writeFile(join(commands, "curl"), `#!/bin/sh\ncase "$*" in *api.github.com/repos/glade-sh/glade/attestations/*)\n case "$*" in *predicate_type=https%3A%2F%2Fslsa.dev%2Fprovenance%2Fv1*|*predicate_type=https%3A%2F%2Fcyclonedx.org%2Fbom*) ;; *) exit 1;; esac\n while [ "$#" -gt 0 ]; do if [ "$1" = -o ]; then shift; output="$1"; fi; shift; done\n if [ "$TEST_MODE" = url-bundle ]; then printf '%s' '{"attestations":[{"bundle_url":"https://storage.example.test/bundle"}]}' > "$output"; elif [ "$TEST_MODE" = missing-bundle ]; then printf '%s' '{"attestations":[]}' > "$output"; else printf '%s' '{"attestations":[{"bundle":{"fixture":true}}]}' > "$output"; fi\n exit 0;; *storage.example.test/bundle*) while [ "$#" -gt 0 ]; do if [ "$1" = -o ]; then shift; output="$1"; fi; shift; done; printf '%s' '{"fixture":true}' > "$output"; exit 0;; esac\nexec "${curl}" "$@"\n`, {mode:0o755});
  await writeFile(join(commands, "gh"), `#!/bin/sh\nif [ "$1" = auth ]; then [ "$TEST_MODE" = logged-in ]; exit $?; fi\nprintf '%s|%s\\n' "\${GH_TOKEN-}" "$*" >> "$GH_LOG"\ncase "$TEST_MODE" in explicit-token) [ "\${GH_TOKEN-}" = expected-token ] || exit 1;; anonymous|url-bundle|missing-bundle) case "$*" in *--bundle*) ;; *) exit 1;; esac;; esac\nexit 0\n`, {mode:0o755});
  const env = {...process.env, PATH:`${commands}:${process.env.PATH}`, HOME:join(root,"home"), XDG_DATA_HOME:join(root,"xdg"), GLADE_VERSION:version, GLADE_DOWNLOAD_BASE:pathToFileURL(host).href, GLADE_INSTALL_DIR:join(root,"bin"), GH_LOG:join(root,"gh.log"), TEST_MODE:mode};
  delete env.GLADE_HOME; delete env.GLADE_ROOT; delete env.GH_TOKEN; delete env.GITHUB_TOKEN; delete env.GLADE_GITHUB_TOKEN;
  if (mode === "explicit-token") env.GLADE_GITHUB_TOKEN = "expected-token";
  const result = spawnSync("sh", [join(repo, "site/install.sh")], {env, encoding:"utf8", timeout:20000});
  if (mode === "missing-bundle") { assert.notEqual(result.status, 0); return; }
  assert.equal(result.status, 0, result.stderr + result.stdout);
  const calls = (await readFile(env.GH_LOG,"utf8")).trim().split("\n");
  assert.equal(calls.length, 2);
  assert.match(calls[0], /attestation verify/); assert.match(calls[1], /cyclonedx/);
});
