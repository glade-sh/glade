package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native setter and getter observations: setter_getter_source_routes/org.tsv:2-25.
func TestSetterGetterSourceRoutesNative(t *testing.T) {
	const fixture = "testdata/conformance/setter_getter_source_routes"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(rows), "\n") != 25 {
		t.Fatal("expected 24 native observations")
	}

	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "project"), root)
	index := loadTestIndex(t, root)
	restore := EnableDiskCacheForTesting()
	defer restore()
	InvalidateRuntimeCaches()
	defer InvalidateRuntimeCaches()

	for pass, route := range []string{"cold", "memory", "disk"} {
		t.Run(route, func(t *testing.T) {
			if pass == 2 {
				InvalidateRuntimeCaches()
			}
			ResetPerfCounters()
			run := Run(index, Options{Parallelism: 1, PerfCounters: true})
			counts := make(map[string]int)
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					if result.ClassName != "SetterRead62" && result.ClassName != "SetterRead67" {
						t.Errorf("unexpected class %s", result.ClassName)
					}
					counts[result.ClassName+"."+result.MethodName]++
					switch result.MethodName {
					case "nullReject", "qualifiedSame", "sameMember":
						if result.Problem == nil || result.Problem.Type != "NullPointerException" || !strings.HasPrefix(result.Problem.Message, "Attempt to de-reference a null object") {
							t.Errorf("%s.%s: native null pointer, local result: %+v", result.ClassName, result.MethodName, result)
						}
					case "directBacking", "otherGetter", "sameHelper", "sameSafeObserve":
						if result.Status != "pass" {
							t.Errorf("%s.%s: native pass, local result: %+v", result.ClassName, result.MethodName, result.Problem)
						}
					default:
						t.Errorf("unexpected test %s.%s", result.ClassName, result.MethodName)
					}
				}
			}
			for _, class := range []string{"SetterRead62", "SetterRead67"} {
				for _, method := range []string{"nullReject", "qualifiedSame", "sameMember", "directBacking", "otherGetter", "sameHelper", "sameSafeObserve"} {
					if counts[class+"."+method] != 1 {
						t.Errorf("selected %d %s.%s cases, want 1", counts[class+"."+method], class, method)
					}
				}
			}
			phases := SnapshotPerfCounters().Phases
			if pass == 1 && phases.MemoryCacheHits != 1 {
				t.Error("warm runner missed memory cache")
			}
			if pass == 2 && phases.DiskCacheHits != 1 {
				t.Error("restored runner missed disk cache")
			}
		})
	}
}
