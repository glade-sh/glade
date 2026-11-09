package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestApprovalIdentifierNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/approval_identifier"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				sources := map[string]string{}
				err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
					if err != nil {
						return err
					}
					if strings.HasPrefix(rel, ".sf/") || strings.HasPrefix(rel, ".sfdx/") {
						return nil
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.HasSuffix(rel, "-meta.xml") || rel == "sfdx-project.json" {
						data = []byte(strings.ReplaceAll(string(data), "62.0", api))
					}
					sources[rel] = string(data)
					writeFile(t, filepath.Join(root, rel), string(data))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				source := "System.assert(false, 'IDENTIFIER_ROWS|' + JSON.serialize(ApprovalIdentifierCapture.observe()));"
				if route == "istest" {
					source = "ApprovalIdentifierCaptureTest.capture();"
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				if route == "istest" {
					machine.EnableTestContext()
					if err := RegisterTestMethodForRequest(machine, index, "ApprovalIdentifierCaptureTest", "capture"); err != nil {
						t.Fatal(err)
					}
				}
				if route == "istest" {
					_, err = machine.ExecuteInClass(program, "ApprovalIdentifierCaptureTest")
				} else {
					_, err = machine.Execute(program)
				}
				var failure *vm.RuntimeError
				if !errors.As(err, &failure) {
					t.Fatalf("missing observation: %v", err)
				}
				_, payload, ok := strings.Cut(failure.ExceptionMessage(), "IDENTIFIER_ROWS|")
				if !ok {
					t.Fatalf("missing observation: %v", err)
				}
				var actual map[string]any
				if err := json.Unmarshal([]byte(payload), &actual); err != nil {
					t.Fatal(err)
				}
				capture := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route)
				run, err := os.ReadFile(filepath.Join(capture, "RUN-FACTS.txt"))
				if err != nil {
					t.Fatal(err)
				}
				for _, fact := range []string{"api=" + api, "context=" + route, "rows=7", "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+string(run), "\n"+fact+"\n") {
						t.Fatalf("missing %s", fact)
					}
				}
				for name, source := range sources {
					fact := fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256([]byte(source)))
					if !strings.Contains(string(run), fact) {
						t.Fatalf("capture source changed: %s", name)
					}
				}
				data, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				rows := strings.Split(strings.TrimSpace(string(data)), "\n")
				if len(rows) != 8 {
					t.Fatalf("expected seven rows, got %d", len(rows)-1)
				}
				for _, line := range rows[1:] {
					key, value, ok := strings.Cut(line, "\t")
					if !ok {
						t.Fatal(line)
					}
					var want any
					if err := json.Unmarshal([]byte(value), &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(actual[key], want) {
						t.Errorf("%s: got %v want %v", key, actual[key], want)
					}
				}
				if len(actual) != len(rows)-1 {
					t.Fatal(fmt.Sprintf("unexpected observation count %d", len(actual)))
				}
			})
		}
	}
}
