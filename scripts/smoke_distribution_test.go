package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestDistributionSmokeUsesManifestArchiveBinary(t *testing.T) {
	root, runtimeLog := distributionFixture(t, false)

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("distribution smoke failed: %v\n%s", err, out)
	}
	if got := strings.Count(string(out), "distribution smoke: ok"); got != 1 {
		t.Fatalf("success marker count = %d, want 1\n%s", got, out)
	}
	invoked, err := os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	got := strings.TrimSpace(string(invoked))
	if !filepath.IsAbs(got) || filepath.Base(got) != "glade" {
		t.Fatalf("runtime received %q, want absolute extracted glade", got)
	}
	if strings.Contains(got, "decoy") {
		t.Fatalf("runtime received decoy binary: %q", got)
	}
	counts, err := os.ReadFile(filepath.Join(root, "calls.log"))
	if err != nil {
		t.Fatalf("read calls: %v", err)
	}
	if got := strings.Count(string(counts), "release\n"); got != 1 {
		t.Fatalf("release-build call count = %d, want 1", got)
	}
	if got := strings.Count(string(counts), "runtime\n"); got != 1 {
		t.Fatalf("runtime call count = %d, want 1", got)
	}
}

func TestDistributionSmokeRejectsCorruptChecksumBeforeRuntime(t *testing.T) {
	root, runtimeLog := distributionFixture(t, true)

	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("distribution smoke succeeded with corrupt checksum:\n%s", out)
	}
	if _, err := os.Stat(runtimeLog); !os.IsNotExist(err) {
		t.Fatalf("runtime was invoked before checksum rejection: %v", err)
	}
}

func TestDistributionSmokeRejectsUnsafeArchiveMembersBeforeExtraction(t *testing.T) {
	for _, kind := range []string{
		"malicious-tar",
		"malicious-zip",
		"malicious-link",
		"malicious-tar-unknown",
		"malicious-zip-socket",
	} {
		t.Run(kind, func(t *testing.T) {
			root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{archiveKind: kind})
			cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("distribution smoke accepted unsafe %s:\n%s", kind, out)
			}
			if !strings.Contains(string(out), "unsafe archive member") {
				t.Fatalf("failure did not come from member validation:\n%s", out)
			}
			if _, err := os.Stat(runtimeLog); !os.IsNotExist(err) {
				t.Fatalf("runtime was invoked before archive rejection: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "escaped")); !os.IsNotExist(err) {
				t.Fatalf("unsafe member escaped extraction root: %v", err)
			}
		})
	}
}

func TestDistributionSmokeRejectsUnsafeIndexReferencesBeforeRuntime(t *testing.T) {
	for _, reference := range []string{
		"file:///smoke/release-manifest.json",
		"https://attacker.example/smoke/release-manifest.json",
		"https://downloads.glade.sh/smoke/release-manifest.json?redirect=attacker",
		"https://downloads.glade.sh/smoke/release-manifest.json#other",
	} {
		t.Run(reference, func(t *testing.T) {
			root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{indexReference: reference})
			cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("distribution smoke accepted unsafe index reference %q:\n%s", reference, out)
			}
			if _, err := os.Stat(runtimeLog); !os.IsNotExist(err) {
				t.Fatalf("runtime was invoked before index rejection: %v", err)
			}
		})
	}
}

func TestDistributionSmokeRunsManifestSelectedZipBinary(t *testing.T) {
	root, runtimeLog := distributionFixtureWithOptions(t, distributionFixtureOptions{archiveKind: "zip"})
	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zip distribution smoke failed: %v\n%s", err, out)
	}
	invoked, err := os.ReadFile(runtimeLog)
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	if got := filepath.Base(strings.TrimSpace(string(invoked))); got != "glade.exe" {
		t.Fatalf("runtime received %q, want glade.exe", got)
	}
}

func TestDistributionSmokeRejectsRuntimeBinaryMutation(t *testing.T) {
	root, _ := distributionFixtureWithOptions(t, distributionFixtureOptions{mutateRuntimeBinary: true})
	cmd := exec.Command(filepath.Join(root, "scripts", "smoke-distribution.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("distribution smoke accepted mutated binary:\n%s", out)
	}
	if !strings.Contains(string(out), "release binary changed during runtime smoke") {
		t.Fatalf("unexpected mutation failure:\n%s", out)
	}
}

func TestDistributionSmokeStructure(t *testing.T) {
	script := readSmokeScript(t, "smoke-distribution.sh")
	if got := strings.Count(script, `"${ROOT}/scripts/release-build.sh"`); got != 1 {
		t.Errorf("release-build invocation count = %d, want 1", got)
	}
	if got := strings.Count(script, `"${ROOT}/scripts/smoke-runtime.sh" "${GLADE}"`); got != 1 {
		t.Errorf("runtime invocation count = %d, want 1", got)
	}
	if strings.Contains(script, "go build") {
		t.Error("distribution smoke must not build directly")
	}
	for _, want := range []string{
		`manifest_asset.get("url")`,
		"urlparse",
		"archive_name",
		"manifest_asset",
		"validate_archive_members",
		`root manifest version is invalid`,
		`dist / version / "release-manifest.json"`,
		`index.get("latest") == version`,
		`entry.get("version") == version`,
		`f"https://downloads.glade.sh/{version}/release-manifest.json"`,
		`"${GLADE}" doctor --json`,
		`release binary doctor parser verification failed`,
		"binary.lstat()",
		"resolved.relative_to(root)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("distribution smoke missing manifest-selection marker %q", want)
		}
	}
}

