package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// B41 type-text rows: Test.createStub proxy text, stubbed parameter names and
// inner-class stubs, cast exception type text, custom object and custom
// metadata type text, and global describe keys. Each row is captured on both
// routes where the shape runs anonymously (ids NS38*/NS48*), at API 62.0 and 67.0.
// Sources and metadata are the deployed bytes; only the terminal observation
// transport is replaced for local execution.
func TestTypeTextOrgConformance(t *testing.T) {
	type nativeRow struct {
		ID, APIVersion, Route, Behavior, Role, Twin, TestClass, Source string
		ExpectedRuntime, NativeTestMessage, NativeExceptionMessage     string
		ObservationExpression                                          string
		Sources, Metadata, SourceSha256                                map[string]string
	}
	type carry struct{ ID, Kind, Owner, Reason string }
	var data struct {
		APIVersions, Routes []string
		Captures            map[string]string
		// A carried difference is logged, not asserted (doc-1).
		Carries []carry
		Rows    []nativeRow
	}
	raw, err := os.ReadFile("testdata/conformance/type_text.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	// Capture provenance stays in glade-tools; rows keep the native row ids.
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" || len(data.Rows) != 114 ||
		len(data.Captures) != 1 || data.Captures["b41-type-text"] == "" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	ids := map[string]bool{}
	for _, row := range data.Rows {
		key := row.Route + "/" + row.APIVersion + "/" + row.ID
		anonymousID := strings.HasPrefix(row.ID, "NS38") || strings.HasPrefix(row.ID, "NS48")
		sources := row.Sources
		message := row.NativeTestMessage
		if row.Route == "anonymous" {
			sources, message = map[string]string{"anonymous": row.Source}, row.NativeExceptionMessage
		}
		if seen[key] || (row.Route == "anonymous") != anonymousID || (row.Route == "isTest" && sources[row.TestClass] == "") ||
			row.ExpectedRuntime == "" || message != "System.AssertException: Assertion Failed: P|"+row.ID+"|"+row.ExpectedRuntime {
			t.Fatalf("invalid native row: %s", key)
		}
		seen[key], ids[row.ID] = true, true
		hashed := map[string]string{}
		for name, code := range sources {
			hashed[name] = code
		}
		for path, code := range row.Metadata {
			hashed[path] = code
		}
		if len(hashed) != len(row.SourceSha256) {
			t.Fatalf("missing native sources: %s", key)
		}
		for name, code := range hashed {
			digest := sha256.Sum256([]byte(code))
			if hex.EncodeToString(digest[:]) != row.SourceSha256[name] {
				t.Fatalf("captured source digest changed: %s/%s", key, name)
			}
		}
	}
	// D35: every row has both APIs and its named twins.
	for _, row := range data.Rows {
		for _, api := range data.APIVersions {
			if !seen[row.Route+"/"+api+"/"+row.ID] {
				t.Fatalf("missing %s@%s", row.ID, api)
			}
		}
		for _, twin := range strings.Split(row.Twin, ",") {
			if twin = strings.TrimSpace(twin); !ids[twin] {
				t.Fatalf("missing twin %s of %s", twin, row.ID)
			}
		}
	}
	carried := map[string]carry{}
	for _, c := range data.Carries {
		if c.ID == "" || c.Kind == "" || c.Reason == "" || !strings.HasPrefix(c.Owner, "ROAD-") {
			t.Fatalf("carry without id, kind, reason or ROAD owner: %+v", c)
		}
		if !ids[c.ID] {
			t.Fatalf("stale carry id: %s (%s)", c.ID, c.Owner)
		}
		carried[c.ID] = c
	}
	// The terminal assertion compares the native text instead of failing.
	assertion := func(row nativeRow) (string, string) {
		observed := row.ObservationExpression
		if observed == "" {
			observed = "String.join(parts, ' ## ')"
		} else if observed != "observed" {
			t.Fatalf("unknown observation transport: %s", row.ID)
		}
		terminal := "System.assert(false, 'P|" + row.ID + "|' + " + observed + ");"
		return terminal, "System.assert(" + conformanceApexString(row.ExpectedRuntime) + ".equals(" + observed + "), '" + row.ID + " actual <' + " + observed + " + '>');"
	}
	report := func(t *testing.T, row nativeRow, problem string) {
		t.Helper()
		if c, ok := carried[row.ID]; ok {
			if problem == "" {
				t.Logf("carry now matches: %s (%s)", row.ID, c.Owner)
			} else {
				t.Logf("carry: %s (%s, %s): %s", row.ID, c.Owner, c.Kind, problem)
			}
			return
		}
		if problem != "" {
			t.Errorf("%s native <%s>: %s", row.ID, row.ExpectedRuntime, problem)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, row := range data.Rows {
				if row.APIVersion != api {
					continue
				}
				t.Run(row.Route+"/"+row.ID, func(t *testing.T) {
					terminal, replacement := assertion(row)
					if row.Route == "anonymous" {
						if strings.Count(row.Source, terminal) != 1 {
							t.Fatal("missing captured anonymous transport")
						}
						index, masked, problem := typeTextAnonymous(t, api, strings.Replace(row.Source, terminal, replacement, 1))
						if problem == "" {
							program, err := vm.CompileAnonymousWithOptions(masked, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							if _, err = runner.execute(program); err != nil {
								problem = err.Error()
							}
						}
						report(t, row, problem)
						return
					}
					test := row.Sources[row.TestClass]
					if strings.Count(test, terminal) != 1 {
						t.Fatal("missing captured terminal observation")
					}
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					base := filepath.Join(root, "force-app", "main", "default")
					for name, code := range row.Sources {
						if name == row.TestClass {
							code = strings.Replace(code, terminal, replacement, 1)
						}
						writeFile(t, filepath.Join(base, "classes", name+".cls"), code)
						writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					for path, code := range row.Metadata {
						writeFile(t, filepath.Join(base, filepath.FromSlash(path)), code)
					}
					index := loadTestIndex(t, root)
					if result := sema.Analyze(index); result.HasErrors() {
						t.Fatalf("local assertion semantics: %#v", result.Diagnostics)
					}
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 1 {
						t.Fatalf("captured @IsTest method missing: %#v", run.Suites)
					}
					problem := ""
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							if !strings.EqualFold(result.ClassName, row.TestClass) || result.Status != testreport.StatusPass || result.Problem != nil {
								problem = result.ClassName + " " + string(result.Status)
								if result.Problem != nil {
									problem += " " + result.Problem.Message
								}
							}
						}
					}
					report(t, row, problem)
				})
			}
		})
	}
}

