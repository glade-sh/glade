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

func TestSchemaReferenceGateFeatureNativeCLIProjectRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	var data struct {
		Features []struct {
			API, Query, Apex string
			Enabled          bool
		}
	}
	raw, err := os.ReadFile("../sema/testdata/schema_relationship_gate/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Features) != 4 {
		t.Fatal("missing native feature cases")
	}
	for _, c := range data.Features {
		if !c.Enabled {
			continue
		}
		for route, source := range map[string]string{"Query": c.Query, "Apex": c.Apex} {
			t.Run(c.API+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				owner := "Feature" + route + strings.TrimSuffix(c.API, ".0")
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"`+c.API+`"}`)
				writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
				writeTestFile(t, filepath.Join(root, "src/classes", owner+".cls"), source)
				writeTestFile(t, filepath.Join(root, "src/classes/RelationshipTest.cls"), `@IsTest private class RelationshipTest{@IsTest static void observe(){if(false){`+owner+`.value();}System.assert(true);}}`)
				writeTestFile(t, filepath.Join(root, "src/classes/IndependentTest.cls"), `@IsTest private class IndependentTest{@IsTest static void observe(){System.assertEquals(7,7);}}`)
				for _, invalid := range []bool{true, false} {
					target := "Account"
					if invalid {
						target = "UnavailableDestination"
					}
					writeTestFile(t, filepath.Join(root, "src/objects/Contact/fields/FeatureDestination__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>FeatureDestination__c</fullName><type>Lookup</type><referenceTo>`+target+`</referenceTo><relationshipName>Features</relationshipName></CustomField>`)
					for _, cached := range []bool{false, true, true} {
						var out, errs bytes.Buffer
						args := []string{"test", "--project", root, "--no-serve", "--no-progress", "--json"}
						if !cached {
							args = append(args, "--no-cache")
						}
						code := Run(context.Background(), args, &out, &errs)
						run, err := decodeTestRunJSON(out.Bytes())
						if err != nil {
							t.Fatalf("%v output=%s", err, out.String())
						}
						got := map[string]testreport.Status{}
						for _, s := range run.Suites {
							for _, tc := range s.Cases {
								got[tc.ClassName] = tc.Status
							}
						}
						want := testreport.StatusPass
						if invalid {
							want = testreport.StatusCompileError
						}
						if (code != 0) != invalid || got["RelationshipTest"] != want || got["IndependentTest"] != testreport.StatusPass {
							t.Fatalf("invalid=%t cached=%t code=%d statuses=%v stderr=%s", invalid, cached, code, got, errs.String())
						}
					}
				}
			})
		}
	}
}
