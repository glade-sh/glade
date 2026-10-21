package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

type locationCase struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Compile       bool   `json:"compile"`
	Expected      string `json:"expected"`
	FloorExpected string `json:"floorExpected"`
	ExpectedNull  bool   `json:"expectedNull,omitempty"`
}

func locationQuote(text string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(text) + "'"
}

func locationCode(row locationCase) string {
	code := row.Code
	if row.Compile {
		code = strings.Split(code, "pq.out(")[0]
		if !strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
			code += "r = 'compiled';"
		}
	}
	if !strings.Contains(code, ";") {
		code = "r = " + code + ";"
	}
	return "Object r; " + code
}

func locationAssertion(row locationCase, expected string) string {
	if row.ExpectedNull {
		// Assert the raw null value; the text 'null' must still fail.
		return "Object observed; try { " + locationCode(row) + " observed = r; } catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } System.assert(observed == null, " + locationQuote(row.ID+": exact null") + ");\n"
	}
	quoted := locationQuote(expected)
	return "String observed; try { " + locationCode(row) + " observed = String.valueOf(r); } catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } String expectedText = " + quoted + "; System.assert(expectedText.equals(observed), " + locationQuote(row.ID) + " + ' expected <' + expectedText + '> actual <' + observed + '>');\n"
}

// locationAPI68Uncaptured lists rows with no native API 68 answer. API 68
// asserts only rows whose captured answer equals the API 67 oracle.
func locationAPI68Uncaptured() map[string]string {
	out := map[string]string{}
	for i := 1; i <= 19; i++ {
		out[fmt.Sprintf("C%03d", i)] = "System.Location control has no API 68 capture"
	}
	for i := 1; i <= 3; i++ {
		out[fmt.Sprintf("T%03d", i)] = "System.Location control has no API 68 capture"
	}
	return out
}

// locationAPI68Uncaught holds the 14 uncaught-exception rows missing from the
// primary API 68 capture. The native anonymous UNCAUGHT recapture
// native-capture (SHA-256
// 17ecb87bcf62b9d5989bb4fb3b8c0a50e8c939c140a3559894b0ba38ccdb49d4) answers
// each one; every answer equals the API 67 oracle text.
func locationAPI68Uncaught() map[string]string {
	return map[string]string{
		"L137": "UNCAUGHT|System.UnexpectedException: Latitude outside valid range.  Should be between -90 and 90.",
		"L138": "UNCAUGHT|System.UnexpectedException: Latitude outside valid range.  Should be between -90 and 90.",
		"L139": "UNCAUGHT|System.UnexpectedException: Longitude outside valid range. Should be between -180 and 180.",
		"L140": "UNCAUGHT|System.UnexpectedException: Longitude outside valid range. Should be between -180 and 180.",
		"L141": "UNCAUGHT|System.UnexpectedException: Latitude outside valid range.  Should be between -90 and 90.",
		"L142": "UNCAUGHT|System.UnexpectedException: Latitude value is missing ",
		"L143": "UNCAUGHT|System.UnexpectedException: Longitude value is missing ",
		"L145": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
		"L146": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
		"L147": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
		"L148": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
		"L150": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
		"L151": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
		"L152": "UNCAUGHT|System.UnexpectedException: Invalid distance unit specified",
	}
}

