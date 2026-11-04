package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestCase313NativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/case313_native"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, ent fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if ent.IsDir() {
					if ent.Name() == ".sf" {
						return filepath.SkipDir
					}
					return nil
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(b), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			t.Run("anonymous", func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := newConformanceRunner(t, index, conformanceRunnerOptions{}).newMachine()
				_, err = machine.Execute(program)
				if err == nil || !strings.Contains(err.Error(), "System.runAs can only be used within a test method") {
					t.Fatalf("native runAs rejection, got %v", err)
				}
			})
			results := Run(index, Options{SelectedClasses: []string{"Case313Capture"}, NoDiskCache: true, Parallelism: 1})
			if len(results.Suites) != 1 {
				t.Fatalf("missing suite: %+v", results.Summary())
			}
			for _, row := range rows[1:] {
				if row[0] != api || row[1] != "istest" {
					continue
				}
				t.Run(row[2], func(t *testing.T) {
					for _, result := range results.Suites[0].Cases {
						if result.MethodName != row[2] {
							continue
						}
						if result.Problem == nil {
							t.Fatal("missing observation")
						}
						_, payload, found := strings.Cut(result.Problem.Message, "OBS|")
						var got map[string]any
						if !found || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("missing observation: %s", result.Problem.Message)
						}
						want := map[string]any{}
						if row[2] == "scalarDelete" {
							want["scalar"] = row[3]
						} else if row[2] == "listDelete" {
							want["list"] = row[3]
						} else {
							for _, item := range strings.Split(row[3], ";") {
								key, value, ok := strings.Cut(item, "=")
								if !ok {
									t.Fatal("invalid oracle")
								}
								if value == "true" {
									want[key] = true
								} else if value == "false" {
									want[key] = false
								} else {
									want[key] = value
								}
							}
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%v local=%v", want, got)
						}
						return
					}
					t.Fatal("native method not discovered")
				})
			}
		})
	}
}
