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
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type runtimeBoundaryCell struct {
	ID, API, Route, Status, Stage   string
	Sources, SourceSHA256           map[string]string
	BoundSources, BoundSourceSHA256 map[string]string
	OwnerSeed, OwnerSeedSHA256      string
	Native                          json.RawMessage
	EvidenceDigest                  string
	CarryReason                     string
	NativeEvidence                  json.RawMessage
}
type runtimeBoundaryFixture struct {
	Sources map[string]map[string]string
	SHA256  map[string]string
}

// Exact emitted sources preserve frame locations. Named chunk transports are
// read from their captureSeed record, without changing the measured Apex body.
func runRuntimeBoundaryNativeConformance(t *testing.T, group string) {
	t.Helper()
	t.Run(group, func(t *testing.T) {
		var data struct {
			Group               string
			APIVersions, Routes []string
			ObservationCount    int
			Fixtures            map[string]runtimeBoundaryFixture
			Cells               []runtimeBoundaryCell
		}
		raw, err := os.ReadFile("testdata/conformance/runtime_boundary_" + strings.ToLower(group) + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		total := map[string]int{"C02": 60, "C03": 60, "C04": 96, "C13": 42, "CTRL": 24}[group]
		if data.Group != group || data.ObservationCount != total || len(data.Cells) != total || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
			t.Fatal("incomplete native matrix")
		}
		for _, fixture := range data.Fixtures {
			for kind, sources := range fixture.Sources {
				for name, source := range sources {
					runtimeBoundaryHash(t, source, fixture.SHA256[kind+":"+name])
				}
			}
		}
		counts := map[string]int{}
		seen := map[string]bool{}
		for _, cell := range data.Cells {
			key := cell.API + "/" + cell.ID + "/" + cell.Route
			if seen[key] {
				t.Fatal("duplicate native cell", key)
			}
			seen[key] = true
			counts[cell.Stage]++
			t.Run(key, func(t *testing.T) {
				if cell.Stage == "carried" || cell.Stage == "pending" {
					if cell.CarryReason == "" || string(cell.Native) != "null" {
						t.Fatal("carry needs its reason and no fabricated native value")
					}
					if cell.Stage == "carried" && (len(cell.NativeEvidence) == 0 || len(cell.EvidenceDigest) != 64) {
						t.Fatal("settled carry needs native fault evidence")
					}
					t.Logf("%s: %s; native=%s", cell.Stage, cell.CarryReason, cell.NativeEvidence)
					return
				}
				if len(cell.EvidenceDigest) != 64 {
					t.Fatal("unbound native runtime answer")
				}
				for name, source := range cell.Sources {
					runtimeBoundaryHash(t, source, cell.SourceSHA256[name])
				}
				if cell.OwnerSeed != "" {
					runtimeBoundaryHash(t, cell.OwnerSeed, cell.OwnerSeedSHA256)
				}
				if len(cell.BoundSources) != 0 {
					for name, source := range cell.BoundSources {
						runtimeBoundaryHash(t, source, cell.BoundSourceSHA256[name])
					}
					cell.Sources = cell.BoundSources
				}
				fixture := data.Fixtures[cell.API]
				index := runtimeBoundaryProject(t, cell.API, fixture, cell)
				analysis := sema.Analyze(index)
				if cell.Status == "compile-rejected" {
					if cell.Stage != "partial" || cell.CarryReason == "" {
						t.Fatal("compile coordinate carry needs an explicit reason")
					}
					t.Log(cell.CarryReason)
					diagnostics := analysis.Diagnostics
					if cell.Route == "anonymous" {
						diagnostics = sema.AnalyzeAnonymous(index, cell.Sources["anonymous"], cell.API).Diagnostics
					}
					runtimeBoundaryCompileRejection(t, cell, diagnostics)
					return
				}
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native accepted; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = cell.API
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, RequestRuntime: &runtime})
				machine := runner.newMachine()
				execute := func(source string) error {
					t.Helper()
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: cell.API})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					return err
				}
				if cell.OwnerSeed != "" && cell.Route == "anonymous" {
					if err := execute(cell.OwnerSeed); err != nil {
						t.Fatal(err)
					}
				}
				var payload string
				if cell.Route == "anonymous" {
					source := cell.Sources["anonymous"]
					if group == "C13" {
						if err := execute(cell.Sources["enqueue"]); err != nil {
							t.Fatal(err)
						}
						var job string
						for _, row := range machine.Org.Objects["Account"].Records {
							name, _ := row.GetField("Name")
							if strings.HasSuffix(name.String, " ENQUEUE") {
								value, _ := row.GetField("Description")
								job = value.String
							}
						}
						if !runtimeBoundaryID.MatchString(job) {
							t.Fatalf("missing queued job identity %q", job)
						}
						if err := machine.DrainAsync(&vm.Result{}); err != nil {
							t.Fatal(err)
						}
						source = strings.ReplaceAll(cell.Sources["inspect"], "'[ID]'", "'"+job+"'")
					}
					payload = runtimeBoundaryObservation(t, cell, execute(source))
				} else {
					var name string
					for candidate := range cell.Sources {
						name = candidate
					}
					setup, setupErrors, programs, programErrors := compileTestSetupMethodsForClasses(index, map[string]bool{name: true})
					if setupErrors[name] != nil || programErrors[name] != nil {
						t.Fatalf("named setup compilation %v %v", setupErrors[name], programErrors[name])
					}
					machine.EnableTestContext()
					if len(programs[name]) != 0 {
						for _, method := range setup[name] {
							if err := machine.RegisterMethod(method); err != nil {
								t.Fatal(err)
							}
						}
						for _, program := range programs[name] {
							if _, err := machine.ExecuteTestInClass(program, name); err != nil {
								t.Fatal(err)
							}
						}
						for _, row := range machine.Org.Objects["Account"].Records {
							value, _ := row.GetField("Name")
							if value.String == "E51_PAYLOAD" {
								description, _ := row.GetField("Description")
								if payload != "" {
									t.Fatal("duplicate capture payload")
								}
								payload = description.String
							}
						}
						if payload == "" {
							t.Fatal("missing captured setup payload")
						}
					} else {
						cases := Discover(index, Options{SelectedClasses: []string{name}, SelectedMethod: "observe"})
						if len(cases) != 1 {
							t.Fatalf("ordinary method discovery %d", len(cases))
						}
						methods, methodErrors := compileTestMethodsForIndex(&index, cases)
						invokes, invokeErrors := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if methodErrors[key] != nil || invokeErrors[key] != nil {
							t.Fatalf("named compilation %v %v", methodErrors[key], invokeErrors[key])
						}
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(invokes[key], name)
						payload = runtimeBoundaryObservation(t, cell, err)
					}
				}
				var got, want any
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(cell.Native, &want); err != nil {
					t.Fatal(err)
				}
				runtimeBoundaryIdentities(t, group, want, got, "")
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("native %s; Glade %s", cell.Native, payload)
				}
			})
		}
		wantCounts := map[string]map[string]int{"C02": {"matched": 60}, "C03": {"matched": 60}, "C04": {"matched": 90, "carried": 6}, "C13": {"matched": 40, "pending": 2}, "CTRL": {"matched": 18, "partial": 6}}[group]
		if !reflect.DeepEqual(counts, wantCounts) {
			t.Fatalf("native disposition counts %v", counts)
		}
	})
}

