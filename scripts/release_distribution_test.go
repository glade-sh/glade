package scripts

import (
	"os"
	"path/filepath"
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
		"actions/checkout@v6",
		"actions/setup-go@v6",
		`go-version: "1.26.3"`,
		"actions/setup-node@v6",
		`node-version: "22"`,
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
		"curl_github",
		"private repo",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
}
