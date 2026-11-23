package testdaemon

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonInvalidationMatchesCold(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, noCache := range []bool{true, false} {
		policy := "cache-enabled"
		if noCache {
			policy = "no-cache"
		}
		for _, name := range []string{"body", "signature", "delete", "rename", "forceignore", "object", "field", "api-version", "class-api-version", "hydration-provider", "hydration-reader"} {
			t.Run(name+"/"+policy, func(t *testing.T) {
				root := incrementalProject(t)
				classes := filepath.Join(root, "force-app/main/default/classes")
				other := filepath.Join(classes, "Other.cls")
				object := filepath.Join(root, "force-app/main/default/objects/Widget__c/Widget__c.object-meta.xml")
				field := filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/Score__c.field-meta.xml")
				objectXML := `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><label>Widget Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`
				fieldXML := `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Score__c</fullName><label>Score</label><type>Number</type><precision>10</precision><scale>0</scale></CustomField>`
				writeFile(t, object, objectXML)
				writeFile(t, field, fieldXML)
				// The hydration cases add a reader whose check depends on what an
				// earlier type hydrated: Asset's catalog field InstallDate is known
				// only after some body loads Asset.
				hydrator := filepath.Join(classes, "AHydrator.cls")
				reader := filepath.Join(classes, "BReader.cls")
				hydratorSource := func(statement string) string {
					return "public class AHydrator { public static void run() { " + statement + " } }"
				}
				const loadsAsset = "String n = Schema.Asset.SerialNumber.getDescribe().getName();"
				readerSource := func(marker string) string {
					return "public class BReader { public static Object read(ApexPages.StandardSetController ssc) { Integer m = " + marker + "; return ssc.getRecord().InstallDate; } }"
				}
				if strings.HasPrefix(name, "hydration") {
					const meta = `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`
					writeFile(t, hydrator, hydratorSource(loadsAsset))
					writeFile(t, hydrator+"-meta.xml", meta)
					writeFile(t, reader, readerSource("1"))
					writeFile(t, reader+"-meta.xml", meta)
				}
				d, err := New(root)
				if err != nil {
					t.Fatal(err)
				}
				if err := d.Warm(); err != nil {
					t.Fatal(err)
				}
				before, err := json.Marshal(d.IndexSnapshot())
				if err != nil {
					t.Fatal(err)
				}
				var changes []watch.Change
				add := func(path string, op watch.ChangeOp) {
					c := watch.ClassifyPath(path)
					changes = append(changes, watch.Change{Path: path, Op: op, Kind: c.Kind, Name: c.Name})
				}
				switch name {
				case "body":
					path := filepath.Join(classes, "Util.cls")
					writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return (v + v); } }`)
					add(path, watch.ChangeModified)
				case "signature":
					path := filepath.Join(classes, "Util.cls")
					writeFile(t, path, `public class Util { public static Integer twice(Long v) { return 4; } }`)
					add(path, watch.ChangeModified)
				case "delete":
					if err := os.Remove(other); err != nil {
						t.Fatal(err)
					}
					add(other, watch.ChangeDeleted)
				case "rename":
					renamed := filepath.Join(classes, "Renamed.cls")
					if err := os.Remove(other); err != nil {
						t.Fatal(err)
					}
					writeFile(t, renamed, `public class Renamed { public static String name() { return 'other'; } }`)
					add(other, watch.ChangeDeleted)
					add(renamed, watch.ChangeAdded)
				case "forceignore":
					path := filepath.Join(root, ".forceignore")
					writeFile(t, path, "**/Other.cls\n")
					add(path, watch.ChangeModified)
				case "object":
					writeFile(t, object, strings.Replace(objectXML, "<label>Widget</label>", "<label>Changed</label>", 1))
					add(object, watch.ChangeModified)
				case "field":
					writeFile(t, field, strings.Replace(fieldXML, "<precision>10</precision>", "<precision>12</precision>", 1))
					add(field, watch.ChangeModified)
				case "class-api-version":
					path := other + "-meta.xml"
					content, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, path, strings.Replace(string(content), "67.0", "62.0", 1))
					add(path, watch.ChangeModified)
				case "hydration-provider":
					writeFile(t, hydrator, hydratorSource("Integer unused = 1;"))
					add(hydrator, watch.ChangeModified)
				case "hydration-reader":
					writeFile(t, reader, readerSource("(1 + 1)"))
					add(reader, watch.ChangeModified)
				case "api-version":
					path := filepath.Join(root, "sfdx-project.json")
					writeFile(t, path, `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
					add(path, watch.ChangeModified)
				}
				if err := d.UpdateChanges(changes); err != nil {
					t.Fatal(err)
				}
				apextest.InvalidateRuntimeCaches()
				warmErr := d.Warm()
				if strings.HasPrefix(name, "hydration") {
					if stats := d.semaState.LastStats(); !stats.Incremental || stats.Reused == 0 {
						t.Fatalf("warm after %s edit did not reuse semantic checks: %+v", name, stats)
					}
					if flipped := warmErr != nil && strings.Contains(warmErr.Error(), "InstallDate"); flipped != (name == "hydration-provider") {
						t.Fatalf("warm after %s edit: error %v", name, warmErr)
					}
				}
				updated, err := json.Marshal(d.IndexSnapshot())
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(before, updated) {
					t.Fatal("edit did not change the indexed project")
				}
				request := NewRunRequestV1(apextest.Options{NoDiskCache: noCache, Parallelism: 1}, "", 0, 0, false)
				warmRun, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				request.ReturnClassShards = true
				request.ShardCount = 2
				_, _, warmPlan, err := d.RunRequestV1(context.Background(), request, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				apextest.InvalidateRuntimeCaches()
				cold, err := New(root)
				if err != nil {
					t.Fatal(err)
				}
				coldErr := cold.Warm()
				coldIndex, err := json.Marshal(cold.IndexSnapshot())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(updated, coldIndex) {
					t.Fatal("updated index differs from cold index")
				}
				if (warmErr == nil) != (coldErr == nil) || warmErr != nil && warmErr.Error() != coldErr.Error() {
					t.Fatalf("warm and cold diagnostics differ: %v / %v", warmErr, coldErr)
				}
				_, _, coldPlan, err := cold.RunRequestV1(context.Background(), request, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(warmPlan, coldPlan) {
					t.Fatalf("warm and cold plans differ: %+v / %+v", warmPlan, coldPlan)
				}
				request.ReturnClassShards = false
				request.ShardCount = 0
				coldRun, _, _, err := cold.RunRequestV1(context.Background(), request, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if bodyEditOutcomeDigest(warmRun) != bodyEditOutcomeDigest(coldRun) {
					t.Fatalf("warm and cold outcomes differ: %s / %s", runProblems(warmRun), runProblems(coldRun))
				}
				clearRunDurations(&warmRun)
				clearRunDurations(&coldRun)
				if !reflect.DeepEqual(warmRun, coldRun) {
					t.Fatal("warm and cold reports differ beyond durations")
				}
			})
		}
	}
}

func clearRunDurations(run *testreport.Run) {
	run.DurationMS = 0
	for i := range run.Suites {
		run.Suites[i].DurationMS = 0
		for j := range run.Suites[i].Cases {
			run.Suites[i].Cases[j].DurationMS = 0
		}
	}
}
