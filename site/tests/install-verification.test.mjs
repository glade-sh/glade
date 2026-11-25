import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const installer = fileURLToPath(new URL("../install.sh", import.meta.url));
const version = "v9.9.9";
const os = platform() === "darwin" ? "darwin" : "linux";
const cpu = arch() === "arm64" ? "arm64" : "amd64";
const archive = `glade_${version}_${os}_${cpu}.tar.gz`;

async function fixture(t) {
  const root = await mkdtemp(join(tmpdir(), "glade-verify-install-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const commands = join(root, "commands");
  const release = join(root, version);
  const payload = join(root, "payload");
  await Promise.all([commands, release, payload].map(dir => mkdir(dir)));
  const tools = new Map();
  for (const tool of ["awk", "cp", "grep", "install", "mkdir", "mkfifo", "mktemp", "mv", "python3", "rm", "sh", "sleep", "tar", "uname"]) {
    const path = tool === "python3"
      ? spawnSync(tool, ["-c", "import sys; print(sys.executable)"], { encoding: "utf8" })
      : spawnSync("which", [tool], { encoding: "utf8" });
    assert.equal(path.status, 0, tool);
    tools.set(tool, path.stdout.trim());
    await symlink(path.stdout.trim(), join(commands, tool));
  }
  const checksum = ["shasum", "sha256sum"].map(tool => [tool, spawnSync("which", [tool], { encoding: "utf8" })]).find(([, result]) => result.status === 0);
  assert.ok(checksum);
  tools.set(checksum[0], checksum[1].stdout.trim());
  await symlink(checksum[1].stdout.trim(), join(commands, checksum[0]));
  await writeFile(join(payload, "glade"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  const packed = spawnSync("tar", ["-czf", join(release, archive), "-C", payload, "."], { encoding: "utf8" });
  assert.equal(packed.status, 0, packed.stderr);
  const sha = createHash("sha256").update(await readFile(join(release, archive))).digest("hex");
  const checksums = join(release, "SHA256SUMS.txt");
  await writeFile(checksums, `${sha}  ./${archive}\n`);
  await writeFile(join(release, "release-manifest.json"), JSON.stringify({ version }));
  const curlLog = join(root, "curl.log");
  const ghLog = join(root, "gh.log");
  await writeFile(join(commands, "curl"), `#!/bin/sh
printf '%s\\n' "$*" >> "$CURL_LOG"
case "\${FAIL_DOWNLOAD:-}:$*" in
  mirror-archive:*file://*/*.tar.gz*|mirror-checksum:*file://*/SHA256SUMS.txt*)
    printf '%s\\n' 'curl: (28) mirror asset timed out' 'additional mirror details' >&2
    exit 28 ;;
esac
case "$*" in
  *release-manifest.json*)
    case "\${FAIL_DOWNLOAD:-}" in
      metadata|archive|checksum)
        printf '%s\\n' 'curl: (28) mirror timed out' 'additional mirror details' >&2
        exit 28 ;;
    esac ;;
  *api.github.com/repos/glade-sh/glade/releases/tags/*)
    case "\${FAIL_DOWNLOAD:-}" in
      archive|checksum)
        while [ "$1" != -o ]; do shift; done
        printf '%s\\n' \
          '"url": "https://api.github.com/repos/glade-sh/glade/releases/assets/1",' \
          '"name": "${archive}",' \
          '"url": "https://api.github.com/repos/glade-sh/glade/releases/assets/2",' \
          '"name": "SHA256SUMS.txt"' > "$2"
        exit 0 ;;
    esac
    printf '%s\\n' 'curl: (22) release metadata unavailable' 'additional metadata details' >&2
    exit 22 ;;
  *api.github.com/repos/glade-sh/glade/releases/assets/1*)
    if [ "$FAIL_DOWNLOAD" = checksum ]; then
      while [ "$1" != -o ]; do shift; done
      cp "$TEST_ARCHIVE" "$2"
      exit 0
    fi
    printf '%s\\n' 'curl: (56) archive connection reset' 'additional archive details' >&2
    exit 56 ;;
  *api.github.com/repos/glade-sh/glade/releases/assets/2*)
    printf '%s\\n' 'curl: (56) checksum connection reset' 'additional checksum details' >&2
    exit 56 ;;
  *api.github.com/repos/glade-sh/glade/attestations/*)
    case "$*" in *predicate_type=https%3A%2F%2Fcyclonedx.org%2Fbom*) ;; *) exit 99 ;; esac
    while [ "$1" != -o ]; do shift; done
    printf '%s' '{"attestations":[{"bundle":{"fixture":"sbom"}}]}' > "$2"
    exit 0 ;;
esac
exec /usr/bin/curl "$@"
`, { mode: 0o755 });
  const env = {
    PATH: commands,
    HOME: root,
    GLADE_VERSION: version,
    GLADE_DOWNLOAD_BASE: pathToFileURL(root).href,
    GLADE_INSTALL_DIR: join(root, "bin"),
    CURL_LOG: curlLog,
    GH_LOG: ghLog,
    TEST_ARCHIVE: join(release, archive),
  };
  return {
    run: (overrides = {}, script = installer) => spawnSync("/bin/sh", [script], { env: { ...env, ...overrides }, encoding: "utf8", timeout: 10000 }),
    root, commands, release, checksums, checksumTool: checksum[0], curlLog, ghLog, tools,
    binary: join(env.GLADE_INSTALL_DIR, "glade"),
  };
}

test("installer without gh verifies the checksum and prints one provenance notice", async t => {
  const f = await fixture(t);
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  const notices = result.stderr.split("\n").filter(line => line.startsWith("glade install: SHA-256"));
  assert.equal(notices.length, 1, result.stderr);
  assert.match(notices[0], /checksum guards against corrupted downloads/i);
  assert.match(notices[0], /provenance.*origin.*gh/i);
  assert.match(notices[0], /https:\/\/glade\.sh\/guide\/security-trust#release-proof/);
  assert.doesNotMatch(notices[0], /scripts\/verify-release-download\.sh/);
  assert.ok(existsSync(f.binary));
  assert.doesNotMatch(await readFile(f.curlLog, "utf8"), /attestations\/|sigstore\.json/);
});

test("installer without gh rejects an invalid or ambiguous checksum", async t => {
  const f = await fixture(t);
  const checksum = await readFile(f.checksums, "utf8");
  for (const invalid of [checksum.replace(/^./, "z"), checksum + checksum, `${"0".repeat(64)}  ./${archive}\n`]) {
    await writeFile(f.checksums, invalid);
    const result = f.run();
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /checksum/);
    assert.doesNotMatch(result.stderr, /glade install: SHA-256/);
    assert.equal(existsSync(f.binary), false);
  }
});

test("installer checks strict provenance and required tools before downloading", async t => {
  const f = await fixture(t);
  const strict = f.run({ GLADE_REQUIRE_PROVENANCE: "1" });
  assert.notEqual(strict.status, 0);
  assert.match(strict.stderr, /GLADE_REQUIRE_PROVENANCE=1.*gh/);
  assert.equal(existsSync(f.curlLog), false);
  for (const tool of ["python3", "mkfifo", f.checksumTool]) {
    const path = join(f.commands, tool);
    await rm(path);
    const result = f.run();
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, new RegExp(tool));
    assert.equal(existsSync(f.curlLog), false);
    await symlink(f.tools.get(tool), path);
  }
  await rm(join(f.commands, "python3"));
  await writeFile(join(f.commands, "python3"), "#!/bin/sh\nexit 1\n", { mode: 0o755 });
  const broken = f.run();
  assert.match(broken.stderr, /working Python 3/);
  assert.equal(existsSync(f.curlLog), false);
  await rm(join(f.commands, "curl"));
  const missingCurl = f.run();
  assert.match(missingCurl.stderr, /required command: curl/);
});

test("installer reports mirror curl status and reserves the private hint for token users", async t => {
  const f = await fixture(t);
  for (const token of ["", "test-token"]) {
    const result = f.run({ FAIL_DOWNLOAD: "metadata", GLADE_GITHUB_TOKEN: token });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /downloads\.glade\.sh unavailable \(curl exit 28\); trying GitHub Releases/);
    assert.match(result.stderr, /curl: \(28\) mirror timed out/);
    assert.match(result.stderr, /curl: \(22\) release metadata unavailable/);
    assert.doesNotMatch(result.stderr, /additional \w+ details/);
    assert.match(result.stderr, /check your network or proxy/i);
    assert.equal(result.stderr.includes("private repo"), Boolean(token));
  }
});

test("installer reports one line of curl diagnostics for failed GitHub assets", async t => {
  const f = await fixture(t);
  for (const asset of ["archive", "checksum"]) {
    const result = f.run({ FAIL_DOWNLOAD: asset });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, new RegExp(`curl: \\(56\\) ${asset} connection reset`));
    assert.doesNotMatch(result.stderr, /additional \w+ details/);
    assert.match(result.stderr, /check your network or proxy/i);
    assert.equal(existsSync(f.binary), false);
  }
});

test("installer reports curl diagnostics when both mirror asset paths fail", async t => {
  const f = await fixture(t);
  for (const asset of ["mirror-archive", "mirror-checksum"]) {
    const result = f.run({ FAIL_DOWNLOAD: asset });
    assert.notEqual(result.status, 0);
    assert.equal(result.stderr.match(/curl: \(28\) mirror asset timed out/g)?.length, 1, result.stderr);
    assert.doesNotMatch(result.stderr, /additional mirror details/);
    assert.equal(existsSync(f.binary), false);
  }
});

test("installer parses strict, off, and invalid provenance settings", async t => {
  const f = await fixture(t);
  for (const value of ["1", "TrUe", "YeS"]) {
    const result = f.run({ GLADE_REQUIRE_PROVENANCE: value });
    assert.notEqual(result.status, 0, value);
    assert.match(result.stderr, /GLADE_REQUIRE_PROVENANCE=.* requires the GitHub CLI \(gh\)/);
    assert.equal(existsSync(f.curlLog), false);
  }
  for (const value of ["2", "on", " true "]) {
    const result = f.run({ GLADE_REQUIRE_PROVENANCE: value });
    assert.notEqual(result.status, 0, value);
    assert.match(result.stderr, /invalid GLADE_REQUIRE_PROVENANCE value.*1, true, yes.*0, false, no/i);
    assert.equal(existsSync(f.curlLog), false);
  }
  for (const value of ["", "0", "FaLsE", "nO"]) {
    const result = f.run({ GLADE_REQUIRE_PROVENANCE: value });
    assert.equal(result.status, 0, result.stderr);
    assert.ok(existsSync(f.binary));
  }
});

test("installer rejects the macOS system Python stub before executing it", async t => {
  const f = await fixture(t);
  const python = join(f.commands, "python3");
  const xcodeSelect = join(f.commands, "xcode-select");
  const uname = join(f.commands, "uname");
  const pythonLog = join(f.root, "python.log");
  const xcodeLog = join(f.root, "xcode.log");
  await rm(python);
  await rm(uname);
  await writeFile(uname, `#!/bin/sh\ncase "$1" in -s) echo Darwin ;; -m) echo ${cpu} ;; esac\n`, { mode: 0o755 });
  await writeFile(python, '#!/bin/sh\nprintf "%s\\n" "$*" > "$PYTHON_LOG"\nexit 1\n', { mode: 0o755 });
  await writeFile(xcodeSelect, '#!/bin/sh\nprintf "%s\\n" "$*" > "$XCODE_LOG"\nexit 1\n', { mode: 0o755 });

  // Redirect the two system paths in a scratch copy to exercise the real check on any host.
  const script = join(f.root, "install.sh");
  const source = await readFile(installer, "utf8");
  await writeFile(script, source
    .replace("/usr/bin/python3 ]", `"${python}" ]`)
    .replace("/usr/bin/xcode-select", `"${xcodeSelect}"`));
  const result = f.run({ PYTHON_LOG: pythonLog, XCODE_LOG: xcodeLog }, script);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /macOS Python 3 needs the Xcode Command Line Tools.*xcode-select --install/);
  assert.equal(await readFile(xcodeLog, "utf8"), "-p\n");
  assert.equal(existsSync(pythonLog), false);
  assert.equal(existsSync(f.curlLog), false);
});

