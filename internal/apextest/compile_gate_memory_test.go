package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// All units have their own error, even though their sources form a dense cycle.
// There is no usable dependency graph: own errors always take precedence. Keep
// preparation outside the allocation window so this guards the error path.
func compileGateRootMemoryFixture(n int) (compileGateUnits, map[string][]diagnostic.Diagnostic, *sourceCache) {
	units := compileGateUnits{classes: map[string][]string{}, fileOf: map[string]string{}, types: map[string]typesys.TypeSymbol{}}
	byFile := make(map[string][]diagnostic.Diagnostic, n)
	sources := newSourceCache()
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("MemoryUnit%05d", i)
		file := name + ".cls"
		units.order = append(units.order, file)
		units.classes[file] = []string{name}
		units.fileOf[strings.ToLower(name)] = file
		units.types[strings.ToLower(name)] = typesys.TypeSymbol{Name: name, File: file}
		var source strings.Builder
		fmt.Fprintf(&source, "public class %s { AbsentPkg.Tool absent;", name)
		for j := 1; j <= 16; j++ {
			fmt.Fprintf(&source, " MemoryUnit%05d peer%d;", (i+j)%n, j)
		}
		source.WriteString(" }")
		sources.files[file] = source.String()
		byFile[file] = []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "dependency_missing", File: file, Message: "unloaded declared dependency: AbsentPkg"}}
	}
	return units, byFile, sources
}

func newCompileGateMemoryState() *compileGate {
	return &compileGate{excludedFiles: map[string]bool{}, fileEntry: map[string]int{}, classEntry: map[string]int{}, rootProblems: map[string]string{}}
}

func TestCompileGateMissingDependencyRootsAllocation(t *testing.T) {
	const n = 256
	units, byFile, sources := compileGateRootMemoryFixture(n)
	gate := newCompileGateMemoryState()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if !gate.exclude(typesys.Index{}, units, byFile, sources) {
		t.Fatal("root attribution fell back")
	}
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("root gate: units=%d allocated=%d", n, allocated)
	// The report needs O(n) entries and diagnostic strings, not source ASTs or
	// edges between already-invalid roots. Leave ample room for Go map growth.
	if allocated > 4<<20 {
		t.Errorf("root gate allocated %d bytes; limit is 4 MiB for %d roots", allocated, n)
	}
	for _, file := range units.order {
		name := units.classes[file][0]
		entry := gate.excluded[gate.fileEntry[file]]
		if entry.ClassName != name || entry.Root != name || entry.DependsOn != "" || entry.Message != byFile[file][0].Message || gate.rootProblems[file] != indexCompileError(byFile[file]).Error() {
			t.Errorf("root diagnostics changed: %+v", entry)
		}
	}
	if len(gate.excluded) != n {
		t.Fatalf("excluded %d units, want %d", len(gate.excluded), n)
	}
}

func TestCompileGateMissingDependencyRootDiagnostics(t *testing.T) {
	units, byFile, sources := compileGateRootMemoryFixture(3)
	// One compile unit can own several declarations and several diagnostics.
	// Neither the namespace/code nor the first diagnostic is a sufficient
	// shortcut: preserve all structured and native report fields.
	file := units.order[0]
	units.classes[file] = append(units.classes[file], "Companion")
	line := 17
	byFile[file] = append(byFile[file], diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA008", File: file, Message: "local detail", NativeMessage: "native detail", NativeLine: &line})
	gate := newCompileGateMemoryState()
	if !gate.exclude(typesys.Index{}, units, byFile, sources) {
		t.Fatal("root attribution fell back")
	}
	if len(gate.excluded) != 4 {
		t.Fatalf("excluded=%+v", gate.excluded)
	}
	for _, name := range units.classes[file] {
		entry := gate.excluded[gate.classEntry[strings.ToLower(name)]]
		if entry.Root != name || entry.DependsOn != "" || len(entry.Problems) != 2 || entry.Problems[1].Line != line || entry.Problems[1].Message != "native detail" {
			t.Fatalf("multi-declaration diagnostics changed: %+v", entry)
		}
	}
	if got, want := gate.rootProblems[file], indexCompileError(byFile[file]).Error(); got != want {
		t.Fatalf("root problem=%q want %q", got, want)
	}
}

func BenchmarkCompileGateMissingDependencyRoots(b *testing.B) {
	units, byFile, sources := compileGateRootMemoryFixture(2500)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gate := newCompileGateMemoryState()
		if !gate.exclude(typesys.Index{}, units, byFile, sources) || len(gate.excluded) != 2500 {
			b.Fatal("root diagnostics changed")
		}
	}
}

