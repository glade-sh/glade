package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardExtUpsertAnonymousAndWarmRoutes(t *testing.T) {
	dir := filepath.Join("testdata", "native-capture", "standard_ext_upsert_anonymous")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 {
			t.Fatal("bad native row")
		}
		if rows[f[0]] == nil {
			rows[f[0]] = map[string]string{}
		}
		rows[f[0]][f[1]] = f[2]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(dir, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(dir, "project"), path)
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			assertRows := func(observed string) {
				t.Helper()
				got := map[string]string{}
				for _, line := range strings.Split(observed, "\n") {
					k, v, ok := strings.Cut(line, "=")
					if ok {
						got[k] = v
					}
				}
				if len(rows[api]) != 48 {
					t.Fatalf("native rows %d", len(rows[api]))
				}
				for k, want := range rows[api] {
					if got[k] != want {
						t.Errorf("%s got %q want %q", k, got[k], want)
					}
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				p, err := vm.CompileAnonymousWithOptions("String observation=String.join(ExtProbe.run(),'\\n');", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				if _, err := machine.Execute(p); err != nil {
					t.Fatal(err)
				}
				assertRows(machine.Globals["observation"].Text)
			})
			t.Run("warm_project", func(t *testing.T) {
				opts := Options{SelectedClasses: []string{"ExtProbeTest"}, SelectedMethod: "extProbe", NoDiskCache: true, Parallelism: 1}
				Run(index, opts)
				ResetPerfCounters()
				opts.PerfCounters = true
				run := Run(index, opts)
				if p := SnapshotPerfCounters().Phases; p.MemoryCacheHits != 1 || p.CacheMisses != 0 {
					t.Fatalf("warm cache: %+v", p)
				}
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				_, s, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED_EXT\n")
				if !ok {
					t.Fatal("missing marker")
				}
				assertRows(s)
			})
		})
	}
}
