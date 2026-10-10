package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type l08Case struct {
	ID                 string              `json:"id"`
	Kind               string              `json:"kind"`
	Group              string              `json:"group"`
	JS                 string              `json:"js"`
	Template           string              `json:"template"`
	Apex               string              `json:"apex"`
	ApexBase           string              `json:"apex_base"`
	RuntimeID          string              `json:"runtime_id"`
	Expected           map[string]string   `json:"expected"`
	CompileExpected    map[string]string   `json:"compile_expected"`
	SourceIndex        int                 `json:"source_index"`
	DiagnosticExpected map[string][]string `json:"diagnostic_expected"`
	DiagnosticOwner    string              `json:"diagnostic_owner"`
	DiagnosticCarried  string              `json:"diagnostic_carried"`
	Carried            string              `json:"carried"`
}

type l08Table struct {
	ClassPrefix string    `json:"class_prefix"`
	Controller  string    `json:"controller"`
	DTO         string    `json:"dto"`
	Exception   string    `json:"exception"`
	Cases       []l08Case `json:"cases"`
}

// TestL08SalesforceConformance replays owned compiler/metadata and rendered Apex
// bridge observations exported from the API 59 and 67 org.tsv and runtime.tsv
// captures. The exported sources keep the native API class suffix, so
// custom-exception names compare exactly without normalizing the two raw rows.
// Review controls add top-level custom exceptions and DTOs with populated
// unannotated fields/properties, captured through imperative and wired calls.
// CI uses only local fixtures; no capture tool or Salesforce access is required.
// Phase-one GLADE_L08_CAPTURE logs all differences without failing row checks;
// GLADE_L08_REPORT receives the accepted-behaviour before report.
func TestL08SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L08_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l08_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table l08Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var runtimeCases []l08Case
	compileCases := 0
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate L08 row %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") || (c.Kind == "runtime" && !strings.HasPrefix(want, "DOM|")) {
				t.Fatalf("missing native L08 answer: %s API %s", c.ID, api)
			}
			if c.Kind == "runtime" && c.CompileExpected[api] != "COMPILE_OK" {
				t.Fatalf("missing native runtime-source compilation: %s API %s", c.ID, api)
			}
		}
		if c.Kind == "runtime" {
			runtimeCases = append(runtimeCases, c)
		} else {
			compileCases++
		}
	}
	if compileCases != 154 || len(runtimeCases) != 132 || table.Controller == "" || table.DTO == "" {
		t.Fatalf("L08 requires owned controller/DTO, 154 compile and 132 runtime rows; got %d/%d", compileCases, len(runtimeCases))
	}
	for _, group := range []string{"imperative-calls", "cacheable-wire", "error-shapes", "serialization", "refreshApex-cache-keys"} {
		if !groups[group] {
			t.Fatalf("missing L08 group %s", group)
		}
	}
	data, err = os.ReadFile("testdata/l08_review_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controls l08Table
	if err := json.Unmarshal(data, &controls); err != nil {
		t.Fatal(err)
	}
	controlCompile, controlRuntime := 0, 0
	for _, c := range controls.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") || c.Carried != "" || c.DiagnosticCarried != "" {
			t.Fatalf("invalid, duplicate or carried L08 review control %q", c.ID)
		}
		seen[c.ID] = true
		for _, api := range versions {
			if c.Kind == "compile" && c.Expected[api] != "COMPILE_OK" {
				t.Fatalf("missing native control compilation: %s API %s", c.ID, api)
			}
			if c.Kind == "runtime" && (!strings.HasPrefix(c.Expected[api], "DOM|") || c.CompileExpected[api] != "COMPILE_OK") {
				t.Fatalf("missing native control DOM/source answer: %s API %s", c.ID, api)
			}
		}
		if c.Kind == "compile" {
			controlCompile++
		} else {
			controlRuntime++
		}
	}
	if controls.ClassPrefix != "FamilyL08Review" || controls.Controller == "" || controls.DTO == "" || controls.Exception == "" || controlCompile != 7 || controlRuntime != 4 {
		t.Fatalf("L08 review requires distinguishing fixtures, seven compile and four runtime controls; got %d/%d", controlCompile, controlRuntime)
	}
	fixtures := []l08Table{table, controls}
	caseTotal := len(table.Cases) + len(controls.Cases)
	runtimeTotal := len(runtimeCases) + controlRuntime
	// Missing toolchain inputs are infrastructure failures, never compiler matches.
	repo, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := gladehome.LWCToolchainDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(repo, "third_party/lwc/compile.mjs"), filepath.Join(toolchain, "node_modules/@lwc/compiler/package.json"), filepath.Join(toolchain, "node_modules/@lwc/engine-dom/dist/index.js"), filepath.Join(toolchain, "node_modules/@lwc/synthetic-shadow/dist/index.js")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	var report strings.Builder
	matches, total, nativeDOMRows, localDOMRows, carried := 0, 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal := 0, 0
	for _, api := range versions {
		apiMatches := 0
		for _, table := range fixtures {
			runtimeCases := []l08Case{}
			for _, c := range table.Cases {
				if c.Kind == "runtime" {
					runtimeCases = append(runtimeCases, c)
				}
			}
			fixtureMatches := 0
			baseRoot := t.TempDir()
			l08WriteProject(t, baseRoot, api)
			l08WriteApex(t, baseRoot, table, api)
			baseProject, baseErr := project.Load(baseRoot)
			if baseErr == nil {
				_, baseErr = l08Index(baseProject)
			}
			dom, domErrors, domErr := l08ObserveDOM(t, table, api, runtimeCases, repo)
			compileErrs := l08CompileRows(t, table, api)
			// Runtime-source compiler outcomes are distinct rows in the table.
			for _, c := range table.Cases {
				got, reason := "", ""
				want := c.Expected[api]
				diagnosticGot, diagnosticWant := "", ""
				if c.Kind == "compile" {
					compileErr, ok := compileErrs[c.ID]
					if !ok {
						t.Fatalf("L08 compile observation missing for %s API %s", c.ID, api)
					}
					got = "COMPILE_OK"
					if c.ApexBase == "" && baseErr != nil {
						// A broken shared dependency cannot certify a rejected row.
						got = "COMPILE_DEPENDENCY_ERROR"
						reason = "L08: shared controller/DTO compilation failed: " + baseErr.Error()
					} else if compileErr != nil {
						got = "COMPILE_ERROR"
						if got != want {
							reason = "L08: local compiler rejects native-accepted source: " + compileErr.Error()
						}
					} else if got != want {
						reason = "L08: local compiler accepts native-rejected declaration/import"
					}
					if native := c.DiagnosticExpected[api]; len(native) > 0 {
						diagnosticTotal++
						expectedJSON, err := json.Marshal(native)
						if err != nil {
							t.Fatal(err)
						}
						diagnosticWant = string(expectedJSON)
						diagnosticGot = l08CompileDiagnostics(compileErr)
						if got == want && diagnosticGot == diagnosticWant {
							diagnosticMatches++
						} else {
							// Exact native diagnostics stay visible. Cross-family text
							// differences never turn into an assertion of local wording.
							diagnosticReason := c.DiagnosticCarried
							if diagnosticReason == "" {
								diagnosticReason = c.DiagnosticOwner + ": local compiler diagnostic text differs from native"
							}
							t.Logf("%s API %s diagnostic carry %s expected <%s> actual <%s>", c.ID, api, diagnosticReason, diagnosticWant, diagnosticGot)
							if reason == "" {
								reason = diagnosticReason
							}
							if !capture && c.DiagnosticCarried == "" {
								t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
							}
						}
					}
				} else {
					nativeDOMRows++
					if raw, ok := dom[c.ID]; ok {
						got, err = l08DOMText(raw)
						if err != nil {
							t.Fatal(err)
						}
						localDOMRows++
					} else {
						got = "DOM_ERROR"
						reason = "L08: local rendered bridge observation missing"
						if domErrors[c.ID] != "" {
							reason += ": " + domErrors[c.ID]
						} else if domErr != nil {
							reason += ": " + domErr.Error()
						}
					}
					if got != want && reason == "" {
						reason = "L08: rendered " + c.Group + " output differs from native"
					}
				}
				status := "MISMATCH"
				// Go equality compares the exact row text: case, Id length, and null.
				if got == want {
					status = "MATCH"
					matches++
					apiMatches++
					fixtureMatches++
				} else if c.Carried != "" {
					status, reason = "CARRIED", c.Carried
					carried++
					t.Logf("%s API %s carry %s expected <%s> actual <%s>", c.ID, api, reason, want, got)
				}
				total++
				fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, got, want, l08OneLine(reason), diagnosticGot, diagnosticWant)
				if !capture && status != "MATCH" && status != "CARRIED" {
					t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l08OneLine(reason))
				}
			}
			prefix := table.ClassPrefix
			if prefix == "" {
				prefix = "FamilyL08"
			}
			fmt.Fprintf(&report, "FIXTURE_TOTAL\t%s\t%s\t%d/%d\n", api, prefix, fixtureMatches, len(table.Cases))
			t.Logf("L08 API %s %s matches %d/%d", api, prefix, fixtureMatches, len(table.Cases))
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\n", api, apiMatches, caseTotal)
		t.Logf("L08 API %s matches %d/%d", api, apiMatches, caseTotal)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nNATIVE_DOM_ROWS\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\nCARRIED\t%d/%d\nCOMPILE_DIAGNOSTIC_TOTAL\t%d/%d\n", matches, total, nativeDOMRows, runtimeTotal*len(versions), localDOMRows, runtimeTotal*len(versions), carried, total, diagnosticMatches, diagnosticTotal)
	t.Logf("L08 matches %d/%d; native DOM %d; local DOM %d", matches, total, nativeDOMRows, localDOMRows)
	if path := os.Getenv("GLADE_L08_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var (
	l08CompileDiagnosticRE = regexp.MustCompile(`(?m)^(?:Error|SyntaxError): ([^\r\n]+)\r?$`)
	l08LWCCodeRE           = regexp.MustCompile(`\bLWC[0-9]+: [^\r\n]+$`)
)

