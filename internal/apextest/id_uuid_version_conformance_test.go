package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type idUUIDVersionCase struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Compile       bool   `json:"compile"`
	Expected      string `json:"expected"`
	FloorExpected string `json:"floorExpected"`
	ExpectedNull  bool   `json:"expectedNull,omitempty"`
	Carry         string `json:"carry"`
}

func idUUIDVersionQuote(text string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(text) + "'"
}

func idUUIDVersionAssertion(row idUUIDVersionCase, expected string) string {
	code := row.Code
	if !strings.Contains(code, ";") {
		code = "r = " + code + ";"
	}
	observedType, observedValue := "String", "String.valueOf(r)"
	// Both operands are plain Strings, so String.equals checks exact text.
	assertion := "String expectedText = " + idUUIDVersionQuote(expected) + "; System.assert(expectedText.equals(observedText), " + idUUIDVersionQuote(row.ID) + " + ' expected <' + expectedText + '> actual <' + observedText + '>');\n"
	if row.ExpectedNull {
		// Assert the raw null value; the text 'null' must still fail.
		observedType, observedValue = "Object", "r"
		assertion = "System.assert(observedText == null, " + idUUIDVersionQuote(row.ID) + " + ' expected <null> actual <' + String.valueOf(observedText) + '>');\n"
	}
	return observedType + " observedText; try { Object r; " + code + " observedText = " + observedValue + "; } catch(Exception e) { observedText = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } " + assertion
}

// All expected answers are exported from owned native API62 and API67 captures.
// Carries remain in the dataset with their owner and exact reason. The custom
// object is an oracle fixture, not a product-side prefix special case.
func TestIdUUIDVersionOrgConformance(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/id_uuid_version/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []idUUIDVersionCase
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 229 {
		t.Fatalf("rows=%d, want=229", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "Id, UUID and version", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/FamilyBaseline__c/FamilyBaseline__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>FamilyBaseline</label><pluralLabel>FamilyBaselines</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			boundaryRunner := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{})
			var source strings.Builder
			source.WriteString("@IsTest private class IdUUIDVersionOracleTest {\n")
			count, methods := 0, 0
			expectedErrors := map[string]string{}
			for _, row := range rows {
				if row.Carry != "" {
					t.Logf("%s carried: %s; native=%s", row.ID, row.Carry, row.Expected)
					continue
				}
				expected := row.Expected
				if api == "62.0" {
					expected = row.FloorExpected
				}
				if row.Compile {
					t.Run(row.ID+"/compile", func(t *testing.T) {
						counts.TrackKind(t, "anonymous", "category", 1)
						code := strings.Split(row.Code, "pq.out(")[0]
						if result := sema.AnalyzeAnonymous(typesys.Index{}, code, api); !result.HasErrors() {
							t.Fatalf("native rejected source: %s", code)
						}
						counts.TrackKind(t, "@IsTest", "category", 1)
						invalidRoot := t.TempDir()
						writeFile(t, filepath.Join(invalidRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						path := filepath.Join(invalidRoot, "force-app/main/default/classes/RejectedIdUUIDVersion.cls")
						writeFile(t, path, "@IsTest private class RejectedIdUUIDVersion { @IsTest static void rejected() {"+code+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := loadTestIndex(t, invalidRoot)
						if !idx.HasErrors() && !sema.Analyze(idx).HasErrors() {
							t.Fatalf("native rejected named source: %s", code)
						}
					})
					continue
				}
				if strings.HasPrefix(expected, "UNCAUGHT|") {
					want := strings.TrimPrefix(expected, "UNCAUGHT|")
					expectedErrors[row.ID] = want
					code := "try { Object r; r = " + row.Code + "; } catch(Exception e) { System.assert(false, 'native boundary was incorrectly catchable'); }"
					t.Run(row.ID+"/anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = boundaryRunner.execute(program)
						if err == nil || err.Error() != want {
							t.Fatalf("error=%v, want=%s", err, want)
						}
					})
					// Close a partially filled group before emitting an isolated
					// method whose failure is the expected native boundary.
					if len(expectedErrors) == 1 && count%25 != 0 {
						source.WriteString("}\n")
					}
					fmt.Fprintf(&source, "@IsTest static void %s() { %s }\n", row.ID, code)
					continue
				}
				body := idUUIDVersionAssertion(row, expected)
				t.Run(row.ID+"/anonymous", func(t *testing.T) {
					counts.Track(t, "anonymous", 1)
					if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
						t.Fatalf("analysis: %v", result.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := anonymousRunner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				if count%25 == 0 {
					if count > 0 {
						source.WriteString("}\n")
					}
					fmt.Fprintf(&source, "@IsTest static void batch%d() {\n", methods)
					methods++
				}
				count++
				source.WriteString("{\n" + body + "}\n")
			}
			source.WriteString("}\n")
			if count != 204 {
				t.Fatalf("runtime rows=%d, want=204", count)
			}
			path := filepath.Join(root, "force-app/main/default/classes/IdUUIDVersionOracleTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			t.Run("isTest", func(t *testing.T) {
				counts.Track(t, "@IsTest", count+len(expectedErrors))
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
				if got := run.Summary(); got.Total != methods+len(expectedErrors) || got.Passed != methods {
					t.Fatalf("summary=%#v; want=%d passing batches and %d native boundaries; first problem: %s", got, methods, len(expectedErrors), firstRunProblem(run))
				}
				seen := 0
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if want, ok := expectedErrors[c.MethodName]; ok {
							seen++
							if c.Problem == nil || c.Problem.Message != want {
								t.Fatalf("%s problem=%v, want=%s", c.MethodName, c.Problem, want)
							}
						} else if c.Status != testreport.StatusPass {
							t.Fatalf("%s status=%s problem=%v", c.MethodName, c.Status, c.Problem)
						}
					}
				}
				if seen != len(expectedErrors) {
					t.Fatalf("native boundaries executed=%d want=%d", seen, len(expectedErrors))
				}
			})
		})
	}
}
