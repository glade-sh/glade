package gladecli

import (
 "context"
 "github.com/glade-sh/glade/internal/dap"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDAPTestIndexLoaderCachesFrontend(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `@isTest private class SampleTest { @isTest static void pass() {} }`)
	if _, err := dapTestIndexLoader(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "semantic", "frontend.json")); err != nil {
		t.Fatalf("frontend snapshot missing: %v", err)
	}
}

func TestDAPTestIndexLoaderNoCachePolicy(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "1")
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `@isTest private class SampleTest { @isTest static void pass() {} }`)
	if _, err := dapTestIndexLoader(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "semantic", "frontend.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected frontend snapshot: %v", err)
	}
}

func TestDAPWarmTestIndexSkipsBuild(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `@isTest private class SampleTest { @isTest static void pass() {} }`)
	builds := 0
	build := func(p project.Project, s schema.Schema) (typesys.Index, typesys.BuildArtifacts) {
		builds++
		return typesys.BuildWithArtifacts(p, s)
	}
	if _, err := loadDAPTestIndexWithBuilder(root, build); err != nil {
		t.Fatal(err)
	}
	if builds != 1 {
		t.Fatalf("cold builds = %d", builds)
	}
	if _, err := loadDAPTestIndexWithBuilder(root, build); err != nil {
		t.Fatal(err)
	}
	if builds != 1 {
		t.Fatalf("warm access rebuilt index: %d builds", builds)
	}
}

func TestDAPColdLaunchPopulatesWarmIndex(t *testing.T) {
 t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
 restore := apextest.EnableDiskCacheForTesting()
 t.Cleanup(restore)
 root := t.TempDir()
 t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
 writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
 writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `@isTest private class SampleTest { @isTest static void pass() {} }`)
 launch := func() { t.Helper(); if err := handleDAPLaunch(context.Background(), dap.NewHandler(dap.Snapshot{}), dap.LaunchRequest{ClassName:"SampleTest", MethodName:"pass"}, root, "", true); err != nil { t.Fatal(err) } }
 builds := 0
 previous := dapTestIndexLoader
 dapTestIndexLoader = func(root string) (typesys.Index, error) { return loadDAPTestIndexWithBuilder(root, func(p project.Project, s schema.Schema) (typesys.Index, typesys.BuildArtifacts) { builds++; return typesys.BuildWithArtifacts(p,s) }) }
 t.Cleanup(func(){ dapTestIndexLoader = previous })
 launch()
 launch()
 if builds != 0 { t.Fatalf("warm launch rebuilt index %d times", builds) }
}
