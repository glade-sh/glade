package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native anonymous Apex sees an ordinary custom field only through an
// assigned permission set or an Admin.profile grant, and default-mode
// Database.insert/update(list,false) fails only the denied records. See
// testdata/conformance/admin_field_grant_sources/ORG-RUN.txt.
func TestAdminFieldGrantSourcesNativeConformance(t *testing.T) {
	const family = "testdata/conformance/admin_field_grant_sources"
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 1+3*5 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(family, "anon", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	probe, staticProp, staticSoql, staticInsert := read("probe.apex"), read("static_prop.apex"), read("static_soql.apex"), read("static_insert.apex")
	okCount := regexp.MustCompile(`ok:\d+`)
	for _, row := range rows[1:] {
		api, method := row[0], row[1]
		row := row
		runConformanceSubtestParallel(t, api+"/"+method, func(t *testing.T) {
			var want map[string]any
			if json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("bad row %s", method)
			}
			root := t.TempDir()
			project := filepath.Join(family, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if strings.Contains(rel, "permissionsets") && method == "anon_noGrant" {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			switch method {
			case "anon_permsetAssigned":
				writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [CapFls]\n")
			case "anon_adminProfileGrant":
				writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Admin.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom><fieldPermissions><editable>true</editable><field>CapObj__c.Optional__c</field><readable>true</readable></fieldPermissions></Profile>`)
			}
			index := loadTestIndex(t, root)
			if method == "mixedLists_noGrant" {
				run := Run(index, Options{SelectedClasses: []string{"MixProbe62"}, SelectedMethod: "mixed", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing runner observation %+v", run.Summary())
				}
				_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED|mixed|")
				var got map[string]any
				if !ok || json.Unmarshal([]byte(payload), &got) != nil {
					t.Fatalf("invalid transport: %s", run.Suites[0].Cases[0].Problem.Message)
				}
				if !reflect.DeepEqual(got, want) {
					gotJSON, _ := json.Marshal(got)
					t.Fatalf("native=%s\nGlade =%s", row[2], gotJSON)
				}
				return
			}
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			got := map[string]any{}
			program, err := vm.CompileAnonymousWithOptions(probe, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatalf("anonymous compile: %v", err)
			}
			result, err := runner.newMachine().Execute(program)
			if err != nil {
				t.Fatalf("anonymous run: %v", err)
			}
			for _, line := range result.Debug {
				if _, payload, ok := strings.Cut(line, "OBSERVED|anonymous|"); ok {
					if json.Unmarshal([]byte(okCount.ReplaceAllString(payload, "ok")), &got) != nil {
						t.Fatalf("bad payload %s", payload)
					}
				}
			}
			for key, src := range map[string]string{"staticProp": staticProp, "staticSoql": staticSoql, "staticInsert": staticInsert} {
				got[key] = "compiled"
				visible := sema.WithoutAnonymousHiddenFields(index, runner.newMachine().AnonymousPolicyHiddenFields(api, src))
				if analysis := sema.AnalyzeAnonymous(visible, src, api); analysis.HasErrors() {
					got[key] = "compileError"
				}
			}
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("native=%s\nGlade =%s", row[2], gotJSON)
			}
		})
	}
}
