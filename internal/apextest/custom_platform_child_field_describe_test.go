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

// The walk lists every child relationship of a custom object, describes both
// the relationship field and the token returned by the child's field map, and
// emits the native capture columns for the captured finance edge fields.
const childFieldWalkBody = `
        Set<String> rows = new Set<String>();
        List<String> failures = new List<String>();
        for (Schema.ChildRelationship cr : ChildFieldHost__c.SObjectType.getDescribe().getChildRelationships()) {
            if (cr.getRelationshipName() == null) { continue; }
            try {
                String obj = String.valueOf(cr.getChildSObject());
                String fieldName = String.valueOf(cr.getField());
                Schema.DescribeFieldResult direct = cr.getField().getDescribe();
                Schema.SObjectField tok = cr.getChildSObject().getDescribe().fields.getMap().get(fieldName.toLowerCase());
                if (tok == null) { failures.add(cr.getRelationshipName() + ':map'); continue; }
                Schema.DescribeFieldResult d = tok.getDescribe();
                if (d.getName() != direct.getName()) { failures.add(cr.getRelationshipName() + ':name'); }
                if (obj != 'FinanceTransaction' && obj != 'FinanceBalanceSnapshot') { continue; }
                List<String> refs = new List<String>();
                for (Schema.SObjectType r : d.getReferenceTo()) { refs.add(String.valueOf(r)); }
                refs.sort();
                rows.add(String.join(new List<String>{obj, d.getName(), 'ok', String.valueOf(d.getType()), String.join(refs, ','), String.valueOf(d.isNillable()), String.valueOf(d.isCreateable()), String.valueOf(d.isUpdateable()), String.valueOf(d.isDefaultedOnCreate()), d.getRelationshipName(), String.valueOf(d.isAccessible())}, '|'));
            } catch (Exception e) {
                failures.add(cr.getRelationshipName() + ':' + e.getMessage());
            }
        }
        List<String> out = new List<String>(rows);
        out.sort();
        failures.sort();
        out.addAll(failures);
`

const childFieldHelperClass = `public class ChildFieldDescribeHelper {
    public static List<String> rows() {` + childFieldWalkBody + `        return out;
    }
}`

const childFieldHelperTest = `@IsTest private class ChildFieldDescribeHelperTest {
    @IsTest static void walk() { System.assert(false, 'CHILD_ROWS|' + JSON.serialize(ChildFieldDescribeHelper.rows())); }
}`

func nativeChildFieldRows(t *testing.T, api string) []string {
	t.Helper()
	f, err := os.Open("testdata/conformance/platform_child_field_describe/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, row := range records[1:] {
		if row[0] == api {
			want = append(want, strings.Join(row[2:], "|"))
		}
	}
	if len(want) != 7 {
		t.Fatalf("native matrix for %s has %d rows", api, len(want))
	}
	sort.Strings(want)
	return want
}

// A custom object lists platform child relationships. Describing each child's
// field token, directly or through the child's field map, must resolve and
// carry the native facets.
func TestCustomPlatformChildFieldDescribeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/ChildFieldHost__c/ChildFieldHost__c.object-meta.xml"),
				`<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Child Field Host</label><pluralLabel>Child Field Hosts</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			for name, body := range map[string]string{"ChildFieldDescribeHelper": childFieldHelperClass, "ChildFieldDescribeHelperTest": childFieldHelperTest} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, body)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			want := nativeChildFieldRows(t, api)
			check := func(t *testing.T, message string) {
				t.Helper()
				_, payload, found := strings.Cut(message, "CHILD_ROWS|")
				var got []string
				if !found || json.Unmarshal([]byte(payload), &got) != nil {
					t.Fatalf("invalid observation: %s", message)
				}
				if !reflect.DeepEqual(want, got) {
					t.Fatalf("native rows %v; Glade (including failures) %v", want, got)
				}
			}
			runAnonymous := func(t *testing.T, org storage.OrgState, source string) {
				t.Helper()
				org.APIVersion = api
				base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatalf("anonymous diagnostics: %v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := base.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				check(t, strings.Join(result.Debug, "\n"))
			}
			const namedCall = "System.debug(LoggingLevel.ERROR, 'CHILD_ROWS|' + JSON.serialize(ChildFieldDescribeHelper.rows()));"
			// Route 1: the walk is compiled inline as anonymous source.
			t.Run("anonymous", func(t *testing.T) {
				inline := strings.Replace(childFieldWalkBody, "List<String> out = new List<String>(rows);", "out = new List<String>(rows);", 1)
				source := "List<String> out;\n" + inline + "System.debug(LoggingLevel.ERROR, 'CHILD_ROWS|' + JSON.serialize(out));"
				runAnonymous(t, orgFromIndex(index), source)
			})
			// Route 2: a named project class method called from anonymous source.
			t.Run("named", func(t *testing.T) {
				runAnonymous(t, orgFromIndex(index), namedCall)
			})
			// Route 3: the native test runner.
			t.Run("istest", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ChildFieldDescribeHelperTest"}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("test selection: %+v", run.Summary())
				}
				check(t, run.Suites[0].Cases[0].Problem.Message)
			})
			// An org persisted by an older binary holds Id-only finance
			// definitions that still carry the overlay marker.
			t.Run("persisted", func(t *testing.T) {
				org := orgFromIndex(index)
				for _, name := range []string{"FinanceTransaction", "FinanceBalanceSnapshot"} {
					storage.EnsureStandardObject(&org, name)
					state := org.Objects[name]
					state.Definition = state.Definition.Clone()
					for fieldName := range state.Definition.Fields {
						if strings.HasSuffix(fieldName, "EntityId") {
							delete(state.Definition.Fields, fieldName)
						}
					}
					org.Objects[name] = state
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
				if _, ok := storage.ResolveFieldName(loaded.Objects["FinanceTransaction"].Definition, "", "DestinationEntityId"); ok {
					t.Fatal("persisted fixture still has the edge field")
				}
				runAnonymous(t, loaded, namedCall)
			})
		})
	}
}
