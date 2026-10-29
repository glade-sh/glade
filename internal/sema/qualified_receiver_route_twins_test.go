package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The bodies and compile outcomes come from the anonymous API 62/67 capture.
// These derived routes ensure the shared body checker makes the same decision
// when reached through a named method, an isTest method, or a trigger.
func TestQualifiedReceiverShapesNamedIsTestTriggerRouteTwins(t *testing.T) {
	const dir = "testdata/qualified_receiver_shapes"
	rows := readQualifiedReceiverTSV(t, filepath.Join(dir, "org.tsv"), 4)
	deferred := qualifiedReceiverUndeclaredNamespaceDeferrals(t, dir)
	for _, row := range rows[1:] {
		for _, route := range []string{"named", "isTest", "trigger"} {
			t.Run(row[0]+"/"+route+"/"+row[1], func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(dir, row[1]+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				path := filepath.Join(root, "ReceiverRoute.cls")
				source := "public class ReceiverRoute { public static void observe() {" + string(body) + "} }"
				if route == "isTest" {
					source = "@IsTest private class ReceiverRoute { @IsTest static void observe() {" + string(body) + "} }"
				}
				if route == "trigger" {
					path = filepath.Join(root, "ReceiverRoute.trigger")
					source = "trigger ReceiverRoute on Account (before insert) {" + string(body) + "}"
				}
				writeSemaFile(t, path, source)
				paths := []string{path}
				for _, name := range []string{"KnownType", "Rollup"} {
					data, err := os.ReadFile(filepath.Join(dir, "force-app/main/default/classes", name+".cls"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, name+".cls")
					writeSemaFile(t, path, string(data))
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{
					{Name: "Account"}, {Name: "Contact"},
					{Name: "Settings__c", CustomSettingsType: "List", Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}},
				}})
				index.VisualforcePagesKnown = true
				index.VisualforcePageNames = []string{"ownedlanding"}
				result := Analyze(index)
				if deferred[row[1]] {
					if result.HasErrors() {
						t.Fatalf("deferred undeclared-namespace row; derived %s diagnostics=%+v", route, result.Diagnostics)
					}
					return
				}
				if (row[2] == "true") == result.HasErrors() {
					t.Fatalf("native compiled=%s (%s); derived %s diagnostics=%+v", row[2], row[3], route, result.Diagnostics)
				}
			})
		}
	}
}
