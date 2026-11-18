package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
)

// A source-fresh v31 entry can contain an older, unmodeled Transform. The
// version bump must reject it before rebuilding and persisting the new shape.
func TestFlowTransformRejectV31DiskRuntime(t *testing.T) {
	const legacyABI = "apextest-runtime-v31"
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join("testdata/native-capture/flow_transform_record_map", "api"+strings.TrimSuffix(api, ".0"), "TransformProbe.flow-meta.xml")
			flow, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			classes := map[string]string{"TransformLegacyCacheTest": `@IsTest private class TransformLegacyCacheTest {
				@IsTest static void observe() {
					Flow.Interview f=Flow.Interview.createInterview('TransformProbe',new Map<String,Object>{'InputText'=>'Map sample'});
					f.start(); Contact c=(Contact)f.getVariableValue('OutputRecord');
					System.assertEquals('Map sample',c.LastName);
					System.assertEquals(null,c.Title);
					System.assertEquals(true,c.getPopulatedFieldsAsMap().containsKey('Title'));
				}
			}`}
			index := flowCoreIndex(t, api, classes, map[string]string{"force-app/main/default/flows/TransformProbe.flow-meta.xml": string(flow)})
			InvalidateRuntimeCaches()
			key, built, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			input, err := validatedDiskRuntimeInput(index, nil)
			if err != nil || input == nil {
				t.Fatalf("validate input: %v", err)
			}
			entry, err := startupcache.NewEntryWithValidatedInput(input, built.restored.CloneOrg(), startupcache.CompiledRuntime{
				Methods: built.Methods, Classes: built.Classes, Triggers: built.Triggers, PageNames: built.PageNames,
			})
			if err != nil {
				t.Fatal(err)
			}
			entry.RuntimeABI, entry.RuntimeKey = legacyABI, string(key)
			if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
				t.Fatal(err)
			}
			legacy, err := startupcache.ReadFreshRuntimeWithValidatedInput(index.Project.Root, startupcache.SubdirTest, startupcache.Version, legacyABI, string(key), input)
			if err != nil || legacy == nil {
				t.Fatalf("v31 fixture is not otherwise fresh: entry=%v err=%v", legacy != nil, err)
			}
			InvalidateRuntimeCaches()
			if _, ok := tryLoadDiskRuntimeWithSourceDigests(index, nil, key); ok {
				t.Fatal("accepted source-fresh v31 runtime")
			}
			if _, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), true); err != nil {
				t.Fatal(err)
			}
			persisted, err := startupcache.Read(index.Project.Root, startupcache.SubdirTest)
			if err != nil || persisted == nil || persisted.RuntimeABI != testRuntimeCacheABI || persisted.RuntimeABI == legacyABI {
				t.Fatalf("rebuilt runtime cache=%v err=%v", persisted != nil, err)
			}
			InvalidateRuntimeCaches()
			run := Run(index, Options{SelectedClasses: []string{"TransformLegacyCacheTest"}, SelectedMethod: "observe", Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("summary=%+v problem=%s", summary, firstRunProblem(run))
			}
		})
	}
}
