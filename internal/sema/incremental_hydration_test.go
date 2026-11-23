package sema

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// incrementalHydrationProject writes types whose diagnostics depend on what an
// earlier type hydrated. Asset's catalog fields such as InstallDate become
// visible only once some body hydrates Asset, and a StandardSetController
// getRecord() field access is accepted when any visible type has the field.
func incrementalHydrationProject(t *testing.T, root string) {
	t.Helper()
	incrementalWrite(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	meta := `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`
	add := func(name, body string) {
		incrementalWrite(t, root, "force-app/main/default/classes/"+name+".cls", body)
		incrementalWrite(t, root, "force-app/main/default/classes/"+name+".cls-meta.xml", meta)
	}
	add("AHydrator", incrementalHydratorSource(true))
	add("BReader", `public class BReader {
    public static Object read(ApexPages.StandardSetController ssc) {
        Integer marker = 1;
        return ssc.getRecord().InstallDate;
    }
}
`)
	add("CPlain", `public class CPlain {
    public static Integer value() {
        Integer v = 1;
        return v;
    }
}
`)
	add("DReader", `public class DReader {
    public static Object read(ApexPages.StandardSetController ssc) {
        return ssc.getRecord().InstallDate;
    }
}
`)
}

func incrementalHydratorSource(hydrates bool) string {
	statement := "Integer unused = 1;"
	if hydrates {
		statement = "String name = Schema.Asset.SerialNumber.getDescribe().getName();"
	}
	return "public class AHydrator {\n    public static void run() {\n        " + statement + "\n    }\n}\n"
}

func incrementalHasCode(result Result, file, text string) bool {
	for _, d := range result.Diagnostics {
		if strings.HasSuffix(d.File, file) && strings.Contains(d.Message, text) {
			return true
		}
	}
	return false
}

// TestIncrementalHydrationHistoryAcrossTypes pins the cross-type case: a
// type's cached diagnostics depend on fields an earlier type hydrated. Every
// generation must equal a full analysis, and reuse must still engage.
func TestIncrementalHydrationHistoryAcrossTypes(t *testing.T) {
	for _, procs := range []int{1, 4} {
		procs := procs
		t.Run(fmt.Sprintf("gomaxprocs%d", procs), func(t *testing.T) {
			previous := runtime.GOMAXPROCS(procs)
			defer runtime.GOMAXPROCS(previous)
			root := t.TempDir()
			incrementalHydrationProject(t, root)
			state := NewIncrementalState()
			analyze := func(label string) (Result, IncrementalStats) {
				t.Helper()
				gen := incrementalLoad(t, root)
				opts := incrementalOptions(&gen.artifacts)
				got, stats := state.Analyze(gen.index, opts)
				want := AnalyzeWithOptions(gen.index, opts)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: incremental result differs from full analysis (stats %+v)\n got: %v\nwant: %v", label, stats, got.Diagnostics, want.Diagnostics)
				}
				return got, stats
			}
			edit := func(file, text string) {
				incrementalWrite(t, root, "force-app/main/default/classes/"+file, text)
			}
			const missing = "Variable does not exist: InstallDate"
			got, _ := analyze("baseline")
			if incrementalHasCode(got, "BReader.cls", missing) || incrementalHasCode(got, "DReader.cls", missing) {
				t.Fatalf("baseline: readers report %q although AHydrator hydrates Asset: %v", missing, got.Diagnostics)
			}
			if _, stats := analyze("unchanged"); !stats.Incremental || stats.Rechecked != 0 || stats.Reused == 0 {
				t.Fatalf("unchanged generation stats = %+v, want every type reused despite history-dependent answers", stats)
			}
			// The only provider stops hydrating Asset: both readers flip.
			edit("AHydrator.cls", incrementalHydratorSource(false))
			got, stats := analyze("provider removed")
			if !incrementalHasCode(got, "BReader.cls", missing) || !incrementalHasCode(got, "DReader.cls", missing) {
				t.Fatalf("provider removed: readers do not report %q: %v", missing, got.Diagnostics)
			}
			if !stats.Incremental || stats.Reused == 0 {
				t.Fatalf("provider removed stats = %+v, want an incremental run that reuses CPlain", stats)
			}
			// The provider returns: the cached "unknown" answers flip back.
			edit("AHydrator.cls", incrementalHydratorSource(true))
			if got, stats := analyze("provider restored"); incrementalHasCode(got, "BReader.cls", missing) || !stats.Incremental {
				t.Fatalf("provider restored stats = %+v diagnostics %v", stats, got.Diagnostics)
			}
			// A reader edit: its answer comes from the reused provider.
			edit("BReader.cls", strings.Replace(`public class BReader {
    public static Object read(ApexPages.StandardSetController ssc) {
        Integer marker = 1;
        return ssc.getRecord().InstallDate;
    }
}
`, "marker = 1", "marker = 2", 1))
			if got, stats := analyze("reader edited"); incrementalHasCode(got, "BReader.cls", missing) || !stats.Incremental || stats.Rechecked != 1 {
				t.Fatalf("reader edited stats = %+v diagnostics %v, want only BReader rechecked", stats, got.Diagnostics)
			}
			// An unrelated edit keeps both readers cached.
			edit("CPlain.cls", strings.Replace(`public class CPlain {
    public static Integer value() {
        Integer v = 1;
        return v;
    }
}
`, "v = 1", "v = 2", 1))
			if _, stats := analyze("unrelated edit"); !stats.Incremental || stats.Rechecked != 1 {
				t.Fatalf("unrelated edit stats = %+v, want only CPlain rechecked", stats)
			}
		})
	}
}

