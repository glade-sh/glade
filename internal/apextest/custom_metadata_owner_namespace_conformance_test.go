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
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const metadataOwnerFixture = "testdata/conformance/custom_metadata_owner_namespace"

// Replay the captured named-test route with a source dependency standing in for
// the installed managed type. The subscriber owns one record of that type.
func TestCustomMetadataOwnerNamespaceOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string
		NativeObservationCount int
		Files                  []struct{ Path, SHA256 string }
	}
	raw, err := os.ReadFile(filepath.Join(metadataOwnerFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.NativeObservationCount != 42 {
		t.Fatal("incomplete native matrix")
	}
	files := make(map[string]string)
	for _, file := range data.Files {
		if filepath.IsAbs(file.Path) || strings.Contains(file.Path, "..") {
			t.Fatal("invalid fixture path")
		}
		if _, exists := files[file.Path]; exists {
			t.Fatal("duplicate fixture path")
		}
		raw, err := os.ReadFile(filepath.Join(metadataOwnerFixture, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("native fixture changed: %s", file.Path)
		}
		files[file.Path] = string(raw)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			for name, text := range files {
				if strings.HasPrefix(name, "fixture/") {
					writeFile(t, filepath.Join(root, strings.TrimPrefix(name, "fixture/")), text)
				}
			}
			subscriber := filepath.Join(root, "subscriber")
			writeFile(t, filepath.Join(subscriber, "sfdx-project.json"), fmt.Sprintf(`{"namespace":"gladens002","sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, subscriber)
			org := orgFromIndex(index)
			org.APIVersion = api
			if org.Namespace != "gladens002" || len(org.Objects["gladens001__OwnerFlag__mdt"].Records) != 2 {
				t.Fatal("synthetic owner/provider records failed to load")
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			dir := "native/api" + strings.TrimSuffix(api, ".0") + "/"
			run := struct {
				API, Route string
				Rows       int
			}{}
			if err := json.Unmarshal([]byte(files[dir+"RUN-FACTS.txt"]), &run); err != nil {
				t.Fatal(err)
			}
			if run.API != api || run.Route != "istest" || run.Rows != 14 {
				t.Fatal("incomplete native run")
			}
			lines := strings.Split(strings.TrimSpace(files[dir+"org.tsv"]), "\n")
			if len(lines) != 15 || lines[0] != "id\tstatus\tvalue" {
				t.Fatal("incomplete native rows")
			}
			seen := map[string]bool{}
			counts := newConformanceCountLog(t, "CustomMetadataOwnerNamespace", api)
			for _, line := range lines[1:] {
				columns := strings.SplitN(line, "\t", 3)
				if len(columns) != 3 || seen[columns[0]] {
					t.Fatal("invalid native row")
				}
				id, status, expected := columns[0], columns[1], columns[2]
				seen[id] = true
				t.Run(id, func(t *testing.T) {
					counts.TrackKind(t, "isTest", "exact", 1)
					source, ok := files[dir+id+".cls"]
					if !ok {
						t.Fatal("missing captured source")
					}
					className := "MetadataOwner" + strings.TrimSuffix(api, ".0") + strings.ReplaceAll(id, "_", "")
					classRoot := subscriber
					if status == "COMPILE_ERROR" {
						// The language-only control has no metadata references. Load
						// it independently so source-dependency warning policy does
						// not weaken the native compiler rejection assertion.
						classRoot = t.TempDir()
						writeFile(t, filepath.Join(classRoot, "sfdx-project.json"), files["fixture/subscriber/sfdx-project.json"])
					}
					path := filepath.Join(classRoot, "force-app/main/default/classes", className+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					t.Cleanup(func() { os.Remove(path); os.Remove(path + "-meta.xml") })
					cellIndex := loadTestIndex(t, classRoot)
					metadataOwnerNamed(t, cellIndex, base, className, id, status, expected)
				})
			}
		})
	}
}

func metadataOwnerNamed(t *testing.T, index typesys.Index, base *conformanceRunner, className, id, status, expected string) {
	t.Helper()
	diagnostics := sema.NativeLifecycleDiagnostics(index, append(append(index.Diagnostics[:0:0], index.Diagnostics...), sema.Analyze(index).Diagnostics...))
	if status == "COMPILE_ERROR" {
		var native []struct{ Problem string }
		if err := json.Unmarshal([]byte(expected), &native); err != nil || len(native) != 1 {
			t.Fatal("missing native compile diagnostic")
		}
		metadataNamespaceCompile(t, metadataNamespaceCell{Outcome: "compile-rejected", PrimaryDiagnostic: native[0].Problem}, diagnostics, nil)
		return
	}
	if metadataNamespaceErrors(diagnostics) {
		t.Fatalf("native compilation accepted; %v", diagnostics)
	}
	cases := Discover(index, Options{SelectedClasses: []string{className}, SelectedMethod: "observed"})
	if len(cases) != 1 {
		t.Fatal("one captured method required")
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	org := base.newOrg()
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
	machine := runner.newMachine()
	machine.EnableTestContext()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing observation transport: %v", execErr)
	}
	if status == "UNCAUGHT" {
		var message string
		if err := json.Unmarshal([]byte(expected), &message); err != nil {
			t.Fatal(err)
		}
		exceptionType := runtimeErr.Type
		if !strings.Contains(exceptionType, ".") {
			exceptionType = "System." + exceptionType
		}
		if got := exceptionType + ": " + runtimeErr.ExceptionMessage(); got != message {
			t.Fatalf("native <%s>; Glade <%s>", message, got)
		}
		return
	}
	if status != "OBSERVED" || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("wrong observation transport: %s %v", status, execErr)
	}
	_, observed, ok := strings.Cut(runtimeErr.ExceptionMessage(), "METADATA_OWNER|"+id+"|")
	if !ok {
		t.Fatal("missing captured row marker")
	}
	var want, got any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(observed), &got); err != nil {
		t.Fatal(err)
	}
	// SOQL JSON includes server-assigned IDs and REST transport attributes. Compare
	// every explicitly projected identity field, retaining raw evidence on disk.
	if id == "R008" {
		for _, value := range []any{want, got} {
			if rows, ok := value.([]any); ok {
				for _, row := range rows {
					if fields, ok := row.(map[string]any); ok {
						delete(fields, "attributes")
						delete(fields, "Id")
					}
				}
			}
		}
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("native <%s>; Glade <%s>", expected, observed)
	}
}
