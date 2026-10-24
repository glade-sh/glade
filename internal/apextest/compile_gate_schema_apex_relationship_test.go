package apextest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

type relationshipNativeCase struct {
	API, ID, Name, Source, Target string
	References, NativeValid       bool
	ExtraFiles                    map[string]string
}

func relationshipNativeCases(t *testing.T) ([]relationshipNativeCase, map[string]map[string]string) {
	t.Helper()
	const dir = "../sema/testdata/schema_relationship_gate"
	var data struct {
		Cases    []relationshipNativeCase
		Metadata map[string]map[string]string
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	oracle := map[string]bool{}
	for _, row := range rows[1:] {
		oracle[strings.Join(row[:3], "/")] = row[3] == "true"
	}
	for _, c := range data.Cases {
		for route, want := range map[string]bool{"missing_metadata_deploy": false, "loaded_metadata_deploy": c.NativeValid} {
			got, ok := oracle[c.API+"/"+c.ID+"/"+route]
			if !ok || got != want {
				t.Fatalf("missing or inconsistent native %s/%s/%s", c.API, c.ID, route)
			}
		}
		if c.NativeValid && !oracle[c.API+"/"+c.ID+"/istest"] {
			t.Fatalf("native named route did not pass: %s/%s", c.API, c.ID)
		}
	}
	if len(data.Cases) == 0 {
		t.Fatal("empty native case table")
	}
	return data.Cases, data.Metadata
}

func relationshipNativeIndex(t *testing.T, c relationshipNativeCase, files map[string]string, invalid bool) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"`+c.API+`"}`)
	for path, text := range files {
		writeFile(t, filepath.Join(root, "src", path), text)
	}
	for path, text := range c.ExtraFiles {
		writeFile(t, filepath.Join(root, "src", path), text)
	}
	writeFile(t, filepath.Join(root, "src/classes", c.Name+".cls"), c.Source)
	writeFile(t, filepath.Join(root, "src/classes/RelationshipTest.cls"), `@IsTest private class RelationshipTest { @IsTest static void observe(){ if(false){ System.assert(`+c.Name+`.value()); } System.assert(true); } }`)
	writeFile(t, filepath.Join(root, "src/classes/IndependentTest.cls"), `@IsTest private class IndependentTest { @IsTest static void observe(){System.assertEquals(7,7);} }`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if invalid {
		found := false
		for i := range s.Objects {
			if strings.EqualFold(s.Objects[i].Name, c.Target) {
				s.Objects[i].Fields = append(s.Objects[i].Fields, schema.Field{Name: "Broken__c", Type: "Lookup", ReferenceTo: []string{"UnavailableDestination"}})
				found = true
			}
		}
		if !found {
			s.Objects = append(s.Objects, schema.Object{Name: c.Target, Fields: []schema.Field{{Name: "Broken__c", Type: "Lookup", ReferenceTo: []string{"UnavailableDestination"}}}})
		}
	}
	return typesys.Build(p, s)
}

// Every case retains its matching native rejected/loaded source and API row.
// Ownership is asserted separately from Salesforce query acceptance.
func TestSchemaReferenceGateRelationshipNativeBindingRoutes(t *testing.T) {
	cases, metadata := relationshipNativeCases(t)
	for _, c := range cases {
		t.Run(c.API+"/"+c.ID, func(t *testing.T) {
			index := relationshipNativeIndex(t, c, metadata[c.API], false)
			got := compileGateReferencesSchemaObject(c.Name+".cls", c.Source, newCompileGateUnits(index), c.Target)
			if got != c.References {
				t.Fatalf("native-shaped %s target=%s referenced=%t want=%t", c.ID, c.Target, got, c.References)
			}
		})
	}
}

func TestSchemaReferenceGateApexRelationshipRunnerProjectRoutes(t *testing.T) {
	cases, metadata := relationshipNativeCases(t)
	for _, c := range cases {
		if !strings.HasPrefix(c.ID, "apex_") || !c.NativeValid {
			continue
		}
		t.Run(c.API+"/"+c.ID, func(t *testing.T) {
			for _, invalid := range []bool{true, false} {
				index := relationshipNativeIndex(t, c, metadata[c.API], invalid)
				for _, cached := range []bool{false, true, true} {
					run := RunCasesContext(context.Background(), index, Options{NoDiskCache: !cached}, nil)
					got := map[string]testreport.Status{}
					for _, suite := range run.Suites {
						for _, tc := range suite.Cases {
							got[tc.ClassName] = tc.Status
						}
					}
					want := testreport.StatusPass
					if invalid && c.References {
						want = testreport.StatusCompileError
					}
					if got["RelationshipTest"] != want || got["IndependentTest"] != testreport.StatusPass || run.Summary().OK() == invalid {
						t.Fatalf("invalid=%t cached=%t statuses=%v summary=%+v exclusions=%+v", invalid, cached, got, run.Summary(), run.ExcludedClasses)
					}
				}
			}
		})
	}
}
