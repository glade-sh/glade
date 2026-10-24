package vm_test

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestIdleCollectionReturnNativeProjectRoutesBoundReceiverWork(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	family := filepath.Join("..", "apextest", "testdata", "conformance", "idle_collection_returns")
	file, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native API and entry-route matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] || (api != "62.0" && api != "67.0") || (route != "anonymous" && route != "isTest") {
			t.Fatalf("invalid native cell %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(family, "project")
			err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				target := filepath.Join(root, relative)
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0644)
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
				t.Fatal(err)
			}
			source := "IdleCollectionProbeTest.observe();"
			if route == "anonymous" {
				body, err := os.ReadFile(filepath.Join(family, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				source = string(body)
			} else {
				machine.EnableTestContext()
				if err := apextest.RegisterTestMethodForRequest(machine, index, "IdleCollectionProbeTest", "observe"); err != nil {
					t.Fatal(err)
				}
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			visits := vm.ObserveIdleCollectionRefreshForTest(machine)
			observed := ""
			if route == "anonymous" {
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, ok := strings.Cut(line, "OBS|"); ok {
						if observed != "" {
							t.Fatal("duplicate observation")
						}
						observed = value
					}
				}
			} else {
				_, err := machine.ExecuteTestInClass(program, "IdleCollectionProbeTest")
				if err == nil || !strings.Contains(err.Error(), "AssertException") {
					t.Fatalf("missing native assertion transport: %v", err)
				}
				_, observed, _ = strings.Cut(err.Error(), "OBS|")
			}
			var got, want any
			if json.NewDecoder(strings.NewReader(observed)).Decode(&got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid native observation: %q", observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
			t.Logf("native observation matched; project receiver visits=%d", visits())
			// Allow conservative scans for setup and mutation controls, while
			// excluding repeated whole-receiver scans during the idle reads.
			if visits() > 1024 {
				t.Fatalf("project receiver visits=%d; want <=1024 across read and mutation routes", visits())
			}
		})
	}
}
