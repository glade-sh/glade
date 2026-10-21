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

type privateNestedReflectionContext struct {
	ID            string                     `json:"id"`
	SourceAPI     string                     `json:"sourceApi"`
	DependencyAPI string                     `json:"dependencyApi"`
	CallerAPI     string                     `json:"callerApi"`
	TransportAPI  string                     `json:"transportApi"`
	Route         string                     `json:"route"`
	Rows          map[string]json.RawMessage `json:"rows"`
}

// The lookup overloads, raw cast text and parsed lookup are independent native
// observations. Owner-local literals and reflection retain their own answers.
func TestPrivateNestedReflectionConformance(t *testing.T) {
	const fixture = "testdata/conformance/private_nested_reflection"
	var data struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
		Contexts      []privateNestedReflectionContext `json:"contexts"`
		OwnerContexts []privateNestedReflectionContext `json:"ownerContexts"`
		AliasContexts []privateNestedReflectionContext `json:"aliasContexts"`
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Files) != 6 || len(data.Contexts) != 8 || len(data.OwnerContexts) != 3 || len(data.AliasContexts) != 3 {
		t.Fatal("incomplete private nested reflection matrix")
	}
	sources := map[string]string{}
	expectedFiles := map[string]bool{
		"setup/ReflectionProbeBase.cls":       true,
		"setup/ReflectionSubjects.cls":        true,
		"setup/ReflectionObservation.cls":     true,
		"setup/ReflectionObservationTest.cls": true,
		"setup/ReflectionLookupOwner.cls":     true,
		"setup/ReflectionLookupAliases.cls":   true,
	}
	for _, file := range data.Files {
		if !expectedFiles[file.Path] || sources[file.Path] != "" {
			t.Fatalf("invalid or repeated reflection fixture: %s", file.Path)
		}
		body, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if len(body) == 0 || hex.EncodeToString(digest[:]) != file.SHA256 {
			t.Fatalf("captured reflection source changed: %s", file.Path)
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
	cells := 0
	for _, context := range data.Contexts {
		key := context.SourceAPI + "/" + context.CallerAPI + "/" + context.TransportAPI + "/" + context.Route
		if !expectedContexts[key] || seen[key] || context.ID == "" || seenIDs[context.ID] || context.DependencyAPI != "" {
			t.Fatalf("invalid or repeated reflection context: %s", key)
		}
		seen[key], seenIDs[context.ID] = true, true
		ids := []string{"privateRegular", "publicRegular", "publicTop"}
		if context.Route == "istest" {
			ids = append(ids, "privateTest")
		}
		privateNestedReflectionValidateRows(t, context, ids)
		cells += len(ids)
	}
	if cells != 28 {
		t.Fatalf("reflection observation count = %d, want 28", cells)
	}
	seenOwners := map[string]bool{}
	for _, context := range data.OwnerContexts {
		if context.ID == "" || seenIDs[context.ID] || seenOwners[context.SourceAPI] || (context.SourceAPI != "62.0" && context.SourceAPI != "67.0" && context.SourceAPI != "68.0") || context.CallerAPI != context.SourceAPI || context.TransportAPI != context.SourceAPI || context.Route != "anonymous" || context.DependencyAPI != "" {
			t.Fatal("invalid or repeated owner-local reflection context")
		}
		seenOwners[context.SourceAPI], seenIDs[context.ID] = true, true
		privateNestedReflectionValidateRows(t, context, []string{"insideOwner"})
	}
	seenAliases := map[string]bool{}
	for _, context := range data.AliasContexts {
		if context.ID == "" || seenIDs[context.ID] || seenAliases[context.SourceAPI] || (context.SourceAPI != "62.0" && context.SourceAPI != "67.0" && context.SourceAPI != "68.0") || context.CallerAPI != context.SourceAPI || context.TransportAPI != context.SourceAPI || context.Route != "anonymous" || context.DependencyAPI != "68.0" {
			t.Fatal("invalid or repeated namespace-argument reflection context")
		}
		seenAliases[context.SourceAPI], seenIDs[context.ID] = true, true
		privateNestedReflectionValidateRows(t, context, []string{"PrivateChild", "PublicChild"})
	}
	for _, context := range data.Contexts {
		t.Run(context.ID, func(t *testing.T) {
			privateNestedReflectionReplay(t, context, sources, "external")
		})
	}
	for _, context := range data.OwnerContexts {
		t.Run(context.ID, func(t *testing.T) {
			privateNestedReflectionReplay(t, context, sources, "owner")
		})
	}
	for _, context := range data.AliasContexts {
		t.Run(context.ID, func(t *testing.T) {
			privateNestedReflectionReplay(t, context, sources, "alias")
		})
	}
}

func privateNestedReflectionValidateRows(t *testing.T, context privateNestedReflectionContext, ids []string) {
	t.Helper()
	if len(context.Rows) != len(ids) {
		t.Fatalf("incomplete reflection rows: %s", context.ID)
	}
	for _, id := range ids {
		var native map[string]any
		if err := json.Unmarshal(context.Rows[id], &native); err != nil || len(native) == 0 {
			t.Fatalf("missing captured reflection value: %s/%s", context.ID, id)
		}
	}
}

func privateNestedReflectionReplay(t *testing.T, context privateNestedReflectionContext, sources map[string]string, kind string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.TransportAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	pins := map[string]string{}
	switch kind {
	case "external":
		for _, name := range []string{"ReflectionProbeBase.cls", "ReflectionSubjects.cls", "ReflectionObservation.cls", "ReflectionObservationTest.cls"} {
			pins[name] = context.SourceAPI
		}
	case "owner":
		pins["ReflectionLookupOwner.cls"] = context.SourceAPI
	case "alias":
		pins["ReflectionLookupAliases.cls"] = context.SourceAPI
		pins["ReflectionProbeBase.cls"] = context.DependencyAPI
		pins["ReflectionSubjects.cls"] = context.DependencyAPI
	default:
		t.Fatal("unknown reflection observation shape")
	}
	for path, source := range sources {
		name := strings.TrimPrefix(path, "setup/")
		api, required := pins[name]
		if !required {
			continue
		}
		destination := filepath.Join(root, "force-app/main/default/classes", name)
		writeFile(t, destination, source)
		writeFile(t, destination+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	index := loadTestIndex(t, root)
	for _, typ := range index.Types {
		if strings.HasPrefix(typ.File, root) && typ.EffectiveAPIVersion != pins[filepath.Base(typ.File)] {
			t.Fatalf("changed reflection class source pin: %s=%s", typ.Name, typ.EffectiveAPIVersion)
		}
	}
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native reflection declarations accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = context.TransportAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	const marker = "NATIVE_REFLECTION:"
	var execErr error
	if context.Route == "anonymous" {
		expression := "ReflectionObservation.observe()"
		if kind == "owner" {
			expression = "new Map<String,Object>{'insideOwner'=>ReflectionLookupOwner.observe()}"
		} else if kind == "alias" {
			expression = "ReflectionLookupAliases.observe()"
		}
		body := "System.assert(false,'" + marker + "'+JSON.serialize(" + expression + "));"
		checked := sema.AnalyzeAnonymous(index, body, context.CallerAPI)
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: context.CallerAPI})
		if checked.HasErrors() || err != nil {
			t.Fatalf("native reflection caller accepted; local diagnostics=%v compile=%v", checked.Diagnostics, err)
		}
		_, execErr = runner.execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{"ReflectionObservationTest"}, SelectedMethod: "capture"})
		if len(cases) != 1 {
			t.Fatalf("named reflection discovery: %d", len(cases))
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named reflection compilation: %v %v", methodErrors[key], programErrors[key])
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
		t.Fatalf("missing reflection observation: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("invalid reflection observation marker: %v", execErr)
	}
	var observed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &observed); err != nil || len(observed) != len(context.Rows) {
		t.Fatalf("incomplete reflection payload: %s", payload)
	}
	counts := newConformanceCountLog(t, "PrivateNestedReflection", context.SourceAPI+" transport "+context.TransportAPI)
	route := context.Route
	if route == "istest" {
		route = "isTest"
	}
	for id, native := range context.Rows {
		counts.Run(t, route, id, id, func(t *testing.T) {
			var expected, actual map[string]any
			if err := json.Unmarshal(native, &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(observed[id], &actual); err != nil {
				t.Fatalf("missing reflection row %s", id)
			}
			if !reflect.DeepEqual(expected, actual) {
				t.Fatalf("native reflection=%s; local=%s", native, observed[id])
			}
		})
	}
}
