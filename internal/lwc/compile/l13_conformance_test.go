package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
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

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

type l13Case struct {
	ID                 string              `json:"id"`
	Group              string              `json:"group"`
	Kind               string              `json:"kind"`
	Basis              string              `json:"basis"`
	JS                 string              `json:"js"`
	Template           string              `json:"template"`
	MetaFragment       string              `json:"meta_fragment"`
	DependencyProject  bool                `json:"dependency_project"`
	Expected           map[string]string   `json:"expected"`
	DiagnosticExpected map[string][]string `json:"diagnostic_expected"`
	Carried            map[string]string   `json:"carried"`
}

type l13CompiledRow struct {
	Case        l13Case
	Manifest    compile.Manifest
	Project     project.Project
	Err         error
	Diagnostics []string
	// Messages are the compiler's original diagnostic messages, in order.
	Messages []string
}

// TestL13SalesforceConformance replays the owned inputs and exact native.tsv
// text exported from the API 59/67 import capture. The original capture
// rows are preserved; fifteen review controls add existing custom-object
// fields, a decoded import, sibling dependency imports, inherited-key errors and label resolver neighbours.
// Supplemental native evidence and source inputs are exported with those rows.
// Each API has 218 compiler/metadata and 84 native DOM answers. Nine explicit
// local Experience Builder boundaries are counted separately, never as native.
// GLADE_L13_CAPTURE=1 records accepted-tree behavior without failing mismatches;
// GLADE_L13_REPORT selects the TSV destination. Neither changes the case table.
// CI uses only this export; it never calls Salesforce or imports capture tools.
func TestL13SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L13_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l13_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l13Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups, kinds := map[string]bool{}, map[string]bool{}, map[string]int{}
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.JS == "" || c.Template == "" {
			t.Fatalf("invalid or duplicate L13 case %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		kinds[c.Kind]++
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && !strings.HasPrefix(want, "DOM|")) ||
				(c.Kind == "boundary" && want != "BOUNDARY_ERROR|EXPERIENCE_BUILDER_CONTEXT_REQUIRED") {
				t.Fatalf("missing or invalid L13 answer: %s API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				diagnostics, present := c.DiagnosticExpected[api]
				if !present || (want == "COMPILE_ERROR") != (len(diagnostics) > 0) {
					t.Fatalf("missing native rejection diagnostic: %s API %s", c.ID, api)
				}
			}
			if reason := c.Carried[api]; reason != "" && !strings.HasPrefix(reason, "owner L23:") {
				t.Fatalf("invalid host-context carry for %s API %s", c.ID, api)
			}
		}
		if (c.Kind == "boundary" && c.Basis != "local-boundary") || (c.Kind != "boundary" && c.Basis != "org") {
			t.Fatalf("incorrect native/local provenance for %s", c.ID)
		}
		if c.Expected["59.0"] != c.Expected["67.0"] {
			t.Fatalf("uncaptured version difference for %s", c.ID)
		}
	}
	if len(cases) != 311 || kinds["compile"] != 218 || kinds["runtime"] != 84 || kinds["boundary"] != 9 {
		t.Fatalf("L13 requires 218 compile + 84 DOM + 9 boundary rows, got %v", kinds)
	}
	for _, group := range []string{"schema", "label", "resourceUrl", "contentAssetUrl", "user", "userPermission", "customPermission", "i18n", "i18n-values", "formFactor", "community", "site"} {
		if !groups[group] {
			t.Fatalf("missing L13 group %s", group)
		}
	}
	// Tooling failures cannot count as native compiler rejections.
	dependencyRoot, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"third_party/lwc/node_modules/@lwc/compiler/package.json", "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js", "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"} {
		if _, err := os.Stat(filepath.Join(dependencyRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	observer := filepath.Join(t.TempDir(), "l13_diagnostics.cjs")
	l13Write(t, observer, []byte(l13DiagnosticObserverJS))
	t.Setenv("NODE_OPTIONS", os.Getenv("NODE_OPTIONS")+" --require "+l13JSON(t, observer))
	t.Run("namespace-compatibility", l13AssertNamespaceCompatibility)
	var report strings.Builder
	report.WriteString("API\tID\tKIND\tBASIS\tSTATUS\tACTUAL\tEXPECTED\tREASON\n")
	matches, total, nativeMatches, nativeTotal, boundaryMatches, boundaryTotal := 0, 0, 0, 0, 0, 0
	carried := 0
	for _, api := range versions {
		apiMatches, apiNativeMatches, apiBoundaryMatches := matches, nativeMatches, boundaryMatches
		apiTotal, apiNativeTotal, apiBoundaryTotal := total, nativeTotal, boundaryTotal
		rows := l13CompileRows(t, cases, api)
		dom, errors := l13ObserveDOM(t, rows, dependencyRoot)
		for _, row := range rows {
			c := row.Case
			got, reason := "COMPILE_OK", "L13: local compiler accepted native-rejected source"
			if row.Err != nil {
				got, reason = "COMPILE_ERROR", "L13: "+row.Err.Error()
			}
			if c.Kind != "compile" && row.Err == nil {
				if value, ok := dom[c.ID]; ok {
					got = value
					reason = "L13: local DOM/import-context output differs"
					if c.Kind == "boundary" {
						reason = "L13: Experience Builder import did not produce the explicit local context boundary"
					}
				} else {
					got, reason = "DOM_ERROR", "L13: local DOM observation missing: "+errors[c.ID]
				}
			}
			want := c.Expected[api]
			if c.Kind == "compile" {
				got += "|DIAGNOSTICS|" + l13JSON(t, row.Diagnostics)
				want += "|DIAGNOSTICS|" + l13JSON(t, c.DiagnosticExpected[api])
				if reason == "L13: local compiler accepted native-rejected source" {
					reason = "L13: local compiler outcome or exact diagnostic differs from native"
				}
			}
			// Exact Go string equality preserves case, length and raw JSON null.
			matched := got == want
			status := "MISMATCH"
			if matched {
				status, reason = "MATCH", ""
				matches++
			} else if c.Carried[api] != "" {
				status, reason = "CARRIED", c.Carried[api]
				carried++
				t.Logf("%s API %s CARRIED expected <%s> actual <%s>: %s", c.ID, api, want, got, reason)
			}
			total++
			if c.Basis == "org" {
				nativeTotal++
				if matched {
					nativeMatches++
				}
			} else {
				boundaryTotal++
				if matched {
					boundaryMatches++
				}
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, c.Basis, status, l13Cell(got), l13Cell(want), l13Cell(reason))
			if !capture && !matched && status != "CARRIED" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l13Cell(reason))
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\nAPI_NATIVE_TOTAL\t%s\t%d/%d\nAPI_LOCAL_BOUNDARY_TOTAL\t%s\t%d/%d\n", api, matches-apiMatches, total-apiTotal, api, nativeMatches-apiNativeMatches, nativeTotal-apiNativeTotal, api, boundaryMatches-apiBoundaryMatches, boundaryTotal-apiBoundaryTotal)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nNATIVE_TOTAL\t%d/%d\nLOCAL_BOUNDARY_TOTAL\t%d/%d\n", matches, total, nativeMatches, nativeTotal, boundaryMatches, boundaryTotal)
	fmt.Fprintf(&report, "CARRIED\t%d/%d\n", carried, total)
	t.Logf("L13 matches %d/%d; native %d/%d; local boundaries %d/%d", matches, total, nativeMatches, nativeTotal, boundaryMatches, boundaryTotal)
	if path := os.Getenv("GLADE_L13_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// These checks preserve the shared resolver's accepted local namespace
// contract. They are not native answers and do not add to the match count:
// assigned org B has no package namespace to capture BasePkg -> stagepkg.
func l13AssertNamespaceCompatibility(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range []struct {
			Name, SourceNamespace, Permission string
			Dependency                        bool
		}{
			{Name: "local-label", SourceNamespace: "stagepkg"},
			{Name: "remapped-dependency", SourceNamespace: "BasePkg", Permission: "BasePkg__Permission", Dependency: true},
			{Name: "unqualified-dependency", SourceNamespace: "BasePkg", Permission: "Permission", Dependency: true},
		} {
			t.Run(api+"/"+tc.Name, func(t *testing.T) {
				workspace := t.TempDir()
				root := filepath.Join(workspace, "consumer")
				metadataRoot := root
				manifest := func(namespace string) []byte {
					return []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"` + namespace + `","sourceApiVersion":"` + api + `"}`)
				}
				l13Write(t, filepath.Join(root, "sfdx-project.json"), manifest("stagepkg"))
				if tc.Dependency {
					metadataRoot = filepath.Join(workspace, "dependency")
					l13Write(t, filepath.Join(metadataRoot, "sfdx-project.json"), manifest(tc.SourceNamespace))
					l13Write(t, filepath.Join(root, "glade.yml"), []byte("project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n  managedPackageDependencies: [\"stagepkg:../dependency\"]\n"))
				}
				base := filepath.Join(metadataRoot, "force-app", "main", "default")
				l13Write(t, filepath.Join(base, "labels", "CustomLabels.labels-meta.xml"), []byte(`<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata"><labels><fullName>`+tc.SourceNamespace+`__Billing_Error</fullName><language>en_US</language><protected>false</protected><shortDescription>Billing Error</shortDescription><value>Bad bill</value></labels></CustomLabels>`))
				if tc.Permission != "" {
					l13Write(t, filepath.Join(base, "customPermissions", tc.Permission+".customPermission-meta.xml"), []byte(`<CustomPermission xmlns="http://soap.sforce.com/2006/04/metadata"><label>Permission</label></CustomPermission>`))
				}
				aliases := []string{"c.Billing_Error", "stagepkg.Billing_Error", "c.stagepkg__Billing_Error", "stagepkg.stagepkg__Billing_Error"}
				js := "import { LightningElement } from 'lwc';\n"
				for index, alias := range aliases {
					js += fmt.Sprintf("import label%d from %s;\n", index, l13JSON(t, "@salesforce/label/"+alias))
				}
				if tc.Permission != "" {
					js += "import permission from '@salesforce/customPermission/stagepkg__Permission';\n"
					if tc.Permission == "Permission" {
						js += "import localPermission from '@salesforce/customPermission/Permission';\n"
					}
				}
				js += "export default class FamilyImports extends LightningElement { valueResult = label0; }\n"
				bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "familyImports")
				l13Write(t, filepath.Join(bundle, "familyImports.js"), []byte(js))
				l13Write(t, filepath.Join(bundle, "familyImports.html"), []byte("<template><span>{valueResult}</span></template>"))
				l13Write(t, filepath.Join(bundle, "familyImports.js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`))
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if tc.Dependency {
					if len(p.LabelFiles) != 0 || len(p.CustomPermissionFiles) != 0 || len(p.ManagedPackageDependencies) != 1 {
						t.Fatal("namespace compatibility requires metadata only in the loaded dependency")
					}
					dep := p.ManagedPackageDependencies[0]
					if dep.Status != "loaded" || dep.Project == nil || dep.Project.Namespace != "stagepkg" || len(dep.Project.NamespaceRemaps) != 1 {
						t.Fatalf("shared project loader did not apply namespace remapping: %#v", dep)
					}
					if tc.Permission == "BasePkg__Permission" && namespaceremap.ApplyMetadataName(dep.Project.NamespaceRemaps, tc.Permission) != "stagepkg__Permission" {
						t.Fatal("shared metadata-name resolver did not preserve its remapping contract")
					}
				}
				registry, err := resource.LoadProjectWithDependencies(p)
				if err != nil {
					t.Fatal(err)
				}
				for _, alias := range aliases {
					namespace, name, _ := strings.Cut(alias, ".")
					if value, status := resource.ResolveLabel(registry, p.Namespace, namespace, name); status != resource.LabelLookupResolved || value != "Bad bill" {
						t.Fatalf("shared resolver did not preserve label %s: %q %s", alias, value, status)
					}
				}
				result, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
				if err != nil {
					t.Fatalf("compiler rejected accepted shared namespace resolution: %v", err)
				}
				if _, exists := result.Modules["c:familyImports"]; !exists {
					t.Fatal("namespace compatibility bundle was not compiled")
				}
			})
		}
	}
	t.Log("namespace compatibility preserves the existing local resolver contract; no native package-namespace acceptance claimed")
}

// l13CompileRows compiles every case of one API with compile.CompileBatch.
// Cases sharing a project shape (plain, or consumer plus dependency package)
// load as one project with the same metadata fixtures as a single-case project.
// Each case keeps its own familyImports bundle in its own directory and output
// directory; a failing bundle does not stop the others. Each row's project is
// the loaded project restricted to that row's bundle, as a single-case load.
func l13CompileRows(t *testing.T, cases []l13Case, api string) []l13CompiledRow {
	t.Helper()
	rows := make([]l13CompiledRow, len(cases))
	controls := l13BatchControls(cases, api)
	for _, dependency := range []bool{false, true} {
		var indices []int
		for i, c := range cases {
			if c.DependencyProject == dependency {
				indices = append(indices, i)
			}
		}
		if len(indices) == 0 {
			continue
		}
		root, base := l13WriteProject(t, api, dependency)
		bundles := make(map[int]string, len(indices))
		for _, i := range indices {
			bundles[i] = l13WriteBundle(t, filepath.Join(base, "cases", fmt.Sprintf("%04d", i)), cases[i], api)
		}
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.LWCMetaFiles) != len(indices) {
			t.Fatalf("L13 batch project loaded %d of %d bundles", len(p.LWCMetaFiles), len(indices))
		}
		if dependency {
			if len(p.ObjectFiles) != 0 || len(p.CustomPermissionFiles) != 0 || len(p.ManagedPackageDependencies) != 1 {
				t.Fatalf("L13 dependency metadata must be outside the consumer: %#v", p.ManagedPackageDependencies)
			}
			dep := p.ManagedPackageDependencies[0]
			if dep.Status != "loaded" || dep.Project == nil || len(dep.Project.ObjectFiles) != 1 || len(dep.Project.CustomPermissionFiles) != 1 {
				t.Fatalf("L13 requires the shared project loader's loaded dependency: %#v", dep)
			}
		}
		results, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
		if err != nil {
			t.Fatalf("L13 compiler environment failure: %v", err)
		}
		if len(results) != len(indices) {
			t.Fatalf("L13 batch returned %d of %d bundles", len(results), len(indices))
		}
		for _, i := range indices {
			key, err := filepath.Rel(root, bundles[i])
			if err != nil {
				t.Fatal(err)
			}
			result, ok := results[filepath.ToSlash(key)]
			if !ok {
				t.Fatalf("L13 batch result missing for %s", cases[i].ID)
			}
			messages := make([]string, len(result.Diagnostics))
			for j, diagnostic := range result.Diagnostics {
				messages[j] = diagnostic.Message
			}
			rows[i] = l13Row(t, cases[i], l13CaseProject(p, bundles[i]), result.Manifest, result.Err, messages)
			if controls[i] {
				l13AssertBatchControl(t, rows[i], api)
			}
		}
	}
	t.Logf("L13 API %s: %d batch rows; %d compared with per-case Compile", api, len(rows), len(controls))
	return rows
}

// l13WriteProject writes the native deployment support inputs plus a
// field-free local declaration of the existing FamilyBaseline__c control
// object. These supply metadata only. It returns the project root and the
// package directory that holds the case bundles.
func l13WriteProject(t *testing.T, api string, dependency bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	l13CopyFixtures(t, "testdata/l13_fixtures", filepath.Join(root, "force-app", "main", "default"))
	if !dependency {
		l13Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`))
		return root, filepath.Join(root, "force-app")
	}
	workspace := root
	root = filepath.Join(workspace, "force-app", "consumer")
	dependencyRoot := filepath.Join(workspace, "force-app", "dependency")
	l13CopyFixtures(t, "testdata/l13_dependency_fixtures", filepath.Join(dependencyRoot, "main", "default"))
	for _, pkg := range []struct{ Root, Name, Dependencies string }{
		{dependencyRoot, "L13Dependency", "[]"},
		{root, "L13Consumer", `[{"package":"L13Dependency"}]`},
	} {
		l13Write(t, filepath.Join(pkg.Root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"main","default":true,"package":"`+pkg.Name+`","dependencies":`+pkg.Dependencies+`}],"namespace":"","sourceApiVersion":"`+api+`"}`))
	}
	return root, filepath.Join(root, "main")
}

func l13WriteBundle(t *testing.T, dir string, c l13Case, api string) string {
	t.Helper()
	name := "familyImports"
	bundle := filepath.Join(dir, "lwc", name)
	l13Write(t, filepath.Join(bundle, name+".js"), []byte(c.JS))
	l13Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
	fragment := c.MetaFragment
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	l13Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
	return bundle
}

// l13CaseProject restricts a loaded project to one bundle without changing
// any source path or any non-LWC metadata.
func l13CaseProject(p project.Project, bundle string) project.Project {
	within := func(paths []string) []string {
		var files []string
		for _, path := range paths {
			rel, err := filepath.Rel(bundle, path)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				files = append(files, path)
			}
		}
		return files
	}
	p.LWCFiles = within(p.LWCFiles)
	p.LWCHTMLFiles = within(p.LWCHTMLFiles)
	p.LWCCSSFiles = within(p.LWCCSSFiles)
	p.LWCMetaFiles = within(p.LWCMetaFiles)
	return p
}

