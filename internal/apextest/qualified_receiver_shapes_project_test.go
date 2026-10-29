package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Reuse captured anonymous statement bodies in a reached named helper. This
// pins the project compile gate as well as the direct anonymous sema tests.
// Accepted methods are compiled but not invoked: their runtime side effects
// are outside the compile observations in org.tsv.
func TestQualifiedReceiverShapesProjectNamedNativeConformance(t *testing.T) {
	for _, family := range []string{"qualified_receiver_shapes", "qualified_receiver_widening"} {
		dir := filepath.Join("../sema/testdata", family)
		f, err := os.Open(filepath.Join(dir, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		deferred := qualifiedReceiverDeferrals(t, dir)
		for _, row := range rows[1:] {
			t.Run(family+"/"+row[0]+"/project-isTest-helper/"+row[1], func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(dir, strings.TrimSuffix(row[1], "_reject")+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				files := map[string]string{
					"ReceiverHelper.cls":      "public class ReceiverHelper { public static void observe() {" + string(source) + "} }",
					"ReceiverCompileTest.cls": "@IsTest private class ReceiverCompileTest { @IsTest static void observe() { ReceiverHelper helper = new ReceiverHelper(); System.assertNotEquals(null, helper); } }",
				}
				for _, fixture := range []string{
					"../sema/testdata/qualified_receiver_shapes/force-app/main/default/classes/KnownType.cls",
					"../sema/testdata/qualified_receiver_widening/project/force-app/main/default/classes/Rollup.cls",
					"../sema/testdata/qualified_receiver_widening/project/force-app/main/default/classes/GatewayService.cls",
				} {
					data, err := os.ReadFile(fixture)
					if err != nil {
						t.Fatal(err)
					}
					files[filepath.Base(fixture)] = string(data)
				}
				if row[1] == "dependency_bogus_member" {
					data, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes/Dep.cls"))
					if err != nil {
						t.Fatal(err)
					}
					files["Dep.cls"] = string(data)
				}
				var paths []string
				for name, source := range files {
					path := filepath.Join(root, name)
					writeFile(t, path, source)
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{
					{Name: "Account"}, {Name: "Contact"}, {Name: "Thing__c"},
					{Name: "Settings__c", CustomSettingsType: "List", Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}},
				}})
				index.VisualforcePagesKnown = true
				index.VisualforcePageNames = []string{"ownedlanding"}
				run := Run(index, Options{SelectedClasses: []string{"ReceiverCompileTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				summary := run.Summary()
				if deferred[row[1]] {
					// Undeclared namespace: native rejects only because the
					// capture org lacks it (../sema/testdata/qualified_receiver_shapes/deferred.tsv).
					if summary.Total != 1 || summary.Passed != 1 || summary.CompileErrors != 0 {
						t.Fatalf("deferred row; project result=%+v; problem=%s", summary, firstRunProblem(run))
					}
					return
				}
				if row[2] == "false" {
					if summary.CompileErrors == 0 {
						t.Fatalf("native rejected (%s); project compile gate accepted: %+v", row[3], summary)
					}
				} else if summary.Total != 1 || summary.Passed != 1 || summary.CompileErrors != 0 {
					t.Fatalf("native accepted; project result=%+v; problem=%s", summary, firstRunProblem(run))
				}
			})
		}
	}
}

func qualifiedReceiverDeferrals(t *testing.T, dir string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "deferred.tsv"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	deferred := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		name, _, _ := strings.Cut(line, "\t")
		deferred[name] = true
	}
	return deferred
}