func l08CompileDiagnostics(err error) string {
	messages := []string{}
	if err != nil {
		if match := l08CompileDiagnosticRE.FindStringSubmatch(err.Error()); match != nil {
			message := match[1]
			// The compiler prefixes wire errors with the temporary filename.
			// Keep its exact code and message, excluding paths and stack frames.
			if coded := l08LWCCodeRE.FindString(message); coded != "" {
				message = coded
			}
			messages = append(messages, message)
		} else {
			messages = append(messages, l08OneLine(err.Error()))
		}
	}
	sort.Strings(messages)
	data, _ := json.Marshal(messages)
	return string(data)
}

func l08Tokens(table l08Table, source, api, extra string) string {
	controller, dto, exception := l08Names(table, api)
	return strings.NewReplacer("__CTRL__", controller, "__DTO__", dto, "__TOP_EXCEPTION__", exception, "__EXTRA__", extra).Replace(source)
}

func l08Names(table l08Table, api string) (string, string, string) {
	prefix := table.ClassPrefix
	if prefix == "" {
		prefix = "FamilyL08"
	}
	return prefix + "ControllerV" + api[:2], prefix + "DtoV" + api[:2], prefix + "V" + api[:2] + "LocalException"
}

func l08WriteProject(t *testing.T, root, api string) {
	t.Helper()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
}

