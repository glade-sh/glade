package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

const standardChildFieldFixture = "../../testdata/conformance/standard_platform_child_fields"

func nativeStandardChildFieldRows(t *testing.T, api, route string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(standardChildFieldFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, row := range table[1:] {
		if row[0] == api && row[1] == route {
			want = append(want, row[2])
		}
	}
	count := 15
	if api == "62.0" {
		count = 8
	}
	if len(want) != count {
		t.Fatalf("native %s/%s has %d rows; want %d standard-parent edges", api, route, len(want), count)
	}
	sort.Strings(want)
	return want
}

// Walk all named child edges, then compare the selected fields' complete
// overlay facets with native relationship-token observations. The map route
// checks Glade's projected field against those same facets: a clean native org
// can omit Order.OpportunityId from fields.getMap() even though the relationship
// token describes it. This test does not assert native field-map visibility.
func TestStandardParentPlatformChildFieldDescribeProjectRoutes(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(standardChildFieldFixture, "project", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	walk := read("walk.apex")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			apiWalk := walk
			if api == "62.0" {
				// Native rows expose the staged-sharing object only at later
				// APIs. The local catalog still walks all edges, while facet
				// comparisons use only fields observed at this source API.
				apiWalk = strings.ReplaceAll(apiWalk, ", 'ExtlRecShrStagedRecordAccounts'", "")
				apiWalk = strings.ReplaceAll(apiWalk, ", 'ParentRecords'", "")
				apiWalk = strings.ReplaceAll(apiWalk, "{'ParentRecords'}", "{}")
			}
			for _, tokenRoute := range []string{"relationship-token", "child-field-map"} {
				t.Run(tokenRoute, func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					body := apiWalk
					if tokenRoute == "child-field-map" {
						body = strings.Replace(body,
							"Schema.DescribeFieldResult d = cr.getField().getDescribe();",
							`Schema.DescribeFieldResult d = cr.getChildSObject().getDescribe().fields.getMap().get(String.valueOf(cr.getField()).toLowerCase()).getDescribe();`, 1)
					}
					for _, class := range []string{"StandardChildFieldProbe", "StandardChildFieldProbeTest"} {
						source := read("force-app/main/default/classes/" + class + ".cls")
						if class == "StandardChildFieldProbe" {
							source = strings.Replace(source, walk, body, 1)
						}
						path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					index := loadTestIndex(t, root)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					check := func(t *testing.T, route, message string) {
						t.Helper()
						_, payload, found := strings.Cut(message, "STANDARD_CHILD_ROWS|")
						var got []string
						if !found || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid observation: %s", message)
						}
						want := nativeStandardChildFieldRows(t, api, route)
						if !reflect.DeepEqual(want, got) {
							t.Fatalf("native %v; Glade (including walk failures) %v", want, got)
						}
					}
					runAnonymous := func(t *testing.T, org storage.OrgState, route, source string) {
						t.Helper()
						org.APIVersion = api
						base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
						if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
							t.Fatalf("anonymous compilation: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := base.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						check(t, route, strings.Join(result.Debug, "\n"))
					}
					t.Run("anonymous", func(t *testing.T) {
						source := "List<String> out;\n" + strings.Replace(body, "List<String> out =", "out =", 1) + "System.debug(LoggingLevel.ERROR, 'STANDARD_CHILD_ROWS|' + JSON.serialize(out));"
						runAnonymous(t, orgFromIndex(index), "anonymous", source)
					})
					t.Run("named", func(t *testing.T) {
						runAnonymous(t, orgFromIndex(index), "named", read("named.apex"))
					})
					t.Run("istest", func(t *testing.T) {
						run := Run(index, Options{SelectedClasses: []string{"StandardChildFieldProbeTest"}, NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
							t.Fatalf("test selection: %+v", run.Summary())
						}
						check(t, "istest", run.Suites[0].Cases[0].Problem.Message)
					})
					t.Run("persisted-named", func(t *testing.T) {
						org := orgFromIndex(index)
						missing := map[string][]string{
							"FinanceBalanceSnapshot": {"AccountId"}, "FinanceTransaction": {"AccountId"},
							"ExtlRecShrStagedRecord": {"AccountId", "ParentRecordId"},
							"MessagingEndUser":       {"AccountId", "ContactId", "AuthenticatedEndUserId"},
							"MessagingSession":       {"CaseId", "OpportunityId"}, "Order": {"OpportunityId"},
						}
						for child, fields := range missing {
							storage.EnsureStandardObject(&org, child)
							state := org.Objects[child]
							state.Definition = state.Definition.Clone()
							for _, field := range fields {
								delete(state.Definition.Fields, field)
							}
							org.Objects[child] = state
						}
						store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "org.db"))
						if err != nil {
							t.Fatal(err)
						}
						defer store.Close()
						if err := store.Save(org); err != nil {
							t.Fatal(err)
						}
						loaded, err := store.Load()
						if err != nil {
							t.Fatal(err)
						}
						for child, fields := range missing {
							for _, field := range fields {
								if _, ok := storage.ResolveFieldName(loaded.Objects[child].Definition, "", field); ok {
									t.Fatalf("persisted fixture still has %s.%s", child, field)
								}
							}
						}
						runAnonymous(t, loaded, "named", read("named.apex"))
					})
				})
			}
		})
	}
}
