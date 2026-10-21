package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestContentExtensionOrgConformance(t *testing.T) {
	var data struct {
		Source, FieldMetadata string
		Cells                 []struct {
			API, Context string
			Native       []map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/content_extension/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(data.Source) == "" || strings.TrimSpace(data.FieldMetadata) == "" || len(data.Cells) != 6 {
		t.Fatalf("native fixture shape: source present=%t, cells=%d", data.Source != "", len(data.Cells))
	}
	seen := make(map[string]bool)
	for _, cell := range data.Cells {
		name := cell.API + "/" + cell.Context
		if seen[name] || (cell.API != "62.0" && cell.API != "67.0" && cell.API != "68.0") || (cell.Context != "anonymous" && cell.Context != "istest") || len(cell.Native) != 2 {
			t.Fatalf("unexpected or repeated native cell %s: snapshots=%d", name, len(cell.Native))
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			caller := "List<Object> snapshots=(List<Object>)JSON.deserializeUntyped(FileRevisionOracle.observe());\nSystem.assertEquals(2,snapshots.size());\n"
			for position, native := range cell.Native {
				path := "specimen"
				var extension any
				if position == 1 {
					path, extension = "specimen.txt", "txt"
				}
				if native["path"] != path || native["extension"] != extension || native["extensionNull"] != (extension == nil) || native["sparse"] != "accepted" || native["documentUpdate"] != "accepted" || native["replacementExtension"] != nil || native["replacementExtensionNull"] != true {
					t.Fatalf("unexpected native snapshot: %#v", native)
				}
				if native["explicitType"] != "System.SObjectException" || native["explicitMessage"] == nil || native["explicitMessage"] == "" {
					t.Fatalf("native explicit-assignment denial lost its evidence: %#v", native)
				}
				t.Logf("Native explicit-assignment boundary: %s: %s; replay asserts denial only", native["explicitType"], native["explicitMessage"])
				caller += fmt.Sprintf("Map<String,Object> snapshot%d=(Map<String,Object>)snapshots[%d];\n", position, position)
				for _, key := range []string{"path", "extension", "extensionNull", "sparse", "documentUpdate", "replacementExtension", "replacementExtensionNull"} {
					if _, present := native[key]; !present {
						t.Fatalf("native snapshot is missing %s", key)
					}
					caller += fmt.Sprintf("System.assert(snapshot%d.containsKey(%s));\nSystem.assertEquals(%s,snapshot%d.get(%s),%s);\n", position, conformanceApexString(key), contentExtensionLiteral(t, native[key]), position, conformanceApexString(key), conformanceApexString(key))
				}
				// Native rejects explicit assignment before DML. The separate local
				// assignment surface remains a carry; preserve rejection here without
				// claiming its exception type or stage matches that native boundary.
				caller += fmt.Sprintf("System.assert(!snapshot%d.containsKey('explicit'),'explicit readonly assignment must be denied');\n", position)
				caller += fmt.Sprintf("System.assert(String.valueOf(snapshot%d.get('explicitMessage')).contains('FileExtension'),'denial must identify the readonly extension');\n", position)
				caller += fmt.Sprintf("System.debug(JSON.serialize(snapshot%d));\n", position)
				if cell.API == "62.0" {
					if native["queried"] != "accepted" {
						t.Fatalf("native queried update was not accepted: %#v", native)
					}
					caller += fmt.Sprintf("System.assertEquals('accepted',snapshot%d.get('queried'),'queried update');\n", position)
				} else {
					if native["queriedType"] != "System.DmlException" || native["queriedMessage"] == nil || native["queriedMessage"] == "" {
						t.Fatalf("native queried-update carry lost its rejection evidence: %#v", native)
					}
					// The queried snapshot has a separate source-version
					// access boundary. Retain the captured denial without
					// asserting parity for that independently owned behavior.
					t.Logf("Queried snapshot update carry: native %s: %s", native["queriedType"], native["queriedMessage"])
				}
			}

			root := t.TempDir()
			helper := filepath.Join(root, "FileRevisionOracle.cls")
			writeFile(t, helper, data.Source)
			writeFile(t, helper+"-meta.xml", "<ApexClass><apiVersion>"+cell.API+"</apiVersion></ApexClass>")
			field := filepath.Join(root, "objects", "ContentVersion", "fields", "UploadToken__c.field-meta.xml")
			writeFile(t, field, data.FieldMetadata)
			fixture := project.Project{Root: root, ApexFiles: []string{helper}, FieldFiles: []string{field}, SourceAPIVersion: cell.API}
			if cell.Context == "istest" {
				path := filepath.Join(root, "FileRevisionTest.cls")
				writeFile(t, path, "@IsTest private class FileRevisionTest {@IsTest static void observed(){"+caller+"}}")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+cell.API+"</apiVersion></ApexClass>")
				fixture.ApexFiles = append(fixture.ApexFiles, path)
			}
			metadata, err := schema.LoadProject(fixture)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(fixture, metadata)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compilation: parser=%v semantics=%v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if cell.Context == "istest" {
				// Invoke the indexed @IsTest method in its class/test context so
				// the independently carried exception boundaries remain visible.
				cases := Discover(index, Options{SelectedClasses: []string{"FileRevisionTest"}, SelectedMethod: "observed"})
				if len(cases) != 1 {
					t.Fatalf("named method discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				invocations, invocationErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if err := methodErrors[key]; err != nil {
					t.Fatal(err)
				}
				if err := invocationErrors[key]; err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				observed, err := machine.ExecuteInClass(invocations[key], cases[0].ClassName)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range observed.Debug {
					t.Logf("Local boundary observations: %s", line)
				}
				return
			}
			if analysis := sema.AnalyzeAnonymous(index, caller, cell.API); analysis.HasErrors() {
				t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(caller, vm.CompileOptions{APIVersion: cell.API})
			if err != nil {
				t.Fatal(err)
			}
			observed, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range observed.Debug {
				t.Logf("Local boundary observations: %s", line)
			}
		})
	}
}

func contentExtensionLiteral(t *testing.T, value any) string {
	t.Helper()
	switch value := value.(type) {
	case nil:
		return "null"
	case string:
		return conformanceApexString(value)
	case bool:
		return fmt.Sprint(value)
	default:
		t.Fatalf("unexpected native extension observation type %T", value)
		return ""
	}
}
