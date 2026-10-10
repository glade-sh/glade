package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type urlDomainCase struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Compile       bool   `json:"compile"`
	Expected      string `json:"expected"`
	FloorExpected string `json:"floorExpected"`
	Carry         string `json:"carry"`
}

func urlDomainQuote(text string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(text) + "'"
}

func urlDomainCode(row urlDomainCase) string {
	code := row.Code
	if row.Compile {
		code = strings.Split(code, "pq.out(")[0]
		// A compile-control row can also be accepted by the native compiler.
		if !strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
			code += "r = 'compiled';"
		}
	}
	if !strings.Contains(code, ";") {
		code = "r = " + code + ";"
	}
	return "Object r; " + code
}

func urlDomainAssertion(row urlDomainCase, expected string) string {
	// Instance toString renders DomainType enum names as well as scalar values.
	observedType, observedValue := "String", "r == null ? null : r.toString()"
	assertion := "String expectedText = " + urlDomainQuote(expected) + "; System.assert(expectedText.equals(observedText), " + urlDomainQuote(row.ID) + " + ' expected <' + expectedText + '> actual <' + observedText + '>');\n"
	if expected == "null" {
		// All null answers in this table are raw URL/Domain getter results.
		// The String 'null' or a caught exception must fail this check.
		observedType, observedValue = "Object", "r"
		assertion = "System.assert(observedText == null, " + urlDomainQuote(row.ID) + " + ' expected <null> actual <' + String.valueOf(observedText) + '>');\n"
	}
	return observedType + " observedText; try { " + urlDomainCode(row) + " observedText = " + observedValue + "; } catch(Exception e) { observedText = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } " + assertion
}

// Owned API62/API67 answers are exported without credentials or live calls.
// Hostnames are relative to the saved org origin: ${ORG_LABEL} substitutes a
// synthetic identity while retaining the captured host structure. Carries
// retain their exact native answer and the reason they are outside this family.
func TestURLDomainOrgConformance(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/conformance/url_domain/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []urlDomainCase
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 271 {
		t.Fatalf("rows=%d, want=271", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymousMatches, anonymousTotal, namedCompileMatches, namedMatches := 0, 0, 0, 0
			compilerTotal := 0
			defer func() {
				t.Logf("URL and Domain API %s anonymous exact %d/%d; category-only %d; carried 0; partial 0; legacy 0", api, anonymousMatches-namedCompileMatches, anonymousTotal-compilerTotal, compilerTotal)
				t.Logf("URL and Domain API %s @IsTest exact %d/%d; category-only %d; carried 0; partial 0; legacy 0", api, namedMatches, anonymousTotal-compilerTotal, compilerTotal)
			}()
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  domainUrl: https://url-domain.scratch.my.salesforce.com\n")
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			var source strings.Builder
			source.WriteString("@IsTest private class URLDomainOracleTest {\n")
			count, methods, rejections, carries := 0, 0, 0, 0
			seen := map[string]bool{}
			for _, row := range rows {
				if seen[row.ID] {
					t.Fatalf("duplicate row %s", row.ID)
				}
				seen[row.ID] = true
				if row.Carry != "" {
					carries++
					t.Logf("%s carried: %s; native=%s", row.ID, row.Carry, row.Expected)
					continue
				}
				expected := row.Expected
				if api == "62.0" {
					expected = row.FloorExpected
				}
				expected = strings.ReplaceAll(expected, "${ORG_LABEL}", "url-domain")
				anonymousTotal++
				if strings.HasPrefix(expected, "COMPILE_ERROR") {
					rejections++
					compilerTotal++
					if t.Run(row.ID+"/compile", func(t *testing.T) {
						code := urlDomainCode(row)
						if result := sema.AnalyzeAnonymous(index, code, api); !result.HasErrors() {
							t.Fatalf("native rejected anonymous source: %s", code)
						}
						invalidRoot := t.TempDir()
						writeFile(t, filepath.Join(invalidRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						path := filepath.Join(invalidRoot, "force-app/main/default/classes/RejectedURLDomain.cls")
						writeFile(t, path, "@IsTest private class RejectedURLDomain { @IsTest static void rejected() {"+code+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := loadTestIndex(t, invalidRoot)
						if !idx.HasErrors() && !sema.Analyze(idx).HasErrors() {
							t.Fatalf("native rejected named source: %s", code)
						}
					}) {
						anonymousMatches++
						namedCompileMatches++
					}
					continue
				}
				body := urlDomainAssertion(row, expected)
				if t.Run(row.ID+"/anonymous", func(t *testing.T) {
					if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
						t.Fatalf("analysis: %v", result.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := anonymousRunner.execute(program, func(machine *vm.VM) {
						machine.SetServerBaseURL("https://request.example.test:8443")
					}); err != nil {
						t.Fatal(err)
					}
				}) {
					anonymousMatches++
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
			source.WriteString("}\n}\n")
			if count != 259 || rejections != 6 || carries != 6 {
				t.Fatalf("runtime=%d rejected=%d carried=%d, want=259/6/6", count, rejections, carries)
			}
			path := filepath.Join(root, "force-app/main/default/classes/URLDomainOracleTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			if t.Run("isTest", func(t *testing.T) {
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
				if got := run.Summary(); got.Total != methods || got.Passed != methods {
					t.Fatalf("summary=%#v, want=%d passing batches; first problem: %s", got, methods, firstRunProblem(run))
				}
			}) {
				namedMatches = count
			}
		})
	}
}
