package scripts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseWorkflowMatchesCIToolchain(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"macos-15-intel",
		"actions/checkout@v6",
		"actions/setup-go@v6",
		`go-version: "1.26.3"`,
		"actions/setup-node@v6",
		`node-version: "22"`,
		"actions/upload-artifact@v7",
		"actions/download-artifact@v8",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release.yml missing %q", want)
		}
	}
}

func TestInstallScriptSupportsPrivateReleaseToken(t *testing.T) {
	installPath := filepath.Join("..", "site", "install.sh")
	installScript, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read %s: %v", installPath, err)
	}
	scriptText := string(installScript)
	for _, want := range []string{
		"GLADE_GITHUB_TOKEN",
		"GH_TOKEN",
		"GITHUB_TOKEN",
		"Authorization: Bearer",
		"api.github.com/repos",
		"curl_github",
		"download_asset",
		"Accept: application/octet-stream",
		"private repo",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
}

func TestReleaseBuildPackagesLWCRuntimeAssets(t *testing.T) {
	releasePath := filepath.Join("..", "scripts", "release-build.sh")
	releaseScript, err := os.ReadFile(releasePath)
	if err != nil {
		t.Fatalf("read %s: %v", releasePath, err)
	}
	scriptText := string(releaseScript)
	for _, want := range []string{
		"lwcruntime/src/experience",
		"lwcruntime/src/lightning",
		"lwcruntime/src/shell",
		"lwcruntime/src/shims",
		"lwcruntime/src/slds",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("release-build.sh missing %q", want)
		}
	}
}

func TestReleaseWorkflowUsesRepoReleaseNotes(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"Build release notes",
		`scripts/release-notes.sh "$GITHUB_REF_NAME" > release-notes.md`,
		"--notes-file release-notes.md",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release.yml missing %q", want)
		}
	}
	if strings.Contains(workflowText, `--notes "`) {
		t.Fatalf("release.yml should not publish inline release notes")
	}
}

func TestReleaseWorkflowPreservesDownloadIndexHistory(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"scripts/release-index.py",
		`--version "$VERSION"`,
		"--output index.json",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release.yml missing index-history marker %q", want)
		}
	}

	indexPath := filepath.Join("release-index.py")
	indexScript, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read %s: %v", indexPath, err)
	}
	indexText := string(indexScript)
	for _, want := range []string{
		"urllib.request.Request",
		"glade-release-workflow/1.0",
		"versions_by_version",
		"could not read existing download index",
	} {
		if !strings.Contains(indexText, want) {
			t.Fatalf("release-index.py missing index-history marker %q", want)
		}
	}
}