func l08WriteClass(t *testing.T, root, name, source, api string) {
	t.Helper()
	path := filepath.Join(root, "force-app/main/default/classes", name)
	writeLightningFixtureFile(t, path+".cls", source)
	writeLightningFixtureFile(t, path+".cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
}

func l08WriteApex(t *testing.T, root string, table l08Table, api string) {
	t.Helper()
	controller, dto, exception := l08Names(table, api)
	l08WriteClass(t, root, controller, l08Tokens(table, table.Controller, api, ""), api)
	l08WriteClass(t, root, dto, l08Tokens(table, table.DTO, api, ""), api)
	if table.Exception != "" {
		l08WriteClass(t, root, exception, l08Tokens(table, table.Exception, api, ""), api)
	}
}

func l08WriteBundle(t *testing.T, root, name string, table l08Table, c l08Case, api, extra string) {
	t.Helper()
	path := filepath.Join(root, "force-app/main/default/lwc", name, name)
	js := strings.ReplaceAll(l08Tokens(table, c.JS, api, extra), "FamilyBridge", strings.ToUpper(name[:1])+name[1:])
	writeLightningFixtureFile(t, path+".js", js)
	writeLightningFixtureFile(t, path+".html", c.Template)
	writeLightningFixtureFile(t, path+".js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
}

func l08Index(p project.Project) (typesys.Index, error) {
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, err
	}
	index := typesys.Build(p, schema)
	result := sema.AnalyzeWithOptions(index, sema.AnalyzeOptions{Diagnostics: true})
	for _, d := range append(append([]diagnostic.Diagnostic(nil), index.Diagnostics...), result.Diagnostics...) {
		if d.Severity == diagnostic.Error {
			return index, fmt.Errorf("%s: %s", d.Code, d.Message)
		}
	}
	return index, nil
}

// l08CompileRows observes every compile row of one fixture and API. Each row
// keeps its own project for project loading and Apex analysis. The LWC
// compiler never reads Apex classes, so the bundles of rows that pass both
// are compiled with compile.CompileBatch in one project with the same
// relative paths: one Node run instead of one Compile per row. A failing
// bundle does not stop the others; a batch process failure fails the test.
func l08CompileRows(t *testing.T, table l08Table, api string) map[string]error {
	t.Helper()
	errs := map[string]error{}
	type pending struct {
		c       l08Case
		root    string
		project project.Project
	}
	var batch []pending
	batchRoot := t.TempDir()
	l08WriteProject(t, batchRoot, api)
	l08WriteApex(t, batchRoot, table, api)
	for _, c := range table.Cases {
		if c.Kind != "compile" {
			continue
		}
		root := t.TempDir()
		l08WriteCompileRow(t, root, table, api, c, true)
		p, err := project.Load(root)
		if err != nil {
			errs[c.ID] = err
			continue
		}
		// Base controller and DTO rows observe their own Apex compilation, not an
		// empty LWC project. Declaration rows check Apex plus the importing bundle.
		if _, err := l08Index(p); err != nil {
			errs[c.ID] = err
			continue
		}
		errs[c.ID] = nil
		if c.ApexBase != "" {
			continue
		}
		l08WriteCompileRow(t, batchRoot, table, api, c, false)
		batch = append(batch, pending{c: c, root: root, project: p})
	}
	if len(batch) == 0 {
		return errs
	}
	p, err := project.Load(batchRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LWCMetaFiles) != len(batch) {
		t.Fatalf("L08 batch project loaded %d of %d bundles", len(p.LWCMetaFiles), len(batch))
	}
	outDir := filepath.Join(batchRoot, "dist")
	results, err := compile.CompileBatch(p, compile.Options{OutDir: outDir})
	if err != nil {
		t.Fatalf("L08 compiler environment failure: %v", err)
	}
	if len(results) != len(batch) {
		t.Fatalf("L08 batch returned %d of %d bundles", len(results), len(batch))
	}
	controls := l08BatchControls(table, api)
	compared, verbatim := 0, 0
	for _, row := range batch {
		key := "force-app/main/default/lwc/" + l08CompileBundleName(api, row.c)
		result, ok := results[key]
		if !ok {
			t.Fatalf("L08 batch result missing for %s", row.c.ID)
		}
		errs[row.c.ID] = result.Err
		if controls[row.c.ID] {
			compared++
			l08AssertBatchControl(t, row.c, api, row.project, row.root, batchRoot, result)
		}
		if result.Err != nil && (row.c.Expected[api] != "COMPILE_ERROR" || !l08CompileDiagnosticRE.MatchString(result.Err.Error())) {
			// The report prints this error verbatim. Its stack depth depends on
			// earlier bundles in the same Node run (@babel/core raises
			// Error.stackTraceLimit), so take the text from a per-case Compile.
			verbatim++
			_, err := compile.Compile(row.project, compile.Options{OutDir: filepath.Join(row.root, "dist")})
			if err == nil {
				t.Fatalf("L08 %s API %s: batch rejected a bundle the per-case Compile accepts: %v", row.c.ID, api, result.Err)
			}
			errs[row.c.ID] = err
		}
	}
	prefix := table.ClassPrefix
	if prefix == "" {
		prefix = "FamilyL08"
	}
	t.Logf("L08 API %s %s: %d batch rows; %d compared with per-case Compile; %d verbatim errors from per-case Compile", api, prefix, len(batch), compared, verbatim)
	return errs
}

// l08StackFrames matches Node stack frame lines. Their count depends on
// Error.stackTraceLimit, which @babel/core raises in a long-lived process.
// Node 22 folds repeated frames into one "... collapsed N duplicate lines" line.
var l08StackFrames = regexp.MustCompile(`(?m)^ +(?:at |\.\.\. collapsed \d+ duplicate lines)[^\n]*\n`)

func l08CompileBundleName(api string, c l08Case) string {
	return fmt.Sprintf("familyL08C%03dV%s", c.SourceIndex, api[:2])
}

// l08WriteCompileRow writes one compile row's sources into root, plus the
// project and its shared Apex when base is set. Names carry the row's source
// index, so rows can share one batch project.
func l08WriteCompileRow(t *testing.T, root string, table l08Table, api string, c l08Case, base bool) {
	t.Helper()
	if base {
		l08WriteProject(t, root, api)
		if c.ApexBase == "dto" {
			_, dto, _ := l08Names(table, api)
			l08WriteClass(t, root, dto, l08Tokens(table, table.DTO, api, ""), api)
		} else {
			l08WriteApex(t, root, table, api)
		}
	}
	extra := fmt.Sprintf("FamilyL08Extra%03dV%s", c.SourceIndex, api[:2])
	if c.Apex != "" {
		l08WriteClass(t, root, extra, l08Tokens(table, c.Apex, api, extra), api)
	}
	if c.ApexBase == "" {
		l08WriteBundle(t, root, l08CompileBundleName(api, c), table, c, api, extra)
	}
}

// l08BatchControls selects the per-case Compile controls: the first row of
// each group, native outcome, row-owned Apex and native diagnostic presence.
// GLADE_L08_BATCH_CONTROL=all compares every row.
func l08BatchControls(table l08Table, api string) map[string]bool {
	controls := map[string]bool{}
	all := os.Getenv("GLADE_L08_BATCH_CONTROL") == "all"
	seen := map[string]bool{}
	for _, c := range table.Cases {
		if c.Kind != "compile" || c.ApexBase != "" {
			continue
		}
		key := fmt.Sprintf("%s|%s|%t|%t", c.Group, c.Expected[api], c.Apex != "", len(c.DiagnosticExpected[api]) > 0)
		if all || !seen[key] {
			controls[c.ID] = true
		}
		seen[key] = true
	}
	return controls
}

// l08AssertBatchControl recompiles one row with the per-case Compile call
// the test used before batching, in the row's own project. The outcome, the
// error text apart from stack frames, the reported diagnostics and every
// output byte must equal the batch result once the two temporary roots are
// mapped onto each other.
func l08AssertBatchControl(t *testing.T, c l08Case, api string, p project.Project, root, batchRoot string, result compile.BundleResult) {
	t.Helper()
	id := c.ID + " API " + api
	singleOut := filepath.Join(root, "dist")
	single, singleErr := compile.Compile(p, compile.Options{OutDir: singleOut})
	// Output directories first: the batch one is under the batch root.
	normalize := strings.NewReplacer(singleOut+string(filepath.Separator), result.Manifest.OutDir+string(filepath.Separator), singleOut+`"`, result.Manifest.OutDir+`"`, root+string(filepath.Separator), batchRoot+string(filepath.Separator)).Replace
	if (singleErr == nil) != (result.Err == nil) {
		t.Fatalf("L08 control %s: batch error %v; single error %v", id, result.Err, singleErr)
	}
	if singleErr != nil {
		// Stack frames are the only allowed difference: their count is not
		// part of any reported diagnostic, and verbatim rows use Compile.
		if got, want := l08StackFrames.ReplaceAllString(result.Err.Error(), ""), l08StackFrames.ReplaceAllString(normalize(singleErr.Error()), ""); got != want {
			t.Errorf("L08 control %s: error text differs:\nbatch  %q\nsingle %q", id, got, want)
		}
		if got, want := l08CompileDiagnostics(result.Err), l08CompileDiagnostics(singleErr); got != want {
			t.Errorf("L08 control %s: diagnostics differ: batch %s single %s", id, got, want)
		}
		return
	}
	batchModules, err := json.Marshal(result.Manifest.Modules)
	if err != nil {
		t.Fatal(err)
	}
	singleModules, err := json.Marshal(single.Modules)
	if err != nil {
		t.Fatal(err)
	}
	if string(batchModules) != normalize(string(singleModules)) {
		t.Errorf("L08 control %s: manifest differs: batch %s single %s", id, batchModules, singleModules)
	}
	batchFiles, singleFiles := l08OutputBytes(t, result.Manifest.OutDir), l08OutputBytes(t, singleOut)
	if len(batchFiles) == 0 || len(batchFiles) != len(singleFiles) {
		t.Errorf("L08 control %s: batch wrote %d files, single %d", id, len(batchFiles), len(singleFiles))
	}
	for name, data := range singleFiles {
		if batchFiles[name] != normalize(data) {
			t.Errorf("L08 control %s: output %s differs", id, name)
		}
	}
}

func l08OutputBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func l08ObserveDOM(t *testing.T, table l08Table, api string, cases []l08Case, repo string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	root := t.TempDir()
	l08WriteProject(t, root, api)
	l08WriteApex(t, root, table, api)
	var children strings.Builder
	ids := make([]string, len(cases))
	for index, c := range cases {
		name := fmt.Sprintf("familyL08R%03dV%s", index, api[:2])
		l08WriteBundle(t, root, name, table, c, api, "")
		tag := fmt.Sprintf("c-family-l08-r%03d-v%s", index, api[:2])
		fmt.Fprintf(&children, `<section data-case="%s"><%s></%s></section>`, c.ID, tag, tag)
		ids[index] = c.ID
	}
	page := "familyL08Runtime" + api[:2]
	l08WriteBundle(t, root, page, table, l08Case{JS: "import {LightningElement} from 'lwc'; export default class FamilyBridge extends LightningElement {}", Template: "<template>" + children.String() + "</template>"}, api, "")
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, err
	}
	index, err := l08Index(p)
	if err != nil {
		return nil, nil, err
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		return nil, nil, err
	}
	org := storage.NewOrgState()
	// The oracle's query-exception row uses a known, empty Account table, not
	// an org without standard schema. No record data is required.
	definition, ok := storage.StandardObjectDefinition("Account")
	if !ok {
		return nil, nil, fmt.Errorf("L08 Account fixture schema missing")
	}
	org.Objects["Account"] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(index)
	defer handler.ResetLightningCache()
	if handler.runtimeErr != nil {
		return nil, nil, handler.runtimeErr
	}
	if err := handler.ensureLightningLocked(); err != nil {
		return nil, nil, err
	}
	entry, ok := handler.lightning.compiled.Modules["c:"+page]
	if !ok {
		return nil, nil, fmt.Errorf("L08 runtime page missing from compile output")
	}
	bootstrap := lwcbrowser.BootstrapHTML(handler.lightning.pageCfg)
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Resolve appendChild after root creation installs synthetic lifecycle hooks.
			fmt.Fprintf(w, `<!doctype html>%s<main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Page from "c/%s";const el=createElement("%s",{is:Page});document.querySelector("main").appendChild(el);</script>`, bootstrap, page, entry.Tag)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer host.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	config, err := json.Marshal(map[string]any{"url": host.URL, "ids": ids, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/server/testdata/l08_browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("L08 local browser: %w: %s", err, l08OneLine(stderr.String()))
	}
	var observation struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observation); err != nil {
		return nil, nil, err
	}
	return observation.Values, observation.Errors, nil
}

// Serialize observations in the native TSV's sorted, ASCII JSON format. The
// JSON tree remains unchanged: no missing/null equivalence or string folding.
func l08DOMText(raw json.RawMessage) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	var text strings.Builder
	text.WriteString("DOM|")
	for _, r := range strings.TrimSuffix(encoded.String(), "\n") {
		if r < 0x7f {
			text.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&text, `\u%04x`, r)
		} else {
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&text, `\u%04x\u%04x`, hi, lo)
		}
	}
	return text.String(), nil
}

func l08OneLine(value string) string {
	return strings.NewReplacer("\t", "\\t", "\r", "\\r", "\n", "\\n").Replace(value)
}
