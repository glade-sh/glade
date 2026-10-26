package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardFieldOverlayNativeConformanceRoutes(t *testing.T) {
	const fixture = "testdata/conformance/standard_field_overlay"
	routes := []string{"constructor", "member", "stringAccess", "tokenAccess", "selectStatic", "selectDynamic", "insertFields", "updateFields", "describeAccount", "describeLead", "describeContact", "describeOrder"}
	for _, api := range []string{"62.0", "67.0"} {
		for _, state := range []string{"projectSource", "coldAnonymous"} {
			t.Run(api+"/"+state, func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join(fixture, "project")
				if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					relative, err := filepath.Rel(project, path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "67.0", api))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				native := filepath.Join(fixture, "native", "api"+api[:2])
				receipt, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
				if err != nil || !strings.Contains(string(receipt), "source_api="+api+"\n") || !strings.Contains(string(receipt), "native_complete=true\n") {
					t.Fatalf("incomplete native capture: %v", err)
				}
				for _, line := range strings.Split(string(receipt), "\n") {
					path, digest, ok := strings.Cut(line, " sha256=")
					if !ok {
						continue
					}
					body, err := os.ReadFile(filepath.Join(fixture, path))
					if err != nil {
						t.Fatal(err)
					}
					sum := sha256.Sum256(body)
					if hex.EncodeToString(sum[:]) != digest {
						t.Fatal("native source changed: " + path)
					}
				}
				table, err := os.ReadFile(filepath.Join(native, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				rows := strings.Split(strings.TrimSpace(string(table)), "\n")
				if len(rows) != 1+2*len(routes) || rows[0] != "source_api\tentry\troute\tobservation" {
					t.Fatal("incomplete native matrix")
				}
				index := loadTestIndex(t, root)
				if state == "coldAnonymous" {
					index = nativeBoundaryIndex(t, api, nil, nil)
				}
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native project compiled; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
				seen := map[string]bool{}
				for _, row := range rows[1:] {
					columns := strings.SplitN(row, "\t", 4)
					if len(columns) != 4 || columns[0] != api {
						t.Fatalf("invalid native row: %s", row)
					}
					entry, route := columns[1], columns[2]
					if state == "coldAnonymous" && entry == "isTest" {
						continue
					}
					key := entry + "/" + route
					if seen[key] {
						t.Fatal("duplicate native row: " + key)
					}
					seen[key] = true
					t.Run(key, func(t *testing.T) {
						var message string
						if entry == "isTest" {
							run := Run(index, Options{SelectedClasses: []string{"StandardFieldOverlayTest"}, SelectedMethod: route, NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("expected one project observation: %+v", run.Summary())
							}
							result := run.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", result.Problem)
							}
							message = result.Problem.Message
						} else if entry == "anonymous" {
							body, err := os.ReadFile(filepath.Join(fixture, "anonymous", route+".apex"))
							if err != nil {
								t.Fatal(err)
							}
							if analysis := sema.AnalyzeAnonymous(index, string(body), api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, execErr := runner.newMachine().Execute(program)
							var fault *vm.RuntimeError
							if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
								t.Fatalf("missing anonymous observation: %v", execErr)
							}
							message = fault.ExceptionMessage()
						} else {
							t.Fatal("unknown entry: " + entry)
						}
						_, payload, ok := strings.Cut(message, "FIELD_OVERLAY|"+route+"|")
						var got, want map[string]any
						if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[3]), &want) != nil || len(want) != map[string]int{"describeAccount": 5, "describeLead": 5, "describeContact": 1, "describeOrder": 6, "constructor": 17, "member": 17, "stringAccess": 17, "tokenAccess": 17, "selectStatic": 17, "selectDynamic": 17, "insertFields": 17, "updateFields": 17}[route] {
							t.Fatalf("invalid observation: %s", message)
						}
						if !reflect.DeepEqual(got, want) {
							t.Errorf("native=%s; Glade=%s", columns[3], payload)
						}
					})
				}
				for _, route := range routes {
					for _, entry := range []string{"anonymous", "isTest"} {
						if state == "coldAnonymous" && entry == "isTest" {
							continue
						}
						if !seen[entry+"/"+route] {
							t.Fatal("missing native route: " + entry + "/" + route)
						}
					}
				}
			})
		}
	}
}
