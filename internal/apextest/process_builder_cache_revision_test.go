package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
)

// A cache built by the prior compiler must be rejected before restoring the
// project org containing compiled FlowRules; then the current runtime rebuilds it.
func TestProcessBuilderRejectsPriorDiskRuntimeCache(t *testing.T) {
	for _, tc := range []struct{ abi, suffix string }{
		{"apextest-runtime-v31", ""}, {"apextest-runtime-v32", ""}, {"apextest-runtime-v32", "|stale"},
		{"apextest-runtime-v34", "|process-builder-v036"},
		{"apextest-runtime-v34", "|process-builder-v036-case1"},
		{"apextest-runtime-v34", "|process-builder-v036-branched1"},
		{"apextest-runtime-v34", "|process-builder-v036-input1"},
		{"apextest-runtime-v34", "|process-builder-v036-case2"},
		{"apextest-runtime-v34", "|process-builder-v036-replace1"},
		{"apextest-runtime-v34", "|process-builder-v036-calc1"},
		{"apextest-runtime-v34", "|process-builder-v036-datefirst1"},
		{"apextest-runtime-v34", "|process-builder-v036-outputs1"},
		{"apextest-runtime-v34", "|process-builder-v036-outputs2"},
		{"apextest-runtime-v34", "|process-builder-v036-projection1"},
		{"apextest-runtime-v34", ""}, {"apextest-runtime-v35", ""},
	} {
		t.Run(tc.abi+tc.suffix, func(t *testing.T) { testProcessBuilderRejectsRuntimeCache(t, tc.abi, tc.suffix) })
	}
}

func testProcessBuilderRejectsRuntimeCache(t *testing.T, priorABI, priorSuffix string) {
	restore := EnableDiskCacheForTesting()
	defer restore()
	defer InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CacheProbeTest.cls"), `@IsTest private class CacheProbeTest { @IsTest static void works() { System.assert(true); } }`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CacheProbeTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	index := loadTestIndex(t, root)
	proof, err := startupcache.ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := startupcache.NewEntryWithValidatedInput(proof, orgFromIndex(index), startupcache.CompiledRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	entry.RuntimeABI = priorABI
	entry.RuntimeKey = string(runtimeKey(index)) + priorSuffix
	if _, err := startupcache.WriteWithStats(&entry, startupcache.SubdirTest); err != nil {
		t.Fatal(err)
	}
	InvalidateRuntimeCaches()
	if _, ok := tryLoadDiskRuntime(index); ok {
		t.Fatalf("restored prior runtime cache %s", priorABI)
	}
	ResetPerfCounters()
	run := Run(index, Options{Parallelism: 1, PerfCounters: true})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("rebuild summary=%+v", summary)
	}
	if SnapshotPerfCounters().Phases.DiskCacheHits != 0 {
		t.Fatal("rebuild used stale disk cache")
	}
	rebuilt, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || rebuilt == nil {
		t.Fatalf("rebuilt cache=%v err=%v", rebuilt, err)
	}
	if rebuilt.RuntimeABI != testRuntimeCacheABI || rebuilt.RuntimeKey != string(runtimeKey(index)) {
		t.Fatalf("rebuilt ABI=%q, runtime key matches=%t", rebuilt.RuntimeABI, rebuilt.RuntimeKey == string(runtimeKey(index)))
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	warm := Run(index, Options{Parallelism: 1, PerfCounters: true})
	if summary := warm.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("warm summary=%+v", summary)
	}
	if SnapshotPerfCounters().Phases.DiskCacheHits == 0 {
		t.Fatal("rebuilt cache was not restored")
	}
}