// l13Row normalizes the compiler's original diagnostic messages. Batch and
// single compiles pass the same Error objects: CompileBatch returns their
// messages and the single-compile preload prints them before Node's stack.
func l13Row(t *testing.T, c l13Case, p project.Project, manifest compile.Manifest, compileErr error, messages []string) l13CompiledRow {
	t.Helper()
	if compileErr != nil {
		for _, infrastructure := range []string{"Cannot find module", "ERR_MODULE_NOT_FOUND", "decode compile result:", "could not find glade", "signal: killed"} {
			if strings.Contains(compileErr.Error(), infrastructure) {
				t.Fatalf("L13 compiler environment failure: %v", compileErr)
			}
		}
	}
	diagnostics := []string{}
	if compileErr != nil {
		seen := map[string]bool{}
		for _, message := range messages {
			message = l13DiagnosticPosition.ReplaceAllString(message, "")
			message = strings.ReplaceAll(message, "familyImports.js", "{bundle}.js")
			if !seen[message] {
				diagnostics = append(diagnostics, message)
				seen[message] = true
			}
		}
		if len(diagnostics) == 0 {
			t.Fatalf("L13 compiler rejection has no observed diagnostic: %v", compileErr)
		}
	}
	sort.Strings(diagnostics)
	return l13CompiledRow{Case: c, Project: p, Manifest: manifest, Err: compileErr, Diagnostics: diagnostics, Messages: messages}
}

