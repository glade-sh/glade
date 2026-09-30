package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/glade-sh/glade/internal/gladehome"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataWeaveToolchainCLIExplicitComponent(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	var output bytes.Buffer
	err := runToolchain(context.Background(), []string{"status", "dataweave", "--json"}, &output)
	if err == nil {
		t.Fatal("missing runtime reported ready")
	}
	var status struct {
		OK            bool   `json:"ok"`
		Detail        string `json:"detail"`
		EngineVersion string `json:"engineVersion"`
	}
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.OK || status.EngineVersion != "2.12.2" || !strings.Contains(status.Detail, "--java-home") {
		t.Fatalf("status=%+v", status)
	}
	output.Reset()
	if err := runToolchain(context.Background(), []string{"install", "dataweave"}, &output); err == nil || !strings.Contains(err.Error(), "--java-home") {
		t.Fatalf("missing Java path: %v", err)
	}
	output.Reset()
	if err := runToolchain(context.Background(), []string{"install", "dataweave", "--java-home"}, &output); err == nil {
		t.Fatal("missing Java path value accepted")
	}
	output.Reset()
	if err := runToolchain(context.Background(), []string{"install", "dataweave", "--help"}, &output); err != nil || !strings.Contains(output.String(), "Java17-JDK") {
		t.Fatalf("component help: %v %s", err, output.String())
	}
}

func TestDataWeaveToolchainCLIInstalledJSON(t *testing.T) {
	javaHome, engine := os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME"), os.Getenv("GLADE_DATAWEAVE_TEST_ENGINE")
	if javaHome == "" || engine == "" {
		t.Skip("explicit Java17 JDK and verified DataWeave engine required")
	}
	root := t.TempDir()
	t.Setenv("GLADE_HOME", root)
	t.Setenv("GLADE_ROOT", "")
	if err := os.CopyFS(filepath.Join(root, "toolchains", "dataweave", "2.12.2", "engine"), os.DirFS(engine)); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runToolchain(context.Background(), []string{"install", "dataweave", "--java-home", javaHome, "--json"}, &output); err != nil {
		t.Fatal(err)
	}
	var installed gladehome.DataWeaveToolchainStatus
	if err := json.Unmarshal(output.Bytes(), &installed); err != nil {
		t.Fatal(err)
	}
	if !installed.OK || installed.JavaPath == "" || installed.AdapterBuildKey == "" {
		t.Fatalf("installed=%+v", installed)
	}
	output.Reset()
	if err := runToolchain(context.Background(), []string{"status", "dataweave", "--json"}, &output); err != nil {
		t.Fatal(err)
	}
	var status gladehome.DataWeaveToolchainStatus
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status != installed {
		t.Fatalf("status=%+v differs from install=%+v", status, installed)
	}
}
