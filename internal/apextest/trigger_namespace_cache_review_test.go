package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
)

// Cache identity is a local contract. The Salesforce-facing own-namespace
// behavior is captured at subproject_namespace_runtime/org.tsv:2-7.
func TestTriggerNamespaceRuntimeKeyReview(t *testing.T) {
	index := typesys.Index{Triggers: []typesys.TriggerSymbol{{Name: "AlphaTrigger", ObjectName: "Account"}}}
	// The exact pre-change key for this fixed, source-free index. Any persisted
	// entry under that identity must be rejected after the upgrade, even when
	// its trigger namespace is empty.
	const legacyKey = "053eef288628233e5ec9da829e6cf2ae"
	keys := map[runtimeCacheKey]string{}
	for _, namespace := range []string{"", "First", "Second"} {
		index.Triggers[0].Namespace = namespace
		key := runtimeKeyWithDigestLookup(index, nil, nil)
		if key == legacyKey {
			t.Errorf("namespace %q reused pre-upgrade trigger key %s", namespace, key)
		}
		if other, exists := keys[key]; exists {
			t.Errorf("namespace %q and %q share key %s", other, namespace, key)
		}
		keys[key] = namespace
	}
}

func TestTriggerNamespaceProjectCacheMutationReview(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(EnableDiskCacheForTesting())
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			InvalidateRuntimeCaches()
			root := t.TempDir()
			child := filepath.Join(root, "packs", "child")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs","default":true}],"sourceApiVersion":"`+api+`"}`)
			config := filepath.Join(child, "sfdx-project.json")
			setNamespace := func(namespace string) {
				writeFile(t, config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"`+namespace+`","sourceApiVersion":"`+api+`"}`)
			}
			trigger := filepath.Join(child, "force-app", "main", "default", "triggers", "CacheReview.trigger")
			writeFile(t, trigger, `trigger CacheReview on Account (before insert) {}`)
			writeFile(t, trigger+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
			setNamespace("First")
			first := loadTestIndex(t, root)
			get := func(index typesys.Index, want, route string) runtimeCacheKey {
				t.Helper()
				counters := newRunPerfCounters(true)
				key, runtime, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters)
				if err != nil {
					t.Fatal(err)
				}
				if len(runtime.Triggers) != 1 || runtime.Triggers[0].Namespace != want {
					t.Fatalf("trigger namespace after cache lookup: got %+v, want %q", runtime.Triggers, want)
				}
				phases := snapshotPerfCounters(counters).Phases
				if (phases.MemoryCacheHits == 1) != (route == "memory") || (phases.DiskCacheHits == 1) != (route == "disk") {
					input, inputErr := validatedDiskRuntimeInput(index, nil)
					disk, _, diskErr := startupcache.ReadWithStats(index.Project.Root, startupcache.SubdirTest)
					t.Fatalf("cache route %s: %+v; validated input=%v error=%v; disk=%v error=%v", route, phases, input != nil, inputErr, disk != nil, diskErr)
				}
				return key
			}
			firstKey := get(first, "First", "cold")
			if warm := get(first, "First", "memory"); warm != firstKey {
				t.Fatal("warm key changed")
			}
			setNamespace("Second")
			second := loadTestIndex(t, root)
			if len(second.Triggers) != 1 || second.Triggers[0].Namespace != "Second" {
				t.Fatalf("reloaded triggers: %+v", second.Triggers)
			}
			if secondKey := get(second, "Second", "cold"); secondKey == firstKey {
				t.Fatal("namespace-only project edit reused runtime key")
			}
			InvalidateRuntimeCaches()
			get(second, "Second", "disk")
		})
	}
}