func runtimeBoundaryHash(t *testing.T, source, want string) {
	t.Helper()
	sum := sha256.Sum256([]byte(source))
	if hex.EncodeToString(sum[:]) != want {
		t.Fatal("captured source bytes changed")
	}
}
func runtimeBoundaryProject(t *testing.T, api string, fixture runtimeBoundaryFixture, cell runtimeBoundaryCell) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for kind, sources := range fixture.Sources {
		for name, source := range sources {
			var path string
			switch kind {
			case "classes":
				path = filepath.Join(root, "force-app/main/default/classes", name+".cls")
			case "objects":
				path = filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml")
			case "triggers":
				path = filepath.Join(root, "force-app/main/default/triggers", name+".trigger")
			default:
				t.Fatal("unknown fixture source kind", kind)
			}
			writeFile(t, path, source)
			if kind == "classes" {
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			if kind == "triggers" {
				writeFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
			}
		}
	}
	if cell.Route == "istest" {
		for name, source := range cell.Sources {
			path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
			writeFile(t, path, source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		}
	}
	return loadTestIndex(t, root)
}
func runtimeBoundaryPayload(t *testing.T, id string, err error) string {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
		t.Fatalf("missing assertion transport: %v", err)
	}
	_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+id+"|")
	if !ok {
		t.Fatalf("missing observation marker: %v", err)
	}
	return payload
}