// l13SingleDiagnosticMessages reads the preload's markers from a single
// Compile error, in the order the compiler raised them.
func l13SingleDiagnosticMessages(t *testing.T, compileErr error) []string {
	t.Helper()
	messages := []string{}
	if compileErr == nil {
		return messages
	}
	for _, line := range strings.Split(compileErr.Error(), "\n") {
		const marker = "GLADE_L13_DIAGNOSTIC|"
		if !strings.HasPrefix(line, marker) {
			continue
		}
		var message string
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, marker)), &message); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
	return messages
}

// l13BatchControls selects the per-case Compile controls: the first case of
// each kind, native outcome, project shape and import family (schema, label,
// other). GLADE_L13_BATCH_CONTROL=all compares every case.
func l13BatchControls(cases []l13Case, api string) map[int]bool {
	controls := map[int]bool{}
	all := os.Getenv("GLADE_L13_BATCH_CONTROL") == "all"
	seen := map[string]bool{}
	for i, c := range cases {
		family := "other"
		if strings.Contains(c.Group, "schema") {
			family = "schema"
		} else if strings.Contains(c.Group, "label") {
			family = "label"
		}
		outcome, _, _ := strings.Cut(c.Expected[api], "|")
		key := fmt.Sprintf("%s|%s|%t|%s", c.Kind, outcome, c.DependencyProject, family)
		if all || !seen[key] {
			controls[i] = true
		}
		seen[key] = true
	}
	return controls
}

