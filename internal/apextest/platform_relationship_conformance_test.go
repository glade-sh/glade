package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type platformRelationshipRow struct{ api, route, kind, parent, relationship, child, field, query, outcome, message, values string }

func platformRelationshipRows(t *testing.T) []platformRelationshipRow {
	t.Helper()
	f, err := os.Open("testdata/conformance/platform_relationships/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 1403 {
		t.Fatal("incomplete native relationship sweep")
	}
	var rows []platformRelationshipRow
	seen := make(map[string]bool)
	cells := make(map[string]int)
	for _, r := range table[1:] {
		if len(r) != 11 {
			t.Fatal("incomplete native row")
		}
		key := strings.Join(r[:5], "|")
		if seen[key] {
			t.Fatal("duplicate native observation")
		}
		seen[key] = true
		cells[r[0]+"/"+r[1]]++
		rows = append(rows, platformRelationshipRow{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8], r[9], r[10]})
	}
	for cell, count := range map[string]int{"62.0/anonymous": 346, "62.0/istest": 346, "67.0/anonymous": 355, "67.0/istest": 355} {
		if cells[cell] != count {
			t.Fatalf("native cell %s: %d rows; want %d", cell, cells[cell], count)
		}
	}
	return rows
}

// Both routes independently observed every outcome at the supported source APIs.
// Feature-dependent describe entries remain observations, not portability gates.
func TestPlatformRelationshipsNativeConformance(t *testing.T) {
	rows := platformRelationshipRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "PlatformRelationships", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			fixtureRoot := "testdata/conformance/platform_relationships"
			for _, file := range []struct{ src, dst string }{
				{"LinkProbe__c.object-meta.xml", "objects/LinkProbe__c/LinkProbe__c.object-meta.xml"},
				{"Account__c.field-meta.xml", "objects/LinkProbe__c/fields/Account__c.field-meta.xml"},
			} {
				raw, err := os.ReadFile(filepath.Join(fixtureRoot, file.src))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", file.dst), string(raw))
			}
			staticRaw, err := os.ReadFile(filepath.Join(fixtureRoot, "PlatformRelationshipNativeTest.cls"))
			if err != nil {
				t.Fatal(err)
			}
			staticSource := string(staticRaw)
			start := strings.Index(staticSource, "    @IsTest static void staticSlice()")
			if start < 0 {
				t.Fatal("missing native static slice")
			}
			staticSource = "@IsTest public class PlatformRelationshipNativeTest {\n" + staticSource[start:]
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PlatformRelationshipNativeTest.cls"), staticSource)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PlatformRelationshipNativeTest.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			for _, name := range []string{"Account", "User", "Case", "Contact", "Opportunity"} {
				storage.EnsureStandardObject(&org, name)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			setup := `Account a=new Account(Name='Relationship probe');insert a;Contact c=new Contact(LastName='Probe',AccountId=a.Id);insert c;LinkProbe__c link=new LinkProbe__c(Name='Probe',Account__c=a.Id);insert link;Case issue=new Case(Subject='Probe',Status='New',Origin='Web');insert issue;`
			t.Run("nativeStaticSlice", func(t *testing.T) {
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native static slice accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: []string{"PlatformRelationshipNativeTest"}, SelectedMethod: "staticSlice"})
				if len(cases) != 1 {
					t.Fatalf("native static slice discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("static slice compile: %v %v", methodErrors[key], programErrors[key])
				}
				machine := base.newMachine()
				machine.EnableTestContext()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
					t.Fatal(err)
				}

			})
			for _, parent := range []string{"Account", "User", "Case", "Contact", "Opportunity", "LinkProbe__c"} {
				t.Run("staticCatalog/"+parent, func(t *testing.T) {
					var source strings.Builder
					for i, row := range rows {
						if row.api == api && row.route == "anonymous" && row.parent == parent && row.outcome == "ok" {
							fmt.Fprintf(&source, "List<%s> observed%d=[%s];\n", parent, i, row.query)
						}
					}
					analysis := sema.AnalyzeAnonymous(index, source.String(), api)
					for _, d := range analysis.Diagnostics {
						if d.Severity == diagnostic.Error {
							t.Errorf("native accepted static query; Glade: %s", d.Message)
						}
					}
				})
			}
			for _, row := range rows {
				if row.api != api {
					continue
				}
				row := row
				t.Run(row.route+"/"+row.parent+"/"+row.kind+"/"+row.relationship, func(t *testing.T) {
					kind := "exact"
					if row.outcome == "reject" {
						kind = "category"
					}
					if row.kind == "child" && row.outcome == "reject" {
						kind = "carried"
					}
					counts.TrackKind(t, row.route, kind, 1)
					if row.kind == "child" && row.outcome == "reject" {
						t.Logf("feature-dependent native rejection: %s", row.message)
						return
					}
					source := setup + "List<SObject> observed;"
					if row.outcome == "reject" {
						source += "Boolean caught=false;try{observed=Database.query('" + strings.ReplaceAll(row.query, "'", "\\'") + "');}catch(QueryException e){caught=true;}System.assertEquals(true,caught);"
					} else {
						source += "observed=Database.query('" + strings.ReplaceAll(row.query, "'", "\\'") + "');"
						// Anonymous access captures include the actual flags and null-parent value.
						if row.kind == "access" && row.values != "" {
							var values []map[string]any
							if err := json.Unmarshal([]byte(row.values), &values); err != nil {
								t.Fatal(err)
							}
							if len(values) > 0 {
								source += platformAccessAssertions(values[0], "observed[0]")
							}
						}
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := base.newMachine()
					machine.EnableTestContext()
					if row.route == "istest" {
						if err := machine.RegisterClass(vm.Class{Name: "PlatformRelationshipSlice", APIVersion: api, Modifiers: []string{"isTest"}}); err != nil {
							t.Fatal(err)
						}
						_, err = machine.ExecuteInClass(program, "PlatformRelationshipSlice")
					} else {
						_, err = machine.Execute(program)
					}
					if err != nil {
						t.Fatalf("native %s %s; Glade: %v", row.outcome, row.query, err)
					}
				})
			}
		})
	}
}

func platformAccessAssertions(value map[string]any, receiver string) string {
	var source string
	for key, value := range value {
		if strings.EqualFold(key, "Id") || key == "attributes" {
			continue
		}
		field := receiver + ".get('" + key + "')"
		switch value := value.(type) {
		case bool:
			source += fmt.Sprintf("System.assertEquals(%t,%s);", value, field)
		case string:
			source += "System.assertEquals('" + value + "'," + field + ");"
		case nil:
			source += "System.assertEquals(null," + field + ");"
		case map[string]any:
			if records, ok := value["records"].([]any); ok {
				for i, record := range records {
					if row, ok := record.(map[string]any); ok {
						source += platformAccessAssertions(row, fmt.Sprintf("%s.getSObjects('%s')[%d]", receiver, key, i))
					}
				}
			} else {
				source += platformAccessAssertions(value, receiver+".getSObject('"+key+"')")
			}
		}
	}
	return source
}