var runtimeBoundaryID = regexp.MustCompile(`^[A-Za-z0-9]{15}([A-Za-z0-9]{3})?$`)
var runtimeBoundaryUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Only identities vary between executions. Native presence, type, equality
// flags, stack text, counts and every other cell remain exact comparisons.
func runtimeBoundaryIdentities(t *testing.T, group string, want, got any, path string) {
	t.Helper()
	nativeToLocal, localToNative := map[string]string{}, map[string]string{}
	bindIdentity := func(kind, native, local string) {
		t.Helper()
		nativeKey, localKey := kind+":"+native, kind+":"+local
		if previous, ok := nativeToLocal[nativeKey]; ok && previous != local {
			t.Fatalf("native %s acquired different local identities", kind)
		}
		if previous, ok := localToNative[localKey]; ok && previous != native {
			t.Fatalf("distinct native %s identities aliased locally", kind)
		}
		nativeToLocal[nativeKey], localToNative[localKey] = local, native
	}
	var job string
	if group == "C13" {
		if root, ok := got.(map[string]any); ok {
			job, _ = root["job"].(string)
		}
	}
	var walk func(any, any, string)
	walk = func(want, got any, path string) {
		t.Helper()
		switch expected := want.(type) {
		case map[string]any:
			actual, ok := got.(map[string]any)
			if !ok {
				return
			}
			for key, value := range expected {
				if text, ok := value.(string); ok {
					local, isString := actual[key].(string)
					if text == "[ID]" || group == "C04" && key == "id" && runtimeBoundaryID.MatchString(text) {
						if !isString || !runtimeBoundaryID.MatchString(local) {
							t.Fatalf("invalid identity at %s/%s: %v", path, key, actual[key])
						}
						if group == "C04" {
							bindIdentity("id", text, local)
						}
						if group == "C13" && !storage.IDsEqual(storage.ID(job), storage.ID(local)) {
							t.Fatalf("event job at %s/%s differs from queued job: %q vs %q", path, key, local, job)
						}
						actual[key] = text
						continue
					}
					if group == "C04" && key == "message" && expected["status"] == "OPERATION_ENQUEUED" {
						if !runtimeBoundaryUUID.MatchString(text) || !isString || !runtimeBoundaryUUID.MatchString(local) {
							t.Fatalf("invalid operation UUID at %s: %v", path, actual[key])
						}
						bindIdentity("uuid", text, local)
						actual[key] = text
						continue
					}
				}
				walk(value, actual[key], path+"/"+key)
			}
		case []any:
			actual, ok := got.([]any)
			if !ok || len(actual) != len(expected) {
				return
			}
			for i, value := range expected {
				walk(value, actual[i], fmt.Sprintf("%s/%d", path, i))
			}
		}
	}
	walk(want, got, path)
}

func runtimeBoundaryObservation(t *testing.T, cell runtimeBoundaryCell, err error) string {
	t.Helper()
	if strings.HasPrefix(cell.Status, "uncaught_") {
		var fault *vm.RuntimeError
		if !errors.As(err, &fault) {
			t.Fatalf("missing uncaught fault: %v", err)
		}
		kind := fault.Type
		if !strings.HasPrefix(kind, "System.") {
			kind = "System." + kind
		}
		raw, marshalErr := json.Marshal(map[string]any{"outcome": cell.Status, "exceptionType": kind, "exceptionMessage": kind + ": " + fault.ExceptionMessage()})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return string(raw)
	}
	return runtimeBoundaryPayload(t, cell.ID, err)
}
func runtimeBoundaryCompileRejection(t *testing.T, cell runtimeBoundaryCell, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	var native struct{ Diagnostics json.RawMessage }
	if err := json.Unmarshal(cell.Native, &native); err != nil {
		t.Fatal(err)
	}
	var want struct {
		CompileProblem, Problem string
		Line, Column            int
	}
	if cell.Route == "anonymous" {
		if err := json.Unmarshal(native.Diagnostics, &want); err != nil {
			t.Fatal(err)
		}
	} else {
		var rows []json.RawMessage
		if err := json.Unmarshal(native.Diagnostics, &rows); err != nil || len(rows) != 1 {
			t.Fatalf("native diagnostic %s", native.Diagnostics)
		}
		if err := json.Unmarshal(rows[0], &want); err != nil {
			t.Fatal(err)
		}
	}
	problem := want.Problem
	if problem == "" {
		problem = want.CompileProblem
	}
	for _, got := range diagnostics {
		if got.Severity != diagnostic.Error {
			continue
		}
		message := got.NativeMessage
		if message == "" {
			message = got.Message
		}
		if message != problem || got.Range == nil || got.Range.Start.Line != want.Line {
			t.Fatalf("native compile %s at %d:%d; Glade %+v", problem, want.Line, want.Column, got)
		}
		// Native compiler columns are retained as evidence. The local diagnostic
		// highlights an expression span; that editor range is not Apex frame text.
		if got.Range.Start.Column != want.Column {
			t.Logf("carried compiler coordinate: native %d:%d, local expression span starts %d:%d; exact rejection text and line asserted", want.Line, want.Column, got.Range.Start.Line, got.Range.Start.Column)
		}
		return
	}
	t.Fatal("missing native compilation rejection")
}
