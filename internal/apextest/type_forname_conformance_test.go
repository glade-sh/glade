package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Each project is private to one route/row: anonymous Folder declarations must
// retain request-local provenance, and baseline rows must have no Folder class.
func TestTypeForNameConformance(t *testing.T) {
	var data struct {
		APIVersions      []string          `json:"apiVersions"`
		Files            map[string]string `json:"files"`
		AnonymousPrelude string            `json:"anonymousPrelude"`
		Cases            []struct {
			ID              string            `json:"id"`
			OracleID        string            `json:"oracleId"`
			Shadow          bool              `json:"shadow"`
			Code            string            `json:"code"`
			Expected        string            `json:"expected"`
			ExpectedByRoute map[string]string `json:"expectedByRoute"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/type_forname.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 30 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatal("incomplete type-forName export")
	}
	seen := map[string]bool{}
	for _, tc := range data.Cases {
		if tc.ID != tc.OracleID || seen[tc.ID] || tc.Code == "" {
			t.Fatalf("invalid oracle ID %s", tc.ID)
		}
		seen[tc.ID] = true
	}
	// Exactly the native anonymous transport's five-line helper prelude.
	const helper = "class P {\n  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v)); }\n  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }\n}\nP pq = new P();\n"
	compileText := func(ds []diagnostic.Diagnostic, anonymous bool) string {
		var messages []string
		for _, d := range ds {
			if d.Severity != diagnostic.Error {
				continue
			}
			message := d.Message
			if d.NativeMessage != "" {
				message = d.NativeMessage
			}
			if anonymous {
				if d.Range == nil {
					t.Fatal("anonymous diagnostic missing range")
				}
				message = fmt.Sprintf("line %d: %s", d.Range.Start.Line, message)
			}
			messages = append(messages, message)
		}
		if len(messages) == 0 {
			return ""
		}
		return "COMPILE_ERROR\t" + strings.Join(messages, "\n")
	}
	for _, api := range data.APIVersions {
		for _, route := range []string{"anonymous", "named"} {
			for _, tc := range data.Cases {
				t.Run(api+"/"+route+"/"+tc.ID, func(t *testing.T) {
					expected := tc.Expected
					if v, ok := tc.ExpectedByRoute[route]; ok {
						expected = v
					}
					root := t.TempDir()
					var files []string
					add := func(name, source string) {
						path := filepath.Join(root, name)
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						files = append(files, path)
					}
					body := ""
					if route == "anonymous" {
						source := helper
						if tc.Shadow {
							source += data.AnonymousPrelude + "\n"
						}
						source += "try { Object r; " + tc.Code + " pq.out('" + tc.ID + "', r); } catch (Exception e) { pq.err('" + tc.ID + "', e); }\n"
						// Match request preparation: move declarations into transient files and
						// replace their bytes with spaces while retaining source line positions.
						masked := []byte(source)
						for _, decl := range apexast.ParseSource("TypeForNameAnonymous.cls", source).Declarations {
							if decl.Kind != apexast.DeclarationClass {
								continue
							}
							start, end := decl.Range.Start.Offset, decl.Range.End.Offset
							if start < 0 || end <= start || end > len(source) {
								t.Fatal("invalid declaration range")
							}
							prefix := []byte(source[:start])
							for i := range prefix {
								if prefix[i] != '\n' && prefix[i] != '\r' {
									prefix[i] = ' '
								}
							}
							add(decl.Name+".cls", string(prefix)+source[start:end])
							for i := start; i < end; i++ {
								if masked[i] != '\n' && masked[i] != '\r' {
									masked[i] = ' '
								}
							}
						}
						body = string(masked)
					} else {
						if tc.Shadow {
							for name, source := range data.Files {
								add(name, source)
							}
						}
						// The captured named route uses an external top-level Folder and one
						// generated @IsTest class. Replace only its terminal transport assertion.
						source := "@IsTest private class TypeForNameRow {\n@IsTest static void " + tc.ID + "() {\nObject r;\ntry {\n" + tc.Code + "\n} catch(Exception e) { r='EXC|'+e.getTypeName()+'|'+e.getMessage(); }\n"
						source += "System.assertEquals(" + conformanceApexString(expected) + ",String.valueOf(r));\n}\n}\n"
						add("TypeForNameRow.cls", source)
					}
					idx := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, gladeschema.Schema{})
					if idx.HasErrors() {
						t.Fatal(idx.Diagnostics)
					}
					var ds []diagnostic.Diagnostic
					if route == "anonymous" {
						idx = sema.WithAnonymousDeclarationContext(idx)
						ds = append(ds, sema.AnalyzeAnonymousDeclarations(idx).Diagnostics...)
						ds = append(ds, sema.AnalyzeAnonymous(idx, body, api).Diagnostics...)
					} else {
						ds = sema.Analyze(idx).Diagnostics
					}
					actual := compileText(ds, route == "anonymous")
					if strings.HasPrefix(expected, "COMPILE_ERROR\t") {
						if actual != expected {
							t.Fatalf("expected <%s> actual <%s>", expected, actual)
						}
						t.Logf("TFN_RESULT|%s|%s|%s|%s", api, route, tc.ID, actual)
						return
					}
					if actual != "" {
						t.Fatal(actual)
					}
					if route == "anonymous" {
						org := orgFromIndex(idx)
						runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						prefix := "P|" + tc.ID + "|"
						count := 0
						for _, line := range result.Debug {
							if strings.HasPrefix(line, prefix) {
								actual = strings.TrimPrefix(line, prefix)
								count++
							}
						}
						if count != 1 || actual != expected {
							t.Fatalf("observations=%d expected <%s> actual <%s>", count, expected, actual)
						}
					} else {
						run := Run(idx, Options{NoDiskCache: true, Parallelism: 1})
						count := 0
						for _, suite := range run.Suites {
							for _, c := range suite.Cases {
								count++
								if c.MethodName != tc.ID || c.Status != testreport.StatusPass {
									t.Fatalf("method=%s status=%s problem=%v", c.MethodName, c.Status, c.Problem)
								}
							}
						}
						if count != 1 {
							t.Fatalf("named methods executed: %d", count)
						}
						actual = expected // The named method compared the exact observed string.
					}
					t.Logf("TFN_RESULT|%s|%s|%s|%s", api, route, tc.ID, actual)
				})
			}
		}
	}
}