// l13AssertBatchControl recompiles one batch row with the per-case Compile
// path into the same source and output paths. Hydration then sees only this
// bundle's Salesforce import references. Output bytes, manifest, outcome and
// the compiler's original diagnostic messages must be identical.
func l13AssertBatchControl(t *testing.T, row l13CompiledRow, api string) {
	t.Helper()
	id := row.Case.ID + " API " + api
	bundle, err := filepath.Rel(row.Project.Root, filepath.Dir(row.Project.LWCMetaFiles[0]))
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(row.Project.Root, "dist", bundle)
	if row.Manifest.OutDir != outDir {
		t.Fatalf("L13 control %s: unexpected batch output %s", id, row.Manifest.OutDir)
	}
	// Both paths write manifest.json only on success, with the same bytes.
	batchFiles := l13OutputBytes(t, outDir)
	if err := os.RemoveAll(outDir); err != nil {
		t.Fatal(err)
	}
	single, singleErr := compile.Compile(row.Project, compile.Options{OutDir: outDir, Namespace: "c"})
	if (singleErr == nil) != (row.Err == nil) {
		t.Fatalf("L13 control %s: batch error %v; single error %v", id, row.Err, singleErr)
	}
	if l13JSON(t, batchFiles) != l13JSON(t, l13OutputBytes(t, outDir)) {
		t.Errorf("L13 control %s: output bytes differ", id)
	}
	if singleErr == nil && l13JSON(t, row.Manifest) != l13JSON(t, single) {
		t.Errorf("L13 control %s: manifest differs: batch %s single %s", id, l13JSON(t, row.Manifest), l13JSON(t, single))
	}
	singleMessages := l13SingleDiagnosticMessages(t, singleErr)
	if l13JSON(t, row.Messages) != l13JSON(t, singleMessages) {
		t.Errorf("L13 control %s: diagnostic messages differ: batch %s single %s", id, l13JSON(t, row.Messages), l13JSON(t, singleMessages))
	}
}

