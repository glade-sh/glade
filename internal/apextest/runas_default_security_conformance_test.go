package apextest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type runAsDefaultSecurityCell struct {
	NativeStatus    string `json:"nativeStatus"`
	AssertionStatus string `json:"assertionStatus"`
	Native          struct {
		Outcome string         `json:"outcome"`
		Value   map[string]any `json:"value"`
	} `json:"native"`
	CarryReason   string            `json:"carryReason"`
	CarriedFields map[string]string `json:"carriedFields"`
	NativeObject  string            `json:"nativeObject"`
}

// Same-release C137 captures bracket 66 -> 67 on both entry routes. runAs
// operations are named-route only; anonymous rows capture outside-runAs twins.
// No Salesforce calls or external fixture dependencies are used by this test.
func TestRunAsDefaultSecurityConformance(t *testing.T) {
	var data struct {
		APIVersions []string       `json:"apiVersions"`
		ShapeCount  int            `json:"shapeCount"`
		Counts      map[string]int `json:"counts"`
		Fixtures    map[string]struct {
			Object             string            `json:"object"`
			Field              string            `json:"field"`
			Permission         string            `json:"permission"`
			GrantedPermission  string            `json:"granted_permission"`
			ReadonlyPermission string            `json:"readonly_permission"`
			Files              map[string]string `json:"files"`
		} `json:"fixtures"`
		Rows []struct {
			ID           string                              `json:"id"`
			Route        string                              `json:"route"`
			Source       userModeInaccessibleSource          `json:"source"`
			SourceSHA256 map[string]string                   `json:"sourceSha256"`
			Cells        map[string]runAsDefaultSecurityCell `json:"cells"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/runas_default_security/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	wantCounts := map[string]int{"ASSERTED": 1714, "CARRIED": 146, "NO_ANSWER": 30}
	if data.ShapeCount != 378 || len(data.Rows) != 378 || !reflect.DeepEqual(data.Counts, wantCounts) ||
		strings.Join(data.APIVersions, ",") != "59.0,62.0,66.0,67.0,68.0" || len(data.Fixtures) != 5 {
		t.Fatal("incomplete native matrix")
	}
	actual := map[string]int{}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if !strings.HasPrefix(row.ID, "C137_") || seen[row.ID] || len(row.Cells) != 5 || (row.Route != "anonymous" && row.Route != "istest") {
			t.Fatal("invalid row or route")
		}
		seen[row.ID] = true
		texts := map[string]string{}
		if row.Route == "anonymous" {
			texts["anonymous"] = row.Source.Anonymous
		} else {
			for name, source := range row.Source.Classes {
				texts["class:"+name] = source
			}
		}
		if len(texts) == 0 || len(texts) != len(row.SourceSHA256) {
			t.Fatal("missing synthetic source")
		}
		for name, source := range texts {
			hash := sha256.Sum256([]byte(source))
			if source == "" || hex.EncodeToString(hash[:]) != row.SourceSHA256[name] {
				t.Fatalf("captured source changed: %s/%s", row.ID, name)
			}
		}
		for _, api := range data.APIVersions {
			cell, ok := row.Cells[api]
			if !ok {
				t.Fatal("missing API cell")
			}
			switch cell.NativeStatus {
			case "NO_ANSWER":
				if cell.AssertionStatus != "" || cell.CarryReason == "" || cell.Native.Outcome != "NO_ANSWER" {
					t.Fatal("non-answer admitted")
				}
				actual["NO_ANSWER"]++
			case "OBSERVED":
				if cell.Native.Outcome != "runtime" || len(cell.Native.Value) == 0 {
					t.Fatal("missing observation")
				}
				if cell.CarryReason != "" {
					if cell.AssertionStatus != "" {
						t.Fatal("carried observation asserted")
					}
					actual["CARRIED"]++
				} else if cell.AssertionStatus == "ASSERTED" {
					actual["ASSERTED"]++
				} else {
					t.Fatal("observation silently omitted")
				}
			default:
				t.Fatal("unknown native status")
			}
		}
	}
	if !reflect.DeepEqual(actual, wantCounts) {
		t.Fatalf("native cells = %v, want %v", actual, wantCounts)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "RunAsDefaultSecurity", api)
			f := data.Fixtures[api]
			metadata := map[string]string{}
			for path, source := range f.Files {
				metadata[path] = strings.ReplaceAll(source, "[URL]", "http://soap.sforce.com/2006/04/metadata")
			}
			fixture := userModeInaccessibleFixture{Names: map[string]string{"running": f.Permission}, Metadata: metadata}
			baseIndex := userModeInaccessibleIndex(t, api, fixture, nil)
			orgs := map[string]storage.OrgState{}
			bases := map[string]*conformanceRunner{}
			for _, route := range []string{"anonymous", "istest"} {
				org := userModeInaccessibleOrg(t, baseIndex, fixture, route)
				// BASELINE records Account OWD Edit; custom ReadWrite is in metadata.
				account := org.Objects["Account"]
				account.Definition.SharingModel = "ReadWrite"
				org.Objects["Account"] = account
				orgs[route] = org
				bases[route] = newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			}
			bind := strings.NewReplacer("SyntheticMode__c", f.Object, "Value__c", f.Field, "SyntheticGranted", f.GrantedPermission, "SyntheticReadonly", f.ReadonlyPermission)
			for _, row := range data.Rows {
				t.Run(row.ID+"/"+row.Route, func(t *testing.T) {
					cell := row.Cells[api]
					if cell.AssertionStatus != "ASSERTED" {
						counts.TrackKind(t, row.Route, "carried", 1)
						t.Logf("%s: %s", cell.NativeStatus, cell.CarryReason)
						return
					}
					kind := "exact"
					if len(cell.CarriedFields) != 0 {
						kind = "partial"
					}
					counts.TrackKind(t, row.Route, kind, 1)
					classes := map[string]string{}
					for name, source := range row.Source.Classes {
						classes[name] = bind.Replace(source)
					}
					index := userModeInaccessibleIndex(t, api, fixture, classes)
					org := orgs[row.Route]
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: bases[row.Route], LinkProject: true, Org: &org})
					source := row.Source
					source.Anonymous = bind.Replace(source.Anonymous)
					result, execErr := runAsDefaultSecurityExecute(t, api, row.Route, source, index, runner)
					got := runAsDefaultSecurityPayload(t, row.ID, result, execErr)
					if et, ok := got["et"].(string); ok {
						got["et"] = userModeInaccessibleExceptionType(et)
					}
					want := cell.Native.Value
					if cell.NativeObject != "" {
						// Supplemental captures use fresh metadata names. Keep the
						// native payload and bind only its object name for comparison.
						encoded, err := json.Marshal(want)
						if err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal([]byte(strings.ReplaceAll(string(encoded), cell.NativeObject, f.Object)), &want); err != nil {
							t.Fatal(err)
						}
					}
					for field, reason := range cell.CarriedFields {
						if field != "em" || reason == "" {
							t.Fatal("invalid carried diagnostic")
						}
						t.Logf("carried %s: %s; native=%v", field, reason, want[field])
						delete(got, field)
						delete(want, field)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
				})
			}
		})
	}
}

func runAsDefaultSecurityExecute(t *testing.T, api, route string, source userModeInaccessibleSource, index typesys.Index, runner *conformanceRunner) (vm.Result, error) {
	t.Helper()
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native accepts; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	if route == "anonymous" {
		analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
		program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
		if analysis.HasErrors() || err != nil {
			t.Fatalf("native accepts; Glade=%v %v", analysis.Diagnostics, err)
		}
		return runner.execute(program)
	}
	cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
	if len(source.Tests) != 1 || len(cases) != 1 {
		t.Fatal("missing captured test method")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("compilation: %v %v", methodErrors[key], programErrors[key])
	}
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	return machine.ExecuteInClass(programs[key], cases[0].ClassName)
}

func runAsDefaultSecurityPayload(t *testing.T, id string, result vm.Result, err error) map[string]any {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) || userModeInaccessibleExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing observation: %v", err)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, "C137_RESULT|"+id+"|")
	if !found {
		_, header, ok := strings.Cut(message, "C137_HASH|"+id+"|")
		parts := strings.Split(header, "|")
		if !ok || len(parts) != 2 {
			t.Fatalf("missing owned marker: %s", message)
		}
		count, err := strconv.Atoi(parts[0])
		if err != nil || count < 1 {
			t.Fatal("invalid chunk count")
		}
		chunks := map[int]string{}
		for _, line := range result.Debug {
			if tail, ok := strings.CutPrefix(line, "C137_CHUNK|"+id+"|"); ok {
				number, value, ok := strings.Cut(tail, "|")
				i, err := strconv.Atoi(number)
				if !ok || err != nil || i < 0 || i >= count || chunks[i] != "" {
					t.Fatal("invalid observation chunk")
				}
				chunks[i] = value
			}
		}
		if len(chunks) != count {
			t.Fatal("missing observation chunks")
		}
		var encoded strings.Builder
		for i := 0; i < count; i++ {
			encoded.WriteString(chunks[i])
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded.String())
		hash := sha256.Sum256(decoded)
		if err != nil || hex.EncodeToString(hash[:]) != parts[1] {
			t.Fatal("observation hash mismatch")
		}
		payload = string(decoded)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		t.Fatal(fmt.Errorf("invalid observation: %w", err))
	}
	return value
}
