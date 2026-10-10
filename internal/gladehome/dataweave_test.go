package gladehome

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dataweave"
)

func TestDataWeaveLocationAndMissingInstallation(t *testing.T) {
	base := t.TempDir()
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("XDG_DATA_HOME", base)
	check := func(want string) {
		t.Helper()
		if got := dataWeaveDirectory(); got != filepath.Join(want, "toolchains", "dataweave", "2.12.2") {
			t.Fatalf("directory=%s want root=%s", got, want)
		}
	}
	check(UserShareDir())
	t.Setenv("GLADE_ROOT", filepath.Join(base, "root"))
	check(filepath.Join(base, "root"))
	t.Setenv("GLADE_HOME", filepath.Join(base, "home"))
	check(filepath.Join(base, "home"))
	status := DataWeaveStatus()
	if status.OK || !strings.Contains(status.Detail, "--java-home") {
		t.Fatalf("status=%+v", status)
	}
	if _, err := DataWeaveRuntime(); err == nil {
		t.Fatal("missing installation accepted")
	}
	if _, err := os.Stat(dataWeaveDirectory()); !os.IsNotExist(err) {
		t.Fatalf("read-only status wrote directory: %v", err)
	}
	if _, err := InstallDataWeave(context.Background(), ""); err == nil {
		t.Fatal("implicit Java home accepted")
	}
}

// Explicit provisioned inputs keep normal tests offline. This test copies the
// full engine, including source offers and notices, before invoking installation.
func TestDataWeaveInstallExecuteAndTamper(t *testing.T) {
	javaHome, engine := os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME"), os.Getenv("GLADE_DATAWEAVE_TEST_ENGINE")
	if javaHome == "" || engine == "" {
		t.Skip("explicit Java17 JDK and verified DataWeave engine required")
	}
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	copyDataWeaveTestTree(t, engine, filepath.Join(dataWeaveDirectory(), "engine"))
	ctx := context.Background()
	status, err := InstallDataWeave(ctx, javaHome)
	if err != nil || !status.OK {
		t.Fatalf("install=%+v err=%v", status, err)
	}
	firstKey := status.AdapterBuildKey
	status, err = InstallDataWeave(ctx, javaHome)
	if err != nil || !status.OK || status.AdapterBuildKey != firstKey {
		t.Fatalf("reinstall=%+v err=%v", status, err)
	}
	rt, err := DataWeaveRuntime()
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Execute(ctx, dataweave.Request{Name: "ownedInstalled", APIVersion: "67.0", Source: "%dw 2.0\noutput application/json\n---\npayload.left + payload.right", Inputs: map[string]dataweave.Input{"payload": {MIMEType: "application/json", Data: []byte(`{"left":3,"right":4}`)}}})
	if err != nil || strings.TrimSpace(string(result.Data)) != "7" {
		t.Fatalf("execute=%+v err=%v", result, err)
	}
	root, manifest, err := loadDataWeaveInstallation()
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "installation.json")
	originalManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	reject := func() {
		t.Helper()
		if status := DataWeaveStatus(); status.OK {
			t.Fatalf("tampered installation ready: %+v", status)
		}
		if _, err := DataWeaveRuntime(); err == nil {
			t.Fatal("tampered runtime returned")
		}
	}
	t.Run("adapter", func(t *testing.T) {
		path := filepath.Join(rt.AdapterDirectory, "Adapter.class")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(original, 0), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(path, original, 0600)
		reject()
	})
	t.Run("unexpected-adapter-file", func(t *testing.T) {
		path := filepath.Join(rt.AdapterDirectory, "Owned.class")
		if err := os.WriteFile(path, []byte("owned"), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(path)
		reject()
	})
	t.Run("format-descriptor", func(t *testing.T) {
		path := filepath.Join(rt.AdapterDirectory, "META-INF", "services", "org.mule.weave.v2.module.DataFormat")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("UnverifiedFormat\n"), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(path, original, 0600)
		reject()
	})
	t.Run("other-service", func(t *testing.T) {
		path := filepath.Join(rt.AdapterDirectory, "META-INF", "services", "owned.other.Service")
		if err := os.WriteFile(path, []byte("Owned\n"), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(path)
		reject()
	})
	t.Run("identity", func(t *testing.T) {
		changed := manifest
		changed.Identity.EngineVersion = "2.12.1"
		payload, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, payload, 0600); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(manifestPath, originalManifest, 0600)
		reject()
	})
	t.Run("java-identity", func(t *testing.T) {
		changed := manifest
		changed.JavaSHA256 = strings.Repeat("0", 64)
		payload, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, payload, 0600); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(manifestPath, originalManifest, 0600)
		reject()
	})
	t.Run("source-offer", func(t *testing.T) {
		entries, err := os.ReadDir(filepath.Join(root, "engine", "sources"))
		if err != nil || len(entries) == 0 {
			t.Fatalf("sources: %v", err)
		}
		path := filepath.Join(root, "engine", "sources", entries[0].Name())
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(path, original, 0600)
		reject()
	})
	if status := DataWeaveStatus(); !status.OK {
		t.Fatalf("restored installation=%+v", status)
	}
}

func copyDataWeaveTestTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, payload, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