// The same owned project is exercised with and without its declared sibling.
// Set GLADE_DEPENDENCY_MEMORY_CLASSES=2500 for isolated RSS measurements; the
// ordinary lane uses a small project and still checks every exclusion's text.
func TestCompileGateMissingDependencyProjectMemory(t *testing.T) {
	n := 24
	if raw := os.Getenv("GLADE_DEPENDENCY_MEMORY_CLASSES"); raw != "" {
		var err error
		n, err = strconv.Atoi(raw)
		if err != nil || n < 2 || n > 5000 {
			t.Fatalf("invalid GLADE_DEPENDENCY_MEMORY_CLASSES: %q", raw)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, dependency := range []string{"missing", "present"} {
				t.Run(dependency, func(t *testing.T) {
					root := t.TempDir()
					consumer := filepath.Join(root, "consumer")
					writeFile(t, filepath.Join(consumer, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"classes","default":true,"dependencies":[{"package":"AbsentPkg"}]}]}`, api))
					writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"AbsentPkg:../support\"]\n")
					for i := 0; i < n; i++ {
						name := fmt.Sprintf("MemoryUnit%05d", i)
						var source strings.Builder
						fmt.Fprintf(&source, "public class %s { public AbsentPkg.Tool absent;", name)
						for j := 1; j <= 8; j++ {
							fmt.Fprintf(&source, " public MemoryUnit%05d peer%d;", (i+j)%n, j)
						}
						source.WriteString(" }")
						writeFile(t, filepath.Join(consumer, "classes", name+".cls"), source.String())
					}
					writeFile(t, filepath.Join(consumer, "classes/MemoryTest.cls"), `@IsTest private class MemoryTest { AbsentPkg.Tool absent; @IsTest static void observe() { System.assertEquals(1,1); } }`)
					if dependency == "present" {
						writeFile(t, filepath.Join(root, "support/sfdx-project.json"), fmt.Sprintf(`{"namespace":"AbsentPkg","sourceApiVersion":%q,"packageDirectories":[{"path":"classes","default":true}]}`, api))
						writeFile(t, filepath.Join(root, "support/classes/Tool.cls"), `global class Tool {}`)
					}
					p, err := project.Load(consumer)
					if err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(p, gladeschema.Schema{})
					run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
					if dependency == "present" {
						if !run.Summary().OK() || len(run.ExcludedClasses) != 0 || run.Summary().Total != 1 {
							t.Fatalf("healthy project: %+v", run)
						}
					} else {
						if len(run.ExcludedClasses) != n+1 || run.Summary().Total != 1 {
							t.Fatalf("missing dependency: exclusions=%d summary=%+v", len(run.ExcludedClasses), run.Summary())
						}
						for _, entry := range run.ExcludedClasses {
							if entry.Root != entry.ClassName || entry.DependsOn != "" || len(p.DependencyDiagnostics) != 1 || entry.Message != p.DependencyDiagnostics[0].Message {
								t.Errorf("root diagnostic changed: %+v; resolver=%+v", entry, p.DependencyDiagnostics)
							}
						}
						if run.Suites[0].Cases[0].Status != testreport.StatusCompileError {
							t.Fatal("missing dependency ran the test")
						}
					}
					// Record full normalized outcomes, including every diagnostic, for
					// A/B comparisons. Only elapsed time and temporary roots vary.
					run.DurationMS = 0
					for i := range run.Suites {
						run.Suites[i].DurationMS = 0
						for j := range run.Suites[i].Cases {
							run.Suites[i].Cases[j].DurationMS = 0
						}
					}
					raw, err := json.Marshal(run)
					if err != nil {
						t.Fatal(err)
					}
					normalized := strings.ReplaceAll(string(raw), root, "$ROOT")
					t.Logf("project outcomes: classes=%d dependency=%s sha256=%x", n, dependency, sha256.Sum256([]byte(normalized)))
				})
			}
		})
	}
}