// typeTextAnonymous compiles an anonymous row: block-local classes become the
// declaration context, and the masked block keeps native line positions.
func typeTextAnonymous(t *testing.T, api, source string) (typesys.Index, string, string) {
	t.Helper()
	parsed := apexast.NewParser().ParseSource("TypeTextAnonymous.cls", source)
	if (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
		t.Fatalf("anonymous parser: %#v", parsed.Diagnostics)
	}
	root := t.TempDir()
	masked := []byte(source)
	var names, paths []string
	declarations := map[string]string{}
	for _, decl := range parsed.Declarations {
		if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
			continue
		}
		start, end := decl.Range.Start.Offset, decl.Range.End.Offset
		padding := []byte(source[:start])
		for i := range padding {
			if padding[i] != '\n' && padding[i] != '\r' {
				padding[i] = ' '
			}
		}
		declarations[decl.Name] = string(padding) + source[start:end]
		names = append(names, decl.Name)
		for i := start; i < end; i++ {
			if masked[i] != '\n' && masked[i] != '\r' {
				masked[i] = ' '
			}
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(root, name+".cls")
		writeFile(t, path, declarations[name])
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
	if index.HasErrors() {
		t.Fatalf("native source parser: %#v", index.Diagnostics)
	}
	index = sema.WithAnonymousDeclarationContext(index)
	analysis := sema.AnalyzeAnonymousDeclarations(index)
	if !analysis.HasErrors() {
		analysis = sema.AnalyzeAnonymous(index, string(masked), api)
	}
	if analysis.HasErrors() {
		return index, string(masked), "local assertion semantics: " + analysis.Diagnostics[0].Message
	}
	return index, string(masked), ""
}