test("installer prefers the release provenance bundle and verifies the separate SBOM without login", async t => {
  const f = await fixture(t);
  await writeFile(join(f.release, `${archive}.sigstore.json`), '{"fixture":"release"}');
  await writeFile(join(f.commands, "gh"), `#!/bin/sh
printf '%s\\n' "$*" >> "$GH_LOG"
[ "\${GH_TELEMETRY:-}" = false ] || exit 1
[ "$1" = attestation ] && [ "$2" = verify ] || exit 1
case "$*:\${REJECT_PROVENANCE:-}" in
  *--predicate-type*cyclonedx*) case "$*" in *--bundle*sbom.jsonl*) exit 0;; esac ;;
  *--bundle*.sigstore.json*:0) exit 0 ;;
esac
exit 1
`, { mode: 0o755 });
  const bad = f.run({ GLADE_REQUIRE_PROVENANCE: "1", REJECT_PROVENANCE: "1" });
  assert.notEqual(bad.status, 0);
  assert.match(bad.stderr, /release provenance verification failed/);
  assert.equal(existsSync(f.binary), false);
  assert.equal((await readFile(f.ghLog, "utf8")).trim().split("\n").length, 1);
  await rm(f.ghLog);
  const good = f.run({ GLADE_REQUIRE_PROVENANCE: "1", REJECT_PROVENANCE: "0" });
  assert.equal(good.status, 0, good.stderr);
  const calls = (await readFile(f.ghLog, "utf8")).trim().split("\n");
  assert.equal(calls.length, 2);
  for (const call of calls) {
    assert.match(call, /--signer-workflow glade-sh\/glade\/\.github\/workflows\/release\.yml --source-ref refs\/tags\/v9\.9\.9/);
  }
  assert.ok(existsSync(f.binary));
});
