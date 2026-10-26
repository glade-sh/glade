package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestSchemaReferenceGateTriggerAndSingletonCLIProjectRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	var data struct {
		Receivers []struct{ API, Trigger, Singleton string }
	}
	raw, err := os.ReadFile("../sema/testdata/schema_relationship_gate/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Receivers) != 2 {
		t.Fatal("missing native receiver captures")
	}
	for _, c := range data.Receivers {
		for _, route := range []string{"trigger_constructor", "trigger_indexed", "trigger_loop", "trigger_get", "query_singleton"} {
			t.Run(c.API+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				n := strings.TrimSuffix(c.API, ".0")
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"`+c.API+`"}`)
				owner := "SingletonRelation" + n
				body := `if(false){SingletonRelation` + n + `.value();}System.assert(true);`
				if strings.HasPrefix(route, "trigger_") {
					owner = "Relationship" + n
					body = `insert new Contact(LastName='probe');`
					triggerBodies := map[string]string{"trigger_constructor": "Object a=new Contact().Account;", "trigger_indexed": "Object b=Trigger.new[0].Account;", "trigger_loop": "for(Contact c:Trigger.new){Object d=c.Account;}", "trigger_get": "Object e=Trigger.new.get(0).Account;"}
					source := strings.SplitN(c.Trigger, "{", 2)[0] + "{" + triggerBodies[route] + "}"
					writeTestFile(t, filepath.Join(root, "src/triggers", owner+".trigger"), source)
				} else {
					writeTestFile(t, filepath.Join(root, "src/classes", owner+".cls"), c.Singleton)
				}
				writeTestFile(t, filepath.Join(root, "src/classes/RelationshipTest.cls"), `@IsTest private class RelationshipTest{@IsTest static void observe(){`+body+`}}`)
				writeTestFile(t, filepath.Join(root, "src/classes/IndependentTest.cls"), `@IsTest private class IndependentTest{@IsTest static void observe(){System.assertEquals(7,7);}}`)
				for _, invalid := range []bool{true, false} {
					target := "Account"
					if invalid {
						target = "UnavailableDestination"
					}
					writeTestFile(t, filepath.Join(root, "src/objects/Account/fields/TriggerDestination"+n+"__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TriggerDestination`+n+`__c</fullName><type>Lookup</type><referenceTo>`+target+`</referenceTo><relationshipName>Destinations</relationshipName></CustomField>`)
					for _, cached := range []bool{false, true, true} {
						var out, errs bytes.Buffer
						args := []string{"test", "--project", root, "--no-serve", "--no-progress", "--json"}
						if !cached {
							args = append(args, "--no-cache")
						}
						code := Run(context.Background(), args, &out, &errs)
						run, err := decodeTestRunJSON(out.Bytes())
						if err != nil {
							t.Fatalf("%v out=%s", err, out.String())
						}
						found := false
						for _, x := range run.ExcludedClasses {
							if x.ClassName == owner {
								found = true
							}
						}
						got := map[string]testreport.Status{}
						for _, s := range run.Suites {
							for _, tc := range s.Cases {
								got[tc.ClassName] = tc.Status
							}
						}
						if (code != 0) != invalid || found != invalid || got["IndependentTest"] != testreport.StatusPass {
							t.Fatalf("invalid=%t cached=%t code=%d ownerExcluded=%t statuses=%v stderr=%s", invalid, cached, code, found, got, errs.String())
						}
						if route == "query_singleton" {
							want := testreport.StatusPass
							if invalid {
								want = testreport.StatusCompileError
							}
							if got["RelationshipTest"] != want {
								t.Fatalf("dependent status=%v want=%s", got, want)
							}
						}
					}
				}
			})
		}
	}
}
