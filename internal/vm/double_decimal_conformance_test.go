package vm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The checked-in oracle contains API62 and API67 Salesforce results, never inferred
// expectations or a dependency on glade-tools. Both product execution routes
// consume the same table, including compile rejection and caught exceptions.
func TestDoubleDecimalConformance(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	type probeCase struct {
		ID            string `json:"id"`
		Code          string `json:"code"`
		Compile       bool   `json:"compile"`
		Expected      string `json:"expected"`
		FloorExpected string `json:"floorExpected"`
	}
	var fixture struct {
		APIVersion   string      `json:"api_version"`
		Cases        []probeCase `json:"cases"`
		Preservation []probeCase `json:"preservation"`
		FloorOracle  struct {
			APIVersion     string `json:"apiVersion"`
			Rows           int    `json:"rows"`
			MissingResults int    `json:"missingResults"`
		} `json:"floorOracle"`
	}
	data, err := os.ReadFile("testdata/conformance/double_decimal.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 300 || fixture.APIVersion != "67.0" {
		t.Fatal("incomplete or wrong-version oracle")
	}
	if fixture.FloorOracle.APIVersion != "62.0" || fixture.FloorOracle.Rows != len(fixture.Cases) || fixture.FloorOracle.MissingResults != 0 {
		t.Fatal("incomplete or wrong-version floor oracle")
	}
	nativeCompilerRows := map[string]string{}
	for _, row := range fixture.Cases {
		if strings.HasPrefix(row.FloorExpected, "COMPILE_ERROR") {
			nativeCompilerRows[row.ID] = row.FloorExpected
		}
	}
	floorMismatches := newFloorCompilerMismatches(t, "Double and Decimal", nativeCompilerRows, "anonymous", "@IsTest")
	for _, api := range []string{"62.0", fixture.APIVersion} {
		t.Run(api, func(t *testing.T) {
			rows := fixture.Cases
			if api == fixture.APIVersion {
				rows = append(append([]probeCase{}, rows...), fixture.Preservation...)
			}
			anonymousExact, namedExact, anonymousCategory, namedCategory := 0, 0, 0, 0
			anonymousLegacy, namedLegacy, anonymousMismatch, namedMismatch := 0, 0, 0, 0
			anonymousExactRows, namedExactRows := 0, 0
			anonymousFraming, namedFraming := 0, 0
			if api == "62.0" {
				anonymousExactRows, namedExactRows = len(rows), len(rows)
				for id := range nativeCompilerRows {
					switch floorMismatches.Kind(id, "anonymous") {
					case "mismatch":
						anonymousExactRows--
						anonymousMismatch++
					case "line-framing":
						anonymousExactRows--
						anonymousFraming++
					}
					switch floorMismatches.Kind(id, "@IsTest") {
					case "mismatch":
						namedExactRows--
						namedMismatch++
					case "line-framing":
						namedExactRows--
						namedFraming++
					}
				}
			}
			defer func() {
				t.Logf("Double and Decimal API %s anonymous exact %d/%d; category-only %d; carried 0; partial 0; legacy %d; line-framing %d; mismatch %d", api, anonymousExact, anonymousExactRows, anonymousCategory, anonymousLegacy, anonymousFraming, anonymousMismatch)
				t.Logf("Double and Decimal API %s @IsTest exact %d/%d; category-only %d; carried 0; partial 0; legacy %d; line-framing %d; mismatch %d", api, namedExact, namedExactRows, namedCategory, namedLegacy, namedFraming, namedMismatch)
			}()
			var namedMethods strings.Builder
			var namedIDs []string
			for _, row := range rows {
				expected := row.Expected
				if api == "62.0" {
					expected = row.FloorExpected
				}
				t.Run(row.ID, func(t *testing.T) {
					code := row.Code
					if row.Compile {
						code = strings.ReplaceAll(code, fmt.Sprintf("pq.out('%s', r)", row.ID), "actual = '' + String.valueOf(r)")
					} else {
						if !strings.Contains(code, ";") {
							code = "r = " + code + ";"
						}
						code = "Object r; " + code + " actual = '' + String.valueOf(r);"
					}
					body := "String actual; try { " + code + " } catch (Exception e) { actual = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); }"
					reject := strings.HasPrefix(expected, "COMPILE_ERROR")
					if api != "62.0" {
						// These buckets describe the asserted category, not an
						// exact-text match, even when an existing check fails.
						if reject {
							anonymousCategory++
							namedCategory++
						} else {
							anonymousLegacy++
							namedLegacy++
						}
					}
					if !reject {
						if api == "62.0" {
							body += " System.assert('" + apexQuote(expected) + "'.equals(actual));"
						} else {
							body += " System.assertEquals('" + apexQuote(row.Expected) + "', actual);"
						}
						namedMethods.WriteString("@IsTest static void probe" + row.ID + "() { " + body + " }\n")
						namedIDs = append(namedIDs, "probe"+row.ID)
					}
					if t.Run("anonymous", func(t *testing.T) {
						analysis := sema.AnalyzeAnonymous(typesys.Index{}, body, api)
						if reject {
							if !analysis.HasErrors() {
								t.Fatal("Salesforce compile rejection accepted")
							}
							if api == "62.0" {
								observed := doubleDecimalCompilerText(analysis.Diagnostics)
								floorMismatches.Check(t, row.ID, "anonymous", expected, observed)
							}
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("sema: %+v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := vm.Execute(program, nil); err != nil {
							t.Fatal(err)
						}
					}) {
						if api == "62.0" {
							if !reject || floorMismatches.Kind(row.ID, "anonymous") == "exact" {
								anonymousExact++
							}
						}
					}
					if reject {
						if t.Run("isTestCompile", func(t *testing.T) {
							index := numericFamilyIndexAPI(t, api, "@IsTest private class NumericFamilyProbe { @IsTest static void probe() { "+body+" } }")
							if !sema.Analyze(index).HasErrors() && !(diagnostic.Report{Diagnostics: index.Diagnostics}).HasErrors() {
								t.Fatal("Salesforce compile rejection accepted in named class")
							}
							if api == "62.0" {
								analysis := sema.Analyze(index)
								observed := doubleDecimalCompilerText(append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...))
								floorMismatches.Check(t, row.ID, "@IsTest", expected, observed)
							}
						}) {
							if api == "62.0" {
								if floorMismatches.Kind(row.ID, "@IsTest") == "exact" {
									namedExact++
								}
							}
						}
					}
				})
			}
			t.Run("isTest", func(t *testing.T) {
				index := numericFamilyIndexAPI(t, api, "@IsTest private class NumericFamilyProbe { "+namedMethods.String()+" }")
				if analysis := sema.Analyze(index); analysis.HasErrors() {
					t.Fatalf("sema: %+v", analysis.Diagnostics)
				}
				run := apextest.Run(index, apextest.Options{NoDiskCache: true})
				if summary := run.Summary(); summary.Total != len(namedIDs) {
					t.Fatalf("named route executed %d cases, want %d", summary.Total, len(namedIDs))
				}
				results := make(map[string]bool)
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						results[result.MethodName] = result.Status == "pass"
						if result.Status != "pass" {
							t.Errorf("%s: %s %+v", result.MethodName, result.Reason, result.Problem)
						}
					}
				}
				for _, id := range namedIDs {
					if results[id] {
						if api == "62.0" {
							namedExact++
						}
					}
					if !results[id] {
						t.Errorf("named case %s did not pass", id)
					}
				}
			})
		})
	}
}

// Preserve the captured compiler text, including its source line. This formatter
// uses diagnostic metadata directly; it does not normalize the row adapter.
func doubleDecimalCompilerText(diagnostics []diagnostic.Diagnostic) string {
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		message := d.Message
		if d.NativeMessage != "" {
			message = d.NativeMessage
		}
		line := 0
		if d.Range != nil {
			line = d.Range.Start.Line
		}
		if d.NativeLine != nil {
			line = *d.NativeLine
		}
		return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
	}
	return "compiled"
}

func numericFamilyIndex(t *testing.T, source string) typesys.Index {
	t.Helper()
	return numericFamilyIndexAPI(t, "67.0", source)
}

func numericFamilyIndexAPI(t *testing.T, api, source string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	write(filepath.Join(classes, "NumericFamilyProbe.cls"), source)
	write(filepath.Join(classes, "NumericFamilyProbe.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

func apexQuote(text string) string {
	return strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(text)
}
