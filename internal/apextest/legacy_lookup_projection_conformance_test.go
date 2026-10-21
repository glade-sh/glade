package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func runLegacyLookupProjectionOrgConformance(t *testing.T, fixture, entryAPI string, carried int) {
	t.Helper()
	var data struct {
		API, Route, Context, EntryAPI string
		SourcePins                    map[string]string
		SourceSHA256                  map[string]string
		Rows                          []struct {
			ID     string
			Native json.RawMessage
		}
		Carries []struct {
			Count  int
			Reason string
		}
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.API != "62.0" || data.Route != "istest" || data.Context != "plain" || len(data.Rows) != 5 {
		t.Fatal("incomplete captured legacy projection cells")
	}
	if data.EntryAPI != "" && data.EntryAPI != entryAPI {
		t.Fatal("captured entry source version changed")
	}
	if !reflect.DeepEqual(data.SourcePins, map[string]string{"LegacyOmissionQuery": "24.0", "LegacyOmissionWrite": "39.0", "LegacyOmissionRead": "40.0", "LegacyOmissionObserver": entryAPI}) {
		t.Fatal("captured helper source versions changed")
	}
	for _, carry := range data.Carries {
		if carry.Count != carried || carry.Reason == "" {
			t.Fatal("missing explanation for unanswered cells")
		}
		t.Logf("%d native cells carried: %s", carry.Count, carry.Reason)
	}
	for _, row := range data.Rows {
		t.Run(row.ID, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name, want := range data.SourceSHA256 {
				source, err := os.ReadFile(filepath.Join(fixture, "fixture", name))
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(source)
				if hex.EncodeToString(sum[:]) != want {
					t.Fatalf("captured source changed: %s", name)
				}
				dest := filepath.Join(root, "force-app/main/default", name)
				if strings.HasSuffix(name, ".object") {
					dest = filepath.Join(root, "force-app/main/default/objects", name)
				}
				writeFile(t, dest, string(source))
			}
			// Replay the same observer, retaining independently compiled helper APIs.
			entry := filepath.Join(root, "force-app/main/default/classes/LegacyProjectionReplay.cls")
			writeFile(t, entry, fmt.Sprintf(`@IsTest private class LegacyProjectionReplay {@IsTest static void observe(){System.assert(false,'LEGACY_LOOKUP_OBSERVATION|'+JSON.serialize(LegacyOmissionObserver.observe('%s')));}}`, row.ID))
			writeFile(t, entry+"-meta.xml", "<ApexClass><apiVersion>"+entryAPI+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured source rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"LegacyProjectionReplay"}})
			if len(cases) != 1 {
				t.Fatalf("expected one replay, got %d", len(cases))
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("replay compilation: %v %v", methodErrors[key], programErrors[key])
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatalf("missing observation: %v", execErr)
			}
			_, text, ok := strings.Cut(fault.ExceptionMessage(), "LEGACY_LOOKUP_OBSERVATION|")
			if !ok {
				t.Fatalf("missing observation payload: %v", execErr)
			}
			var got, want any
			if err := json.Unmarshal([]byte(text), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(row.Native, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s\nlocal=%s", row.Native, text)
			}
		})
	}
}
