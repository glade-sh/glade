package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
)

// A runtime snapshot made before numeric constants were retained cannot
// correctly render a Flow error after the executing locale becomes relevant.
func TestFlowMergeRejectsPreLocaleRuntimeCache(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	source, err := os.ReadFile("../automation/testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/Format_constant.flow-meta.xml"), string(source))
	idx := loadTestIndex(t, root)
	org, err := orgFromIndexWithError(idx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := startupcache.NewEntry(root, p, idx, org, startupcache.CompiledRuntime{})
	if !startupcache.Fresh(&entry, root, startupcache.Version) {
		t.Fatal("cache fixture must otherwise be fresh")
	}
	for _, version := range []string{"apextest-runtime-v15", "apextest-runtime-v16", "apextest-runtime-v17"} {
		entry.RuntimeABI = version
		if startupcache.FreshRuntime(&entry, root, startupcache.Version, testRuntimeCacheABI) {
			t.Fatalf("stale runtime cache %s accepted", version)
		}
	}
	entry.RuntimeABI = testRuntimeCacheABI
	if !startupcache.FreshRuntime(&entry, root, startupcache.Version, testRuntimeCacheABI) {
		t.Fatal("current runtime cache rejected")
	}
}
