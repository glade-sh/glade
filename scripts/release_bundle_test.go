package scripts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseBundleAddsArchiveJavaScriptComponentsToSBOM(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "glade.tar.gz")
	vsix := releaseBundleVSIX(t)
	writeReleaseBundleArchive(t, archive, map[string][]byte{
		"LICENSE": []byte("Glade license\n"),
		"NOTICE":  []byte("Glade\nCopyright 2026 Matt Simonis\n"),
		"share/glade/third_party/lwc/package.json":                                []byte(`{"dependencies":{"@babel/parser":"7.0.0","@lwc/compiler":"8.0.0"}}`),
		"share/glade/third_party/lwc/node_modules/@babel/parser/package.json":     []byte(`{"name":"@babel/parser","version":"7.0.0","license":"MIT"}`),
		"share/glade/third_party/lwc/node_modules/@babel/parser/lib/package.json": []byte(`{"private":true}`),
		"share/glade/third_party/lwc/node_modules/@lwc/compiler/package.json":     []byte(`{"name":"@lwc/compiler","version":"8.0.0","license":"MIT"}`),
		"share/glade/editor/vscode-glade.vsix":                                    vsix,
	})
	sbom := filepath.Join(root, "glade.sbom.json")
	writeReleaseFixtureFile(t, sbom, `{"bomFormat":"CycloneDX","specVersion":"1.6","components":[]}`)
	writeReleaseFixtureFile(t, filepath.Join(root, "package-lock.json"), `{"packages":{"node_modules/vscode-languageclient":{"version":"999.0.0"}}}`)

	cmd := exec.Command("python3", "release-bundle.py", "sbom", archive, sbom)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("extend SBOM: %v\n%s", err, out)
	}
	var document struct {
		Components []struct {
			Name       string `json:"name"`
			PURL       string `json:"purl"`
			Properties []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"properties"`
		} `json:"components"`
	}
	if err := json.Unmarshal(mustReadFile(t, sbom), &document); err != nil {
		t.Fatalf("parse extended SBOM: %v", err)
	}
	components := make(map[string]struct {
		PURL string
		Path string
	})
	for _, component := range document.Components {
		for _, property := range component.Properties {
			if property.Name == "glade:archive-path" {
				components[component.Name] = struct{ PURL, Path string }{component.PURL, property.Value}
			}
		}
	}
	for name, want := range map[string]struct{ PURL, Path string }{
		"@babel/parser":         {"pkg:npm/%40babel/parser@7.0.0", "share/glade/third_party/lwc/node_modules/@babel/parser/package.json"},
		"@lwc/compiler":         {"pkg:npm/%40lwc/compiler@8.0.0", "share/glade/third_party/lwc/node_modules/@lwc/compiler/package.json"},
		"vscode-languageclient": {"pkg:npm/vscode-languageclient@10.1.0", "share/glade/editor/vscode-glade.vsix!/extension/out/bundled-dependencies.json"},
	} {
		if got := components[name]; got != want {
			t.Fatalf("component %s = %#v, want %#v", name, got, want)
		}
	}
	if _, found := components["dev-only"]; found {
		t.Fatal("SBOM included a lockfile-only development dependency")
	}
}

func TestReleaseBundleRejectsUnboundVSIXEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		vsix func(*testing.T) []byte
		want string
	}{
		{name: "missing-extension", vsix: func(t *testing.T) []byte { return releaseBundleVSIXWith(t, false, "") }, want: "missing bundled dependency evidence"},
		{name: "hash-mismatch", vsix: func(t *testing.T) []byte {
			return releaseBundleVSIXWith(t, true, "0000000000000000000000000000000000000000000000000000000000000000")
		}, want: "does not match extension.js"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "glade.tar.gz")
				writeReleaseBundleArchive(t, archive, map[string][]byte{
					"LICENSE": []byte("Glade license\n"),
					"NOTICE":  []byte("Glade\nCopyright 2026 Matt Simonis\n"),
				"share/glade/third_party/lwc/package.json":                            []byte(`{"dependencies":{"@babel/parser":"7.0.0"}}`),
				"share/glade/third_party/lwc/node_modules/@babel/parser/package.json": []byte(`{"name":"@babel/parser","version":"7.0.0","license":"MIT"}`),
				"share/glade/editor/vscode-glade.vsix":                                tc.vsix(t),
			})
			sbom := filepath.Join(root, "glade.sbom.json")
			writeReleaseFixtureFile(t, sbom, `{"bomFormat":"CycloneDX","specVersion":"1.6","components":[]}`)
			cmd := exec.Command("python3", "release-bundle.py", "sbom", archive, sbom)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.want) {
				t.Fatalf("SBOM evidence rejection = err %v, output %s; want %q", err, out, tc.want)
			}
		})
	}
}

func TestReleaseBundleRejectsMissingProjectNotices(t *testing.T) {
	for _, tc := range []struct {
		name          string
		archiveNotice bool
		vsix          []byte
		want          string
	}{
		{name: "archive", vsix: releaseBundleVSIX(t), want: "archive is missing NOTICE"},
		{name: "vsix", archiveNotice: true, vsix: releaseBundleVSIXWithoutProjectNotice(t), want: "VSIX is missing extension/NOTICE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "glade.tar.gz")
			files := map[string][]byte{
				"LICENSE": []byte("Glade license\n"),
				"share/glade/third_party/lwc/package.json":                            []byte(`{"dependencies":{"@babel/parser":"7.0.0"}}`),
				"share/glade/third_party/lwc/node_modules/@babel/parser/package.json": []byte(`{"name":"@babel/parser","version":"7.0.0","license":"MIT"}`),
				"share/glade/editor/vscode-glade.vsix":                                tc.vsix,
			}
			if tc.archiveNotice {
				files["NOTICE"] = []byte("Glade\nCopyright 2026 Matt Simonis\n")
			}
			writeReleaseBundleArchive(t, archive, files)
			sbom := filepath.Join(root, "glade.sbom.json")
			writeReleaseFixtureFile(t, sbom, `{"bomFormat":"CycloneDX","specVersion":"1.6","components":[]}`)
			cmd := exec.Command("python3", "release-bundle.py", "sbom", archive, sbom)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.want) {
				t.Fatalf("project notice rejection = err %v, output %s; want %q", err, out, tc.want)
			}
		})
	}
}

func releaseBundleVSIX(t *testing.T) []byte {
	return releaseBundleVSIXFixture(t, true, "", true)
}

func releaseBundleVSIXWith(t *testing.T, includeBundle bool, hashOverride string) []byte {
	return releaseBundleVSIXFixture(t, includeBundle, hashOverride, true)
}

func releaseBundleVSIXWithoutProjectNotice(t *testing.T) []byte {
	return releaseBundleVSIXFixture(t, true, "", false)
}

func releaseBundleVSIXFixture(t *testing.T, includeBundle bool, hashOverride string, includeNotice bool) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	bundle := []byte("actual extension bundle\n")
	bundleHash := sha256.Sum256(bundle)
	if hashOverride != "" {
		bundleHash = [sha256.Size]byte{}
	}
	files := map[string]string{
		"extension/LICENSE.txt":                   "extension license\n",
		"extension/out/extension.meta.json":       `{"outputs":{"out/extension.js":{"inputs":{"node_modules/vscode-languageclient/lib/node/main.js":{"bytesInOutput":12},"node_modules/dev-only/index.js":{"bytesInOutput":0}}}}}`,
		"extension/out/bundled-dependencies.json": fmt.Sprintf(`{"schemaVersion":1,"bundle":{"path":"out/extension.js","sha256":"%x"},"packages":[{"name":"vscode-languageclient","version":"10.1.0","license":"MIT","packagePath":"node_modules/vscode-languageclient","noticeFiles":["License.txt"]}]}`, bundleHash),
		"extension/out/THIRD_PARTY_NOTICES.txt":   "vscode-languageclient@10.1.0\n--- License.txt ---\nMIT notice\n",
	}
	if includeNotice {
		files["extension/NOTICE"] = "Glade\nCopyright 2026 Matt Simonis\n"
	}
	if includeBundle {
		files["extension/out/extension.js"] = string(bundle)
	}
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func writeReleaseBundleArchive(t *testing.T, archive string, files map[string][]byte) {
	t.Helper()
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, body := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseBundleResumesWithoutChangingUploadedBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "dist")
	writeReleaseFixtureFile(t, filepath.Join(source, "index.json"), "release index\n")
	bundle := filepath.Join(root, "release.tar.gz")
	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	gz.Header.ModTime = time.Unix(100, 0)
	tw := tar.NewWriter(gz)
	body := []byte("release index\n")
	if err := tw.WriteHeader(&tar.Header{Name: "./index.json", Mode: 0o644, Size: int64(len(body)), ModTime: time.Unix(100, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	original := string(mustReadFile(t, bundle))
	cmd := exec.Command("python3", "release-bundle.py", source, bundle)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("resume: %v\n%s", err, out)
	}
	if string(mustReadFile(t, bundle)) != original {
		t.Fatal("resume changed uploaded bundle bytes")
	}
	writeReleaseFixtureFile(t, filepath.Join(source, "index.json"), "different release\n")
	cmd = exec.Command("python3", "release-bundle.py", source, bundle)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("accepted changed payload: %s", out)
	}
	if string(mustReadFile(t, bundle)) != original {
		t.Fatal("rejection changed uploaded bundle bytes")
	}
}

func TestReleaseBundleIgnoresSourceTimestamps(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "dist")
	file := filepath.Join(source, "v1.2.3", "release-manifest.json")
	writeReleaseFixtureFile(t, file, "{}\n")
	first, second := filepath.Join(root, "first.tar.gz"), filepath.Join(root, "second.tar.gz")
	for i, output := range []string{first, second} {
		stamp := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(file, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("python3", "release-bundle.py", source, output)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bundle: %v\n%s", err, out)
		}
	}
	if string(mustReadFile(t, first)) != string(mustReadFile(t, second)) {
		t.Fatal("bundle depends on source timestamps")
	}
}