func l13OutputBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return files
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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

func l13CopyFixtures(t *testing.T, source, dest string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		l13Write(t, filepath.Join(dest, rel), data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var l13DiagnosticPosition = regexp.MustCompile(`\[Line: \d+, Col: \d+\]\s*`)

// Observe thrown diagnostics before Node prints stack paths. The compiler call,
// original stderr and exit code remain unchanged; no answers enter this preload.
const l13DiagnosticObserverJS = `const original = console.error;
console.error = function (...args) {
  for (const error of args) {
    if (error instanceof Error) {
      for (const diagnostic of Array.isArray(error.errors) ? error.errors : [error]) {
        process.stderr.write('GLADE_L13_DIAGNOSTIC|' + JSON.stringify(diagnostic.message) + '\n');
      }
    }
  }
  return original.apply(console, args);
};
`

func l13Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func l13ObserveDOM(t *testing.T, rows []l13CompiledRow, dependencyRoot string) (map[string]string, map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for route, rel := range map[string]string{
		"/engine.js": "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js",
		"/shadow.js": "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js",
	} {
		file := filepath.Join(dependencyRoot, filepath.FromSlash(rel))
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			http.ServeFile(w, r, file)
		})
	}
	// Shims and runtime assets use the real HTTP handlers and metadata registry.
	// Each navigation has a fresh module/context environment and one owned row.
	specs := []map[string]string{}
	products := map[string]http.Handler{}
	for _, row := range rows {
		if row.Case.Kind == "compile" || row.Err != nil {
			continue
		}
		prefix := "/rows/" + row.Case.ID + "/"
		imports := lwcbrowser.SalesforceImportMap()
		imports["lwc"], imports["@lwc/synthetic-shadow"] = "/engine.js", "/shadow.js"
		for _, module := range row.Manifest.Modules {
			rel, err := filepath.Rel(row.Manifest.OutDir, module.File)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("L13 module escapes output: %s", module.File)
			}
			imports[module.ModuleKey] = prefix + "modules/" + filepath.ToSlash(rel)
		}
		entry, ok := row.Manifest.Modules["c:familyImports"]
		if !ok {
			t.Fatalf("L13 compiled fixture missing for %s", row.Case.ID)
		}
		org := storage.NewOrgState()
		storage.EnsureDeterministicPlatformData(&org)
		registry, err := resource.LoadProject(row.Project)
		if err != nil {
			t.Fatal(err)
		}
		org.Metadata = registry
		product := server.NewWithSource(&org, server.SourceMetadata{Project: row.Project})
		products[row.Case.ID] = product
		// Requests made by each page's imported modules are routed to its own
		// source metadata via a cookie, without replacing product module code.
		mux.Handle(prefix+"modules/", http.StripPrefix(prefix+"modules/", http.FileServer(http.Dir(row.Manifest.OutDir))))
		mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != prefix {
				http.NotFound(w, r)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "l13-row", Value: row.Case.ID, Path: "/"})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}};window.__l13Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><script type="importmap">%s</script><main></main><script type="module">try {await import("@lwc/synthetic-shadow");const {createElement}=await import("lwc");const {default:Ctor}=await import("c/familyImports");document.querySelector("main").appendChild(createElement(%s,{is:Ctor}));} catch(e) {window.__l13Error={name:e.name,message:e.message,code:e.code};}</script>`, l13JSON(t, map[string]any{"imports": imports}), l13JSON(t, entry.Tag))
		})
		specs = append(specs, map[string]string{"id": row.Case.ID, "url": prefix})
	}
	if len(specs) == 0 {
		return map[string]string{}, map[string]string{}
	}
	mux.HandleFunc("/lightning/", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("l13-row")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		handler, ok := products[cookie.Value]
		if !ok {
			http.NotFound(w, r)
			return
		}
		// Dispatch retains the original route for the product handler.
		handler.ServeHTTP(w, r)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	for _, spec := range specs {
		spec["url"] = local.URL + spec["url"]
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l13_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = strings.NewReader(l13JSON(t, map[string]any{"rows": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")}))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("L13 local browser environment: %v: %s", err, stderr.String())
	}
	var observed struct {
		Values map[string]string `json:"values"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatal(err)
	}
	// Canonicalize JSON only to match native TSV serialization. No field is
	// removed, case-folded, coerced, truncated, or supplied from the oracle.
	for id, value := range observed.Values {
		if strings.HasPrefix(value, "DOM|") {
			observed.Values[id] = "DOM|" + l13CanonicalJSON(t, strings.TrimPrefix(value, "DOM|"))
		}
	}
	return observed.Values, observed.Errors
}

func l13JSON(t *testing.T, value any) string {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func l13CanonicalJSON(t *testing.T, text string) string {
	t.Helper()
	var value any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for _, r := range l13JSON(t, value) {
		if r < 0x7f {
			out.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&out, `\u%04x`, r)
		} else {
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&out, `\u%04x\u%04x`, hi, lo)
		}
	}
	return out.String()
}

func l13Cell(value string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(value)
}
