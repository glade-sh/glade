package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type scopeMineContext struct {
	ID           string                     `json:"id"`
	SourceAPI    string                     `json:"sourceApi"`
	CallerAPI    string                     `json:"callerApi"`
	TransportAPI string                     `json:"transportApi"`
	Route        string                     `json:"route"`
	Rows         map[string]json.RawMessage `json:"rows"`
}

// Replay owner-scoped account queries independently from User query scope.
// Captured runAs contexts retain their count, window and restoration controls.
func TestScopeMineConformance(t *testing.T) {
	testScopeMineConformance(t, "scope_mine", "OwnedScopeObservation", "OWNED_SCOPE:")
}

func TestScopeMinePaginationConformance(t *testing.T) {
	testScopeMineConformance(t, "scope_mine_pagination", "OwnedScopePageObservation", "OWNED_SCOPE_PAGE:")
}

func testScopeMineConformance(t *testing.T, fixtureName, helperBase, marker string) {
	t.Helper()
	fixture := filepath.Join("testdata/conformance", fixtureName)
	var data struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
		Contexts []scopeMineContext `json:"contexts"`
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Files) != 2 || len(data.Contexts) != 8 {
		t.Fatal("incomplete Mine scope observations")
	}
	sources := map[string]string{}
	for _, file := range data.Files {
		if (file.Path != "setup/"+helperBase+".cls" && file.Path != "setup/"+helperBase+"Test.cls") || sources[file.Path] != "" {
			t.Fatalf("invalid or repeated Mine scope fixture: %s", file.Path)
		}
		body, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if len(body) == 0 || hex.EncodeToString(digest[:]) != file.SHA256 {
			t.Fatalf("captured Mine scope source changed: %s", file.Path)
		}
		sources[file.Path] = string(body)
	}
	expectedContexts := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			expectedContexts[api+"/"+api+"/"+api+"/"+route] = true
		}
	}
	expectedContexts["67.0/62.0/62.0/anonymous"] = true
	expectedContexts["67.0/67.0/62.0/istest"] = true
	seen, seenIDs := map[string]bool{}, map[string]bool{}
	snapshots := 0
	for _, context := range data.Contexts {
		key := context.SourceAPI + "/" + context.CallerAPI + "/" + context.TransportAPI + "/" + context.Route
		if !expectedContexts[key] || seen[key] || context.ID == "" || seenIDs[context.ID] {
			t.Fatalf("invalid or repeated Mine scope context: %s", key)
		}
		seen[key], seenIDs[context.ID] = true, true
		ids := scopeMineRowIDs(context.Route)
		snapshots += len(ids)
		if len(context.Rows) != len(ids) {
			t.Fatalf("incomplete native Mine scope rows: %s", context.ID)
		}
		for _, id := range ids {
			var native map[string]any
			if err := json.Unmarshal(context.Rows[id], &native); err != nil || len(native) == 0 {
				t.Fatalf("missing native Mine scope: %s/%s", context.ID, id)
			}
		}
	}
	if snapshots != 28 {
		t.Fatalf("Mine scope snapshot count = %d, want 28", snapshots)
	}
	for _, context := range data.Contexts {
		t.Run(context.ID, func(t *testing.T) {
			scopeMineReplay(t, context, sources, helperBase, marker)
		})
	}
}

func scopeMineRowIDs(route string) []string {
	ids := []string{"usersActor"}
	if route == "istest" {
		ids = append(ids, "usersFirst", "accountsActor", "accountsFirst", "accountsSecond", "accountsRestored")
	}
	return ids
}

func scopeMineReplay(t *testing.T, context scopeMineContext, sources map[string]string, helperBase, marker string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.TransportAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	for path, source := range sources {
		name := strings.TrimPrefix(path, "setup/")
		destination := filepath.Join(root, "force-app/main/default/classes", name)
		writeFile(t, destination, source)
		writeFile(t, destination+"-meta.xml", "<ApexClass><apiVersion>"+context.SourceAPI+"</apiVersion></ApexClass>")
	}
	index := loadTestIndex(t, root)
	for _, typ := range index.Types {
		if strings.HasPrefix(typ.File, root) && typ.EffectiveAPIVersion != context.SourceAPI {
			t.Fatalf("changed Mine scope class source pin: %s=%s", typ.Name, typ.EffectiveAPIVersion)
		}
	}
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native Mine scope source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = context.TransportAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	var execErr error
	if context.Route == "anonymous" {
		body := "System.assert(false,'" + marker + "'+JSON.serialize(" + helperBase + ".observe(false)));"
		checked := sema.AnalyzeAnonymous(index, body, context.CallerAPI)
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: context.CallerAPI})
		if checked.HasErrors() || err != nil {
			t.Fatalf("native anonymous Mine scope accepted; local diagnostics=%v compile=%v", checked.Diagnostics, err)
		}
		_, execErr = runner.execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{helperBase + "Test"}, SelectedMethod: "capture"})
		if len(cases) != 1 {
			t.Fatalf("named Mine scope discovery: %d", len(cases))
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named Mine scope compilation: %v %v", methodErrors[key], programErrors[key])
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing Mine scope observation: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("invalid Mine scope marker: %v", execErr)
	}
	var observed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &observed); err != nil || len(observed) != len(context.Rows) {
		t.Fatalf("incomplete observed Mine scope: %s", payload)
	}
	counts := newConformanceCountLog(t, helperBase, context.SourceAPI+" transport "+context.TransportAPI)
	route := context.Route
	if route == "istest" {
		route = "isTest"
	}
	for _, id := range scopeMineRowIDs(context.Route) {
		counts.Run(t, route, id, id, func(t *testing.T) {
			var native, local map[string]any
			if err := json.Unmarshal(context.Rows[id], &native); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(observed[id], &local); err != nil {
				t.Fatalf("missing Mine scope row: %s", id)
			}
			if !reflect.DeepEqual(native, local) {
				t.Fatalf("native Mine scope=%s; local=%s", context.Rows[id], observed[id])
			}
		})
	}
}