func TestReleaseIndexScriptPreservesHistoryWithReleaseUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.UserAgent(); got != "glade-release-workflow/1.0" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "schemaVersion": 1,
  "channel": "stable",
  "latest": "v0.2.3",
  "versions": [
    {"version": "v0.2.3", "manifest": "https://downloads.glade.sh/v0.2.3/release-manifest.json"},
    {"version": "v0.2.2", "manifest": "https://downloads.glade.sh/v0.2.2/release-manifest.json"}
  ]
}`))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "index.json")
	cmd := exec.Command("python3", "release-index.py",
		"--version", "v0.2.4",
		"--download-base", "https://downloads.glade.sh",
		"--existing-index-url", server.URL,
		"--output", outputPath,
	)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-index.py failed: %v\n%s", err, out)
	}

	var index struct {
		Latest   string `json:"latest"`
		Versions []struct {
			Version  string `json:"version"`
			Manifest string `json:"manifest"`
		} `json:"versions"`
	}
	indexBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated index: %v", err)
	}
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		t.Fatalf("parse generated index: %v\n%s", err, indexBytes)
	}
	if index.Latest != "v0.2.4" {
		t.Fatalf("latest = %q, want v0.2.4", index.Latest)
	}
	gotVersions := make([]string, 0, len(index.Versions))
	for _, row := range index.Versions {
		gotVersions = append(gotVersions, row.Version)
	}
	wantVersions := []string{"v0.2.4", "v0.2.3", "v0.2.2"}
	if strings.Join(gotVersions, ",") != strings.Join(wantVersions, ",") {
		t.Fatalf("versions = %#v, want %#v\n%s", gotVersions, wantVersions, indexBytes)
	}
	if index.Versions[0].Manifest != "https://downloads.glade.sh/v0.2.4/release-manifest.json" {
		t.Fatalf("current manifest = %q", index.Versions[0].Manifest)
	}
}

func TestReleaseWorkflowCopiesVersionManifestIntoVersionDirectory(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	if !strings.Contains(workflowText, `shutil.copyfile("release-manifest.json", os.path.join(version, "release-manifest.json"))`) {
		t.Fatalf("release.yml should copy the combined manifest into the version directory")
	}
}

func TestReleaseNotesScriptExtractsVersionSectionWithRealLineBreaks(t *testing.T) {
	cmd := exec.Command("bash", "release-notes.sh", "v0.2.3")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-notes.sh v0.2.3 failed: %v\n%s", err, out)
	}
	notes := string(out)
	for _, want := range []string{
		"Glade v0.2.3 ships the latest fixes after v0.2.2.",
		"Issue closeout:",
		"5,005 Apex types",
		"duplicate top-level Apex",
		"classes inside its configured",
	} {
		if !strings.Contains(notes, want) {
			t.Fatalf("release notes missing %q\n%s", want, notes)
		}
	}
	for _, notWant := range []string{
		`\n`,
		"## v0.2.2",
		"## Unreleased",
	} {
		if strings.Contains(notes, notWant) {
			t.Fatalf("release notes unexpectedly contain %q\n%s", notWant, notes)
		}
	}
}

func TestCIWorkflowResolvesGladeToolsRefBeforeCheckout(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"Resolve glade-tools ref",
		"scripts/resolve-sibling-ref.sh",
		"steps.glade-tools-ref.outputs.ref",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
	if strings.Contains(workflowText, "ref: ${{ startsWith(github.ref, 'refs/tags/') && github.ref_name || 'main' }}") {
		t.Fatalf("ci.yml should not require every glade tag to exist in glade-tools")
	}
}

func TestResolveSiblingRefScript(t *testing.T) {
	remoteWithTag := makeGitRemote(t, "v9.9.9")
	if got := runResolveSiblingRef(t, remoteWithTag, "v9.9.9", "main"); got != "v9.9.9" {
		t.Fatalf("tagged remote resolved %q, want v9.9.9", got)
	}

	remoteWithoutTag := makeGitRemote(t, "")
	if got := runResolveSiblingRef(t, remoteWithoutTag, "v9.9.9", "main"); got != "main" {
		t.Fatalf("untagged remote resolved %q, want main", got)
	}
}

func TestInstallScriptStagesToolchainReplacement(t *testing.T) {
	installPath := filepath.Join("..", "site", "install.sh")
	installScript, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read %s: %v", installPath, err)
	}
	scriptText := string(installScript)
	for _, want := range []string{
		"replace_toolchain_dir",
		".install.",
		".backup.",
		`mv "$share_dir/$name" "$backup"`,
		"restore",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("install.sh missing staged replacement marker %q", want)
		}
	}
}

func TestInstallDocsPutEnvOnShellSideOfPipe(t *testing.T) {
	badPipe := regexp.MustCompile(`GLADE_[A-Z_]+=.*curl -fsSL https://glade\.sh/install\.sh \| sh`)
	for _, docPath := range []string{
		filepath.Join("..", "docs", "INSTALL.md"),
		filepath.Join("..", "site", "docs-src", "guide", "installation.md"),
	} {
		contents, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("read %s: %v", docPath, err)
		}
		docText := string(contents)
		if badPipe.MatchString(docText) {
			t.Fatalf("%s puts GLADE_* env before curl instead of before sh", docPath)
		}
		for _, want := range []string{
			"curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin sh",
			"curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh",
		} {
			if !strings.Contains(docText, want) {
				t.Fatalf("%s missing %q", docPath, want)
			}
		}
	}
}

func TestInstallDocsDocumentGuardedUpdate(t *testing.T) {
	for _, docPath := range []string{
		filepath.Join("..", "docs", "INSTALL.md"),
		filepath.Join("..", "site", "docs-src", "guide", "installation.md"),
	} {
		contents, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("read %s: %v", docPath, err)
		}
		docText := string(contents)
		for _, want := range []string{
			"glade update --dry-run",
			"GLADE_UPDATE_ALLOW_SHELL=1 glade update",
			"updates the",
			"directory that contains the current `glade` binary",
		} {
			if !strings.Contains(docText, want) {
				t.Fatalf("%s missing %q", docPath, want)
			}
		}
	}
}

func runResolveSiblingRef(t *testing.T, remote, requested, fallback string) string {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "github-output")
	cmd := exec.Command("bash", "resolve-sibling-ref.sh", remote, requested, fallback)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+outputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve-sibling-ref.sh failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	outputBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read GITHUB_OUTPUT: %v", err)
	}
	if want := "ref=" + got; !strings.Contains(string(outputBytes), want) {
		t.Fatalf("GITHUB_OUTPUT missing %q in %q", want, outputBytes)
	}
	return got
}

func makeGitRemote(t *testing.T, tag string) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatalf("mkdir work repo: %v", err)
	}
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "glade-test@example.com")
	runGit(t, work, "config", "user.name", "Glade Test")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "branch", "-M", "main")
	if tag != "" {
		runGit(t, work, "tag", tag)
	}
	remote := filepath.Join(root, "remote.git")
	runCommand(t, "", "git", "clone", "--bare", work, remote)
	return remote
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	runCommand(t, dir, "git", args...)
}

func runCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