type distributionFixtureOptions struct {
	corruptChecksum     bool
	archiveKind         string
	indexReference      string
	mutateRuntimeBinary bool
	version             string
	doctorFails         bool
	doctorOutput        string
	doctorExitCode      int
}

func distributionFixture(t *testing.T, corruptChecksum bool) (string, string) {
	return distributionFixtureWithOptions(t, distributionFixtureOptions{corruptChecksum: corruptChecksum})
}

func distributionFixtureWithOptions(t *testing.T, options distributionFixtureOptions) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires a Unix host")
	}
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	subject, err := os.ReadFile("smoke-distribution.sh")
	if err != nil {
		t.Fatalf("read subject: %v", err)
	}
	writeExecutable(t, filepath.Join(scripts, "smoke-distribution.sh"), string(subject))
	runtimeScript := `#!/usr/bin/env bash
set -euo pipefail
printf 'runtime\n' >>"$(dirname "${BASH_SOURCE[0]}")/../calls.log"
printf '%s\n' "$1" >"$(dirname "${BASH_SOURCE[0]}")/../runtime.log"
"$1" version >/dev/null
`
	if options.mutateRuntimeBinary {
		runtimeScript += "printf '# mutation\\n' >>\"$1\"\n"
	}
	writeExecutable(t, filepath.Join(scripts, "smoke-runtime.sh"), runtimeScript)
	corrupt := "0"
	if options.corruptChecksum {
		corrupt = "1"
	}
	archiveKind := options.archiveKind
	if archiveKind == "" {
		archiveKind = "tar"
	}
	version := options.version
	if version == "" {
		version = "smoke"
	}
	indexReference := options.indexReference
	if indexReference == "" {
		indexReference = "https://downloads.glade.sh/" + version + "/release-manifest.json"
	}
	doctorOutput := `{"status":"passed","exitCode":0,"parserOK":true}`
	if options.doctorFails {
		doctorOutput = `{"status":"failed","exitCode":1,"parserOK":false}`
	}
	if options.doctorOutput != "" {
		doctorOutput = options.doctorOutput
	}
	writeExecutable(t, filepath.Join(scripts, "release-build.sh"), `#!/usr/bin/env bash
set -euo pipefail
printf 'release\n' >>"$(dirname "${BASH_SOURCE[0]}")/../calls.log"
mkdir -p "${DIST_DIR}/latest" "${DIST_DIR}/${VERSION}"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
cat >"${work}/glade" <<'BIN'
#!/usr/bin/env bash
if [[ "${1:-}" == version ]]; then
  printf 'glade smoke fixture\n'
elif [[ "${1:-}" == doctor && "${2:-}" == --json ]]; then
  printf '`+doctorOutput+`\n'
  exit `+strconv.Itoa(options.doctorExitCode)+`
else
  exit 9
fi
BIN
chmod +x "${work}/glade"
kind='`+archiveKind+`'
case "${kind}" in
tar)
  archive=glade_smoke_fixture.tar.gz; asset_os=darwin
  tar -C "${work}" -czf "${DIST_DIR}/${archive}" glade
  ;;
zip)
  archive=glade_smoke_fixture.zip; asset_os=windows
  mv "${work}/glade" "${work}/glade.exe"
  (cd "${work}" && zip -q "${DIST_DIR}/${archive}" glade.exe)
  ;;
malicious-tar)
  archive=glade_smoke_fixture.tar.gz; asset_os=darwin
  export MALICIOUS_ARCHIVE="${DIST_DIR}/${archive}" MALICIOUS_BINARY="${work}/glade"
  python3 - <<'PY'
import io, os, tarfile
with tarfile.open(os.environ['MALICIOUS_ARCHIVE'], 'w:gz') as archive:
 archive.add(os.environ['MALICIOUS_BINARY'], arcname='glade')
 info=tarfile.TarInfo('../escaped'); data=b'escaped'; info.size=len(data)
 archive.addfile(info, io.BytesIO(data))
PY
  ;;
malicious-zip)
  archive=glade_smoke_fixture.zip; asset_os=windows
  export MALICIOUS_ARCHIVE="${DIST_DIR}/${archive}"
  python3 - <<'PY'
import os, zipfile
with zipfile.ZipFile(os.environ['MALICIOUS_ARCHIVE'], 'w') as archive:
 info=zipfile.ZipInfo('glade.exe'); info.external_attr=0o100755 << 16
 archive.writestr(info, '#!/usr/bin/env bash\nprintf "glade smoke fixture\\n"\n')
 archive.writestr('../escaped', 'escaped')
PY
  ;;
malicious-link)
  archive=glade_smoke_fixture.tar.gz; asset_os=darwin
  export MALICIOUS_ARCHIVE="${DIST_DIR}/${archive}" MALICIOUS_BINARY="${work}/glade"
  python3 - <<'PY'
import os, tarfile
with tarfile.open(os.environ['MALICIOUS_ARCHIVE'], 'w:gz') as archive:
 archive.add(os.environ['MALICIOUS_BINARY'], arcname='glade')
 info=tarfile.TarInfo('share/unsafe-link'); info.type=tarfile.SYMTYPE; info.linkname='../../escaped'
 archive.addfile(info)
PY
  ;;
malicious-tar-unknown)
  archive=glade_smoke_fixture.tar.gz; asset_os=darwin
  export MALICIOUS_ARCHIVE="${DIST_DIR}/${archive}" MALICIOUS_BINARY="${work}/glade"
  python3 - <<'PY'
import os, tarfile
with tarfile.open(os.environ['MALICIOUS_ARCHIVE'], 'w:gz') as archive:
 archive.add(os.environ['MALICIOUS_BINARY'], arcname='glade')
 info=tarfile.TarInfo('share/unknown'); info.type=b'8'
 archive.addfile(info)
PY
  ;;
malicious-zip-socket)
  archive=glade_smoke_fixture.zip; asset_os=windows
  export MALICIOUS_ARCHIVE="${DIST_DIR}/${archive}"
  python3 - <<'PY'
import os, stat, zipfile
with zipfile.ZipFile(os.environ['MALICIOUS_ARCHIVE'], 'w') as archive:
 binary=zipfile.ZipInfo('glade.exe'); binary.external_attr=0o100755 << 16
 archive.writestr(binary, '#!/usr/bin/env bash\nprintf "glade smoke fixture\\n"\n')
 special=zipfile.ZipInfo('share/socket'); special.external_attr=(stat.S_IFSOCK | 0o755) << 16
 archive.writestr(special, '')
PY
  ;;
esac
cat >"${work}/decoy" <<'BIN'
#!/usr/bin/env bash
exit 77
BIN
chmod +x "${work}/decoy"
tar -C "${work}" -czf "${DIST_DIR}/decoy.tar.gz" decoy
digest="$(shasum -a 256 "${DIST_DIR}/${archive}" | awk '{print $1}')"
if [[ "`+corrupt+`" == 1 ]]; then manifest_digest="$(printf '0%.0s' {1..64})"; else manifest_digest="${digest}"; fi
export FIXTURE_DIST="${DIST_DIR}" FIXTURE_DIGEST="${manifest_digest}" FIXTURE_REAL_DIGEST="${digest}" FIXTURE_ARCHIVE="${archive}" FIXTURE_OS="${asset_os}" FIXTURE_VERSION='`+version+`' FIXTURE_INDEX_REFERENCE='`+indexReference+`'
python3 - <<'PY'
import json, os, pathlib
d=pathlib.Path(os.environ['FIXTURE_DIST'])
version=os.environ['FIXTURE_VERSION']
m={'schemaVersion':2,'channel':'stable','version':version,'assets':[{'os':os.environ['FIXTURE_OS'],'arch':'arm64','url':'https://downloads.example/'+version+'/'+os.environ['FIXTURE_ARCHIVE'],'sha256':os.environ['FIXTURE_DIGEST']}], 'installScript':'https://example/install.sh','pluginRegistry':'https://example/plugins.json','verification':{'versionOutput':'glade smoke fixture','doctor':'passed','parserSmoke':'passed','vscodeExtensionPackage':'not present'}}
for p in (d/'release-manifest.json', d/('release-manifest-'+os.environ['FIXTURE_OS']+'-arm64.json'), d/'latest/release-manifest.json', d/version/'release-manifest.json'):
 p.write_text(json.dumps(m,sort_keys=True)+'\n')
i={'schemaVersion':1,'channel':'stable','latest':version,'versions':[{'version':version,'manifest':os.environ['FIXTURE_INDEX_REFERENCE']}]}
(d/'index.json').write_text(json.dumps(i,sort_keys=True)+'\n')
name=os.environ['FIXTURE_ARCHIVE']; digest=os.environ['FIXTURE_REAL_DIGEST']
(d/(name+'.sha256')).write_text(digest+'  ./'+name+'\n')
(d/'SHA256SUMS.txt').write_text(digest+'  ./'+name+'\n')
for n in (name,name+'.sha256','SHA256SUMS.txt'):
 (d/version/n).write_bytes((d/n).read_bytes())
PY
`)
	return root, filepath.Join(root, "runtime.log")
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
