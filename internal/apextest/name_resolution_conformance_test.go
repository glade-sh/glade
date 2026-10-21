package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Name resolution rows: custom labels as String, a same-spelled local,
// parameter or field as the receiver of Contact.X, SObjectType read through
// that local, block-scope end, and the implicit setter value against a
// same-named type. Sources are the deployed bytes with their companions and
// custom labels; only the terminal observation transport is replaced for
// local execution. A row with remainingReason differs from native and a row
// with messageGap matches the native verdict but not its exact text; both are
// carries with a roadmap owner, listed in carries. A carry that starts to
// match is logged, not failed, so a fix elsewhere never breaks this test.
func TestNameResolutionOrgConformance(t *testing.T) {
	type nativeRow struct {
		ID, APIVersion, Route, Behavior, Role, Twin, Capture, TestClass string
		Source, Terminal, Observed                                      string
		ExpectedCompilation, ExpectedRuntime                            string
		RemainingReason, MessageGap, Owner                              string
		Sources map[string]string `json:"declarations"`; Files, SourceSha256 map[string]string
		NativeDiagnostics                                               []struct {
			Class string
			Line  int
		}
	}
	var data struct {
		APIVersions, Routes []string
		Captures            map[string]string
		Normalization       string
		Carries             []struct{ ID, Kind, Owner string }
		Rows                []nativeRow
	}
	raw, err := os.ReadFile("testdata/conformance/name_resolution.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	// Capture provenance stays in glade-tools; rows keep the native row ids.
	if strings.Join(data.APIVersions, ",") != "62.0,65.0,66.0,67.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" || len(data.Rows) != 406 || len(data.Captures) != 4 {
		t.Fatal("incomplete native matrix")
	}
	for _, capture := range strings.Fields("area-M1 b41-h2 setter-value abstract-override") {
		if data.Captures[capture] == "" {
			t.Fatalf("missing capture description %s", capture)
		}
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.Route + "/" + row.APIVersion + "/" + row.ID
		if seen[key] || data.Captures[row.Capture] == "" || (row.Route != "anonymous" && row.Route != "isTest") || row.ExpectedCompilation == "" ||
			(row.ExpectedCompilation == "compiled") == (row.ExpectedRuntime == "") || (row.ExpectedRuntime != "" && (row.Terminal == "" || row.Observed == "")) ||
			(row.RemainingReason != "" && row.MessageGap != "") || (row.MessageGap != "" && row.ExpectedCompilation == "compiled") ||
			(row.RemainingReason != "" || row.MessageGap != "") != (row.Owner != "") {
			t.Fatalf("invalid native row: %s", key)
		}
		// Only the abstract-override rows were captured at 65.0 and 66.0.
		if (row.APIVersion == "65.0" || row.APIVersion == "66.0") && row.Capture != "abstract-override" {
			t.Fatalf("unexpected API version: %s", key)
		}
		seen[key] = true
		sources := map[string]string{}
		for name, code := range row.Sources {
			sources[name] = code
		}
		for name, code := range row.Files {
			sources[name] = code
		}
		if row.Route == "anonymous" {
			if row.Source == "" || row.TestClass != "" {
				t.Fatalf("invalid anonymous row: %s", key)
			}
			sources["anonymous"] = row.Source
		} else if len(row.Sources) == 0 || (row.TestClass != "" && row.Sources[row.TestClass] == "") || (row.ExpectedRuntime != "" && row.TestClass == "") {
			t.Fatalf("invalid named row: %s", key)
		}
		if len(sources) != len(row.SourceSha256) {
			t.Fatalf("missing native source digests: %s", key)
		}
		for name, code := range sources {
			digest := sha256.Sum256([]byte(code))
			if hex.EncodeToString(digest[:]) != row.SourceSha256[name] {
				t.Fatalf("captured source digest changed: %s/%s", key, name)
			}
		}
	}
	// Every carry names an owner and still has its row; no carried row is
	// left out of the list.
	listed := map[string]string{}
	for _, carry := range data.Carries {
		if carry.ID == "" || carry.Owner == "" || (carry.Kind != "remainingReason" && carry.Kind != "messageGap") || listed[carry.ID+"/"+carry.Kind] != "" {
			t.Fatalf("invalid carry: %+v", carry)
		}
		listed[carry.ID+"/"+carry.Kind] = carry.Owner
	}
	carried := map[string]bool{}
	for _, row := range data.Rows {
		kind := "remainingReason"
		if row.MessageGap != "" {
			kind = "messageGap"
		} else if row.RemainingReason == "" {
			continue
		}
		if listed[row.ID+"/"+kind] != row.Owner {
			t.Fatalf("carried row %s/%s %s (%s) is not in carries with that owner", row.Route, row.ID, kind, row.Owner)
		}
		carried[row.ID+"/"+kind] = true
	}
	for key := range listed {
		if !carried[key] {
			t.Fatalf("stale carry %s: no row carries it", key)
		}
	}
	// Each b41-h2 row is one route (anonymous ids end in -A); area-M1 and
	// abstract-override rows exist on both routes.
	for _, row := range data.Rows {
		if !seen[row.Route+"/62.0/"+row.ID] || !seen[row.Route+"/67.0/"+row.ID] {
			t.Fatalf("row %s/%s lacks the floor or ceiling API", row.Route, row.ID)
		}
		bothRoutes := row.Capture == "area-M1" || row.Capture == "abstract-override"
		if bothRoutes && (!seen["anonymous/"+row.APIVersion+"/"+row.ID] || !seen["isTest/"+row.APIVersion+"/"+row.ID]) {
			t.Fatalf("row %s@%s lacks a route", row.ID, row.APIVersion)
		}
		if row.Capture == "b41-h2" && (row.Route == "anonymous") != strings.HasSuffix(row.ID, "-A") {
			t.Fatalf("b41-h2 route mismatch: %s", row.ID)
		}
	}

	observation := func(items []diagnostic.Diagnostic, withLine bool) string {
		var messages []string
		for _, d := range items {
			if d.Severity != diagnostic.Error {
				continue
			}
			text := d.NativeMessage
			if text == "" {
				text = d.Message
			}
			line := 0
			if d.Range != nil {
				line = d.Range.Start.Line
			}
			if d.NativeLine != nil {
				line = *d.NativeLine
			}
			if withLine && line != 0 {
				text = fmt.Sprintf("line %d: %s", line, text)
			}
			// The IR and source checks can report one native rejection twice.
			duplicate := false
			for _, previous := range messages {
				duplicate = duplicate || previous == text
			}
			if !duplicate {
				messages = append(messages, text)
			}
		}
		if len(messages) == 0 {
			return "compiled"
		}
		return "COMPILE_ERROR\t" + strings.Join(messages, "; ")
	}
	// compare enforces the native verdict, and the exact text unless the
	// row records a message gap; a recorded gap or remaining reason that
	// no longer differs is stale.
	compare := func(t *testing.T, row nativeRow, actual string) bool {
		t.Helper()
		verdict := (actual == "compiled") == (row.ExpectedCompilation == "compiled")
		switch {
		case row.RemainingReason != "":
			if actual == row.ExpectedCompilation {
				t.Logf("carry now matches: %s (%s)", row.ID, row.Owner)
			} else {
				t.Logf("remaining %s %s (%s): native <%s>, actual <%s>: %s", row.Route, row.ID, row.Owner, row.ExpectedCompilation, actual, row.RemainingReason)
			}
			return false
		case row.MessageGap != "":
			if !verdict {
				t.Fatalf("native <%s>, actual <%s>", row.ExpectedCompilation, actual)
			}
			if actual == row.ExpectedCompilation {
				t.Logf("carry now matches: %s (%s)", row.ID, row.Owner)
			}
		case actual != row.ExpectedCompilation:
			t.Fatalf("native <%s>, actual <%s>", row.ExpectedCompilation, actual)
		}
		return true
	}
	writeProject := func(t *testing.T, api string, row nativeRow, extra map[string]string) (string, []string) {
		t.Helper()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
		for name, code := range row.Files {
			writeFile(t, filepath.Join(root, "force-app", "main", "default", filepath.FromSlash(name)), code)
		}
		classes := map[string]string{}
		for name, code := range row.Sources {
			classes[name] = code
		}
		for name, code := range extra {
			classes[name] = code
		}
		var names, paths []string
		for name := range classes {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			path := filepath.Join(root, "force-app", "main", "default", "classes", name+".cls")
			writeFile(t, path, classes[name])
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			paths = append(paths, path)
		}
		return root, paths
	}
	build := func(t *testing.T, root string, paths []string) typesys.Index {
		t.Helper()
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		p.ApexFiles = append([]string{}, paths...)
		index := typesys.Build(p, schema.Schema{})
		if index.HasErrors() {
			t.Fatalf("native source parser: %#v", index.Diagnostics)
		}
		return index
	}
	// Anonymous block-local classes join the project as transient
	// declarations; the masked block keeps line positions for native lines.
	anonymous := func(t *testing.T, api string, row nativeRow, source string) (typesys.Index, string, string) {
		t.Helper()
		parsed := apexast.NewParser().ParseSource("NameResolutionAnonymous.cls", source)
		if (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
			t.Fatalf("anonymous parser: %#v", parsed.Diagnostics)
		}
		declarations := map[string]string{}
		masked := []byte(source)
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
			declarations["GladeAnonymous"+decl.Name] = string(padding) + source[start:end]
			for i := start; i < end; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
		root, companionPaths := writeProject(t, api, row, nil)
		if len(row.Sources) != 0 {
			// Native deployed the companions as ordinary top-level classes.
			if analysis := sema.Analyze(build(t, root, companionPaths)); analysis.HasErrors() {
				return typesys.Index{}, "", observation(analysis.Diagnostics, true)
			}
		}
		transientPaths := []string{}
		var names []string
		for name := range declarations {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			path := filepath.Join(root, "anonymous", name+".cls")
			writeFile(t, path, declarations[name])
			transientPaths = append(transientPaths, path)
		}
		index := build(t, root, append(append([]string{}, companionPaths...), transientPaths...))
		transient := index
		transient.Types = nil
		for i := range index.Types {
			if strings.HasPrefix(filepath.Base(index.Types[i].File), "GladeAnonymous") {
				index.Types[i].Modifiers = append(append([]string(nil), index.Types[i].Modifiers...), vm.AnonymousClassModifier)
				transient.Types = append(transient.Types, index.Types[i])
			}
		}
		analysis := sema.AnalyzeAnonymousDeclarationsInContext(index, transient)
		if !analysis.HasErrors() {
			analysis = sema.AnalyzeAnonymous(index, string(masked), api)
		}
		return index, string(masked), observation(analysis.Diagnostics, true)
	}
	replaceTerminal := func(t *testing.T, row nativeRow, source string) string {
		t.Helper()
		if strings.Count(source, row.Terminal) != 1 {
			t.Fatal("missing captured terminal observation")
		}
		return strings.Replace(source, row.Terminal, "System.assert("+conformanceApexString(row.ExpectedRuntime)+".equals("+row.Observed+"), '"+row.ID+" actual <'+("+row.Observed+")+'>');", 1)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			for _, row := range data.Rows {
				if row.APIVersion != api {
					continue
				}
				t.Run(row.Route+"/"+row.ID, func(t *testing.T) {
					if row.Route == "anonymous" {
						if _, _, actual := anonymous(t, api, row, row.Source); !compare(t, row, actual) || row.ExpectedRuntime == "" {
							return
						}
						index, masked, actual := anonymous(t, api, row, replaceTerminal(t, row, row.Source))
						if actual != "compiled" {
							t.Fatalf("local assertion semantics: %s", actual)
						}
						program, err := vm.CompileAnonymousWithOptions(masked, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						if _, err = runner.execute(program); err != nil {
							t.Fatal(err)
						}
						return
					}
					root, paths := writeProject(t, api, row, nil)
					analysis := sema.Analyze(build(t, root, paths))
					if !compare(t, row, observation(analysis.Diagnostics, false)) {
						return
					}
					for _, native := range row.NativeDiagnostics {
						matched := false
						for _, d := range analysis.Diagnostics {
							matched = matched || (d.Severity == diagnostic.Error && strings.EqualFold(strings.TrimSuffix(filepath.Base(d.File), ".cls"), native.Class) && (native.Line == 0 || d.Range != nil && d.Range.Start.Line == native.Line))
						}
						if !matched {
							t.Fatalf("native rejection is %s line %d: %#v", native.Class, native.Line, analysis.Diagnostics)
						}
					}
					// A class native compiled must not carry a local error
					// (GATE3-01: the setter's implicit value is not class Value).
					if len(row.NativeDiagnostics) != 0 {
						for _, d := range analysis.Diagnostics {
							rejected := false
							for _, native := range row.NativeDiagnostics {
								rejected = rejected || strings.EqualFold(strings.TrimSuffix(filepath.Base(d.File), ".cls"), native.Class)
							}
							if d.Severity == diagnostic.Error && !rejected {
								t.Fatalf("native compiles %s: %#v", d.File, d)
							}
						}
					}
					if row.ExpectedRuntime == "" {
						return
					}
					sources := map[string]string{}
					for name, code := range row.Sources {
						sources[name] = code
					}
					sources[row.TestClass] = replaceTerminal(t, row, row.Sources[row.TestClass])
					root, paths = writeProject(t, api, nativeRow{Sources: sources, Files: row.Files}, nil)
					index := build(t, root, paths)
					if result := sema.Analyze(index); result.HasErrors() {
						t.Fatalf("local assertion semantics: %#v", result.Diagnostics)
					}
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 1 {
						t.Fatalf("captured @IsTest method missing: %#v", run.Suites)
					}
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							if !strings.EqualFold(result.ClassName, row.TestClass) || result.Status != testreport.StatusPass || result.Problem != nil {
								problem := ""
								if result.Problem != nil {
									problem = result.Problem.Message
								}
								t.Errorf("%s native <%s>: %s %s %s", row.ID, row.ExpectedRuntime, result.ClassName, result.Status, problem)
							}
						}
					}
				})
			}
		})
	}
}