func TestSemaValidateHydration(t *testing.T) {
	record := func(typeIndex int, yes, no, self []string) *semaTypeHydration {
		rec := &semaTypeHydration{typeIndex: typeIndex, touched: map[string]map[string]typesys.MemberSymbol{}}
		for _, name := range yes {
			semaAddName(&rec.yes, name)
		}
		for _, name := range no {
			semaAddName(&rec.no, name)
		}
		fields := map[string]typesys.MemberSymbol{}
		for _, name := range self {
			semaAddName(&rec.self, name)
			fields[name] = typesys.MemberSymbol{}
		}
		rec.touched["entry"] = fields
		return rec
	}
	pre := map[string]struct{}{"prefield": {}}
	// A reused "known" answer needs an earlier source.
	records := []*semaTypeHydration{record(0, nil, nil, []string{"f"}), record(1, []string{"f", "prefield"}, nil, nil)}
	if check := semaValidateHydration(records, []bool{false, false}, pre); check.reruns != 0 || check.conflict || check.firstSelf["f"] != 0 {
		t.Fatalf("earlier source: %+v", check)
	}
	records = []*semaTypeHydration{record(0, []string{"f"}, nil, nil), record(1, nil, nil, []string{"f"})}
	if check := semaValidateHydration(records, []bool{false, false}, pre); !check.rerun[0] || check.reruns != 1 {
		t.Fatalf("later source must not settle an earlier answer: %+v", check)
	}
	// A reused "unknown" answer fails once an earlier type touches the field.
	records = []*semaTypeHydration{record(0, nil, nil, []string{"f"}), record(1, nil, []string{"f"}, nil)}
	if check := semaValidateHydration(records, []bool{true, false}, pre); !check.rerun[1] || check.conflict {
		t.Fatalf("contradicted unknown: %+v", check)
	}
	// A rechecked "unknown" answer that an earlier type touched but did not
	// make visible cannot be settled.
	records = []*semaTypeHydration{record(0, nil, nil, nil), record(1, nil, []string{"f"}, nil)}
	records[0].touched["entry"] = map[string]typesys.MemberSymbol{"f": {}}
	if check := semaValidateHydration(records, []bool{false, true}, pre); !check.conflict {
		t.Fatalf("unsettled rechecked unknown: %+v", check)
	}
	// An invalid rechecked type taints later types of its worker and round.
	records = []*semaTypeHydration{record(0, nil, nil, []string{"f"}), record(1, nil, []string{"f"}, nil), record(2, nil, nil, nil), record(3, nil, nil, nil)}
	records[3].worker = 1
	check := semaValidateHydration(records, []bool{false, true, true, true}, pre)
	if !check.rerun[1] || !check.rerun[2] || check.rerun[3] || check.reruns != 2 {
		t.Fatalf("taint: %+v", check)
	}
}
