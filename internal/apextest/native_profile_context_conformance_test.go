package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNativeProfileContextConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "native_profile_context")
	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, fixture, root)
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	InvalidateRuntimeCaches()
	t.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
	})
	file, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows=%d, error=%v; want header and four observations", len(rows), err)
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:2], "/")
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		for _, cache := range []string{"cold", "warm"} {
			t.Run(row[0]+"/"+row[1]+"/"+cache, func(t *testing.T) {
				className := "NativeProfileContext" + strings.TrimSuffix(row[0], ".0")
				const marker = "NATIVE_CONTEXT|"
				observation := ""
				if row[1] == "isTest" {
					InvalidateRuntimeCaches()
					if cache == "warm" {
						if _, ok := tryLoadDiskRuntime(index); !ok {
							t.Fatal("project runtime was not restored from disk")
						}
					}
					run := Run(index, Options{SelectedClasses: []string{className + "Proof"}, SelectedMethod: "observe", Parallelism: 1})
					if run.Summary().Total != 1 {
						t.Fatalf("selected observation count: %+v", run.Summary())
					}
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								_, observation, _ = strings.Cut(c.Problem.Message, marker)
							}
						}
					}
				} else {
					program, err := vm.CompileAnonymousWithOptions("System.debug('"+marker+"'+JSON.serialize("+className+".current()));", vm.CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					result, err := runner.execute(program)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range result.Debug {
						if strings.HasPrefix(line, marker) {
							observation = strings.TrimPrefix(line, marker)
						}
					}
				}
				var want, got any
				if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(observation), &got); err != nil {
					t.Fatalf("missing complete observation: %q (%v)", observation, err)
				}
				if !reflect.DeepEqual(want, got) {
					t.Fatalf("native=%s\nlocal=%s", row[2], observation)
				}
			})
		}
	}
}
