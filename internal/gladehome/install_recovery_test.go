package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootRecoversInterruptedSourceInstall(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	share := UserShareDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs", ".install.lock",
	} {
		writeMinimalToolchainFile(t, share, rel)
	}
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("P"), 0600); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(share, ".toolchain-stage-fixture")
	backup := filepath.Join(stage, "old", "third_party", "lwc")
	if err := os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(share, "third_party", "lwc")
	if err := os.Rename(current, backup); err != nil {
		t.Fatal(err)
	}
	writeMinimalToolchainFile(t, share, "third_party/lwc/compile.mjs")
	writeMinimalToolchainFile(t, share, "third_party/lwc/node_modules/@lwc/compiler/index.js")
	if err := os.WriteFile(filepath.Join(current, "compile.mjs"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "journal.json"), []byte(`[{"rel":"third_party/lwc","hadOld":true}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "started"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Root(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(current, "compile.mjs"))
	if err != nil || string(got) != "third_party/lwc/compile.mjs" {
		t.Fatalf("recovered compiler = %q, %v", got, err)
	}
}

func TestRootRefusesInterruptedShellInstall(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	share := UserShareDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs", ".install.lock", ".install-txn.fixture/started",
	} {
		writeMinimalToolchainFile(t, share, rel)
	}
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("S"+share), 0600); err != nil {
		t.Fatal(err)
	}
	if root, err := Root(); err == nil {
		t.Fatalf("Root returned %q while shell installation needs recovery", root)
	}
}

func TestRootRecoversNewSourceAssetAndKeepsCommittedGeneration(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "uncommitted"
		if committed {
			name = "committed"
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("XDG_DATA_HOME", base)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			share := UserShareDir()
			for _, rel := range []string{
				"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
				"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
				"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
				"lwcruntime/src/lightning/button.mjs", ".install.lock",
				"lwcruntime/src/experience/new.mjs", ".toolchain-stage-fixture/started",
			} {
				writeMinimalToolchainFile(t, share, rel)
			}
			if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("P"), 0600); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(share, ".toolchain-stage-fixture")
			if err := os.WriteFile(filepath.Join(stage, "journal.json"), []byte(`[{"rel":"lwcruntime/src/experience","hadOld":false}]`), 0600); err != nil {
				t.Fatal(err)
			}
			if committed {
				if err := os.WriteFile(filepath.Join(stage, "committed"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Root(); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(filepath.Join(share, "lwcruntime/src/experience/new.mjs"))
			if committed && err != nil {
				t.Fatalf("committed asset missing: %v", err)
			}
			if !committed && !os.IsNotExist(err) {
				t.Fatalf("uncommitted asset remains: %v", err)
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatalf("recovery stage remains: %v", err)
			}
		})
	}
}

func TestRootRejectsInvalidSourceRecoveryJournal(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	share := UserShareDir()
	for _, rel := range []string{".install.lock", ".toolchain-stage-fixture/started"} {
		writeMinimalToolchainFile(t, share, rel)
	}
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("P"), 0600); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(share, ".toolchain-stage-fixture")
	if err := os.WriteFile(filepath.Join(stage, "journal.json"), []byte(`[{"rel":"../outside","hadOld":false}]`), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Root(); err == nil {
		t.Fatal("invalid journal was accepted")
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "keep" {
		t.Fatalf("outside file = %q, %v", got, err)
	}
}

func TestRootRefusesInterruptedCustomShellInstall(t *testing.T) {
	base := t.TempDir()
	share := filepath.Join(base, "custom")
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", share)
	t.Setenv("GLADE_ROOT", "")
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs", ".install-txn.fixture/started",
	} {
		writeMinimalToolchainFile(t, share, rel)
	}
	writeMinimalToolchainFile(t, share, ".install.lock")
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("S"+share), 0600); err != nil {
		t.Fatal(err)
	}
	if root, err := Root(); err == nil {
		t.Fatalf("Root returned %q while custom installation needs recovery", root)
	}
}

func TestRootRefusesPendingShellMarker(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	share := UserShareDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs",
	} {
		writeMinimalToolchainFile(t, share, rel)
	}
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("S"+share), 0600); err != nil {
		t.Fatal(err)
	}
	if root, err := Root(); err == nil {
		t.Fatalf("Root returned %q while shell recovery is pending", root)
	}
}

func TestEnsureRootRefusesPendingCustomShellMarker(t *testing.T) {
	base := t.TempDir()
	share := filepath.Join(base, "custom")
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", share)
	t.Setenv("GLADE_ROOT", "")
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs",
	} {
		writeMinimalToolchainFile(t, share, rel)
	}
	writeMinimalToolchainFile(t, share, ".install.lock")
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("S"+share), 0600); err != nil {
		t.Fatal(err)
	}
	if root, err := EnsureRoot(); err == nil {
		t.Fatalf("EnsureRoot returned %q while custom recovery is pending", root)
	}
}

func TestInstallFromCleansAbandonedSourceStage(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	stage := filepath.Join(UserShareDir(), ".toolchain-stage-orphan")
	if err := os.MkdirAll(filepath.Join(stage, "new"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "new", "partial"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := InstallFrom(filepath.Join(base, "missing")); err == nil {
		t.Fatal("invalid source was accepted")
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("abandoned stage remains: %v", err)
	}
}

func TestRootRefusesUnjournaledSourceBackup(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	share := UserShareDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs",
		".toolchain-stage-orphan/old/third_party/lwc/keep",
	} {
		writeMinimalToolchainFile(t, share, rel)
	}
	if err := os.WriteFile(filepath.Join(share, ".install.lock"), []byte("P"), 0600); err != nil {
		t.Fatal(err)
	}
	if root, err := Root(); err == nil {
		t.Fatalf("Root returned %q despite an unjournaled backup", root)
	}
	if _, err := os.Stat(filepath.Join(share, ".toolchain-stage-orphan/old/third_party/lwc/keep")); err != nil {
		t.Fatal(err)
	}
}