// API62/API67 answers are owned Salesforce observations, including exact Double
// text and exceptions that escape catch(Exception). API68 answers come from the
// primary anonymous capture native-capture
// (SHA-256 9af0cdac3fdbd556d827e71df3ea8e615a47c75d5f6ed801e0e6cdb6228c29fa).
// Its 146 answered rows equal the API67 oracle. It is missing the 14 uncaught
// rows L137-L143, L145-L148 and L150-L152, which the UNCAUGHT recapture answers
// (locationAPI68Uncaught), and the 22 controls C001-C019 and T001-T003, which
// stay excluded. Every row runs through both anonymous execution and a named
// @IsTest method without live calls. API 68 capture is anonymous-only, so the
// @IsTest checks at 68 count as legacy local regressions.
func TestLocationOrgConformance(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/conformance/location/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []locationCase
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 182 {
		t.Fatalf("rows=%d, want=182", len(rows))
	}
	uncaptured68 := locationAPI68Uncaptured()
	if len(uncaptured68) != 22 {
		t.Fatalf("API 68 uncaptured rows=%d, want=22", len(uncaptured68))
	}
	uncaught68 := locationAPI68Uncaught()
	if len(uncaught68) != 14 {
		t.Fatalf("API 68 recaptured uncaught rows=%d, want=14", len(uncaught68))
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "Location", api)
			namedKind := func(kind string) string {
				if api == "68.0" {
					// API 68 has no native named-route capture.
					return "legacy"
				}
				return kind
			}
			if api == "68.0" {
				t.Log("A22 API 68.0: native capture is anonymous-only; @IsTest checks keep anonymous-backed local assertions and count as legacy, not native named-route evidence")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			var source, uncaughtSource strings.Builder
			source.WriteString("@IsTest private class LocationOracleTest {\n")
			count, methods, rejections := 0, 0, 0
			uncaught := map[string]string{}
			seen := map[string]bool{}
			for _, row := range rows {
				if seen[row.ID] {
					t.Fatalf("duplicate row %s", row.ID)
				}
				seen[row.ID] = true
				if reason, ok := uncaptured68[row.ID]; ok && api == "68.0" {
					t.Logf("%s excluded at API 68: %s", row.ID, reason)
					continue
				}
				expected := row.Expected
				if api == "62.0" {
					expected = row.FloorExpected
				}
				if native, ok := uncaught68[row.ID]; ok && api == "68.0" {
					if native != expected {
						t.Fatalf("%s API 68 recapture <%s> differs from oracle <%s>", row.ID, native, expected)
					}
					expected = native
				}
				if strings.HasPrefix(expected, "COMPILE_ERROR") {
					rejections++
					passed := t.Run(row.ID+"/compile", func(t *testing.T) {
						code := locationCode(row)
						if result := sema.AnalyzeAnonymous(index, code, api); !result.HasErrors() {
							t.Fatalf("native rejected anonymous source: %s", code)
						}
						invalidRoot := t.TempDir()
						writeFile(t, filepath.Join(invalidRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						path := filepath.Join(invalidRoot, "force-app/main/default/classes/RejectedLocation.cls")
						writeFile(t, path, "@IsTest private class RejectedLocation { @IsTest static void rejected() {"+code+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := loadTestIndex(t, invalidRoot)
						if !idx.HasErrors() && !sema.Analyze(idx).HasErrors() {
							t.Fatalf("native rejected named source: %s", code)
						}
					})
					counts.record("anonymous", "category", 1, passed)
					counts.record("@IsTest", namedKind("category"), 1, passed)
					continue
				}
				body := locationAssertion(row, expected)
				counts.run(t, row.ID+"/anonymous", "anonymous", "exact", func(t *testing.T) {
					if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
						t.Fatalf("analysis: %v", result.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = anonymousRunner.execute(program)
					if strings.HasPrefix(expected, "UNCAUGHT|") {
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || "UNCAUGHT|"+runtimeErr.Error() != expected {
							t.Fatalf("uncaught=%v, want=%q", err, expected)
						}
					} else if err != nil {
						t.Fatal(err)
					}
				})
				if strings.HasPrefix(expected, "UNCAUGHT|") {
					name := "uncaught_" + row.ID
					uncaught[name] = expected
					fmt.Fprintf(&uncaughtSource, "@IsTest static void %s() {\n%s}\n", name, body)
					continue
				}
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
			source.WriteString("}\n" + uncaughtSource.String() + "}\n")
			wantCount, wantRejections, wantUncaught := 162, 6, 14
			if api == "68.0" {
				wantCount, wantRejections, wantUncaught = 143, 3, 14
			}
			if count != wantCount || rejections != wantRejections || len(uncaught) != wantUncaught {
				t.Fatalf("successful=%d rejected=%d uncaught=%d, want=%d/%d/%d", count, rejections, len(uncaught), wantCount, wantRejections, wantUncaught)
			}
			path := filepath.Join(root, "force-app/main/default/classes/LocationOracleTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			passed := t.Run("isTest", func(t *testing.T) {
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
				if got := run.Summary(); got.Total != methods+len(uncaught) || got.Passed != methods || got.Failed != len(uncaught) {
					t.Fatalf("summary=%#v, want=%d passing batches and %d native uncaught errors; first problem: %s", got, methods, len(uncaught), firstRunProblem(run))
				}
				checked := map[string]bool{}
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						expected, ok := uncaught[result.MethodName]
						if !ok {
							if result.Status != testreport.StatusPass {
								t.Errorf("%s: unexpected %s: %#v", result.MethodName, result.Status, result.Problem)
							}
							continue
						}
						if checked[result.MethodName] {
							t.Errorf("duplicate method %s", result.MethodName)
						}
						checked[result.MethodName] = true
						if result.Status != testreport.StatusFail || result.Problem == nil || "UNCAUGHT|"+result.Problem.Type+": "+result.Problem.Message != expected {
							t.Errorf("%s: status=%s problem=%#v, want=%q", result.MethodName, result.Status, result.Problem, expected)
						}
					}
				}
				if len(checked) != len(uncaught) {
					t.Fatalf("checked uncaught methods=%d, want=%d", len(checked), len(uncaught))
				}
			})
			counts.record("@IsTest", namedKind("exact"), count+len(uncaught), passed)
		})
	}
}
