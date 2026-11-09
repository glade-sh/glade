package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The owned reference packet captures the full field maps and feature boundary
// on both anonymous and named routes at API 62 and 67.
func TestB5B9NPCReferenceObjectNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	for _, api := range []string{"62.0", "67.0"} {
		data, err := os.ReadFile(dir + "npc-references-" + api + ".json")
		if err != nil {
			t.Fatal(err)
		}
		objects := map[string]b5b9CapturedShape{}
		if err := json.Unmarshal(data, &objects); err != nil {
			t.Fatal(err)
		}
		if len(objects) != 3 {
			t.Fatalf("captured %s reference packet has %d objects, want 3", api, len(objects))
		}
		names := make([]string, 0, len(objects))
		for name := range objects {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, label := range []string{"disabled", "license-only", "enabled"} {
			enabled := label == "enabled"
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if enabled || label == "license-only" {
				definition := "npc-minimal.json"
				if label == "license-only" {
					definition = "npc.json"
				}
				def, err := os.ReadFile(dir + definition)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(def))
			} else {
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
			}
			t.Run(api+"/"+label, func(t *testing.T) {
				body := `Map<String,Schema.SObjectType> all=Schema.getGlobalDescribe(); System.assert(all.containsKey('Account'));`
				for _, name := range names {
					want := "false"
					if enabled {
						want = "true"
					}
					body += `System.assertEquals(` + want + `,all.containsKey('` + name + `'),'` + name + `');`
					if enabled {
						fields := make([]string, 0, len(objects[name].Fields))
						for _, f := range objects[name].Fields {
							fields = append(fields, f.Name)
						}
						body += "{" + b5b9ShapeBody(name, fields) + "}"
					}
				}
				if enabled {
					body += `Schema.SObjectType ref=GiftTransaction.SObjectType.getDescribe().fields.getMap().get('GiftCommitmentScheduleId').getDescribe().getReferenceTo()[0];System.assertEquals('GiftCommitmentSchedule',ref.getDescribe().getName());`
				}
				b5b9RunShapeRoutes(t, root, api, body, body)
			})
		}
	}
}

func TestB5B9NPCReferenceStaticNativeRoutes(t *testing.T) {
	f, err := os.Open("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/org.tsv")
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
	for _, row := range rows[1:] {
		if row[1] != "npc-reference-access" || row[2] != "anonymous" {
			continue
		}
		op := strings.TrimPrefix(row[3], "GiftCommitmentSchedule.GenerationalCohort-")
		if op != "read" && op != "write" && op != "query" {
			continue
		}
		t.Run(row[0]+"/"+op, func(t *testing.T) {
			source := "GiftCommitmentSchedule r=new GiftCommitmentSchedule(); Object value=r.GenerationalCohort;"
			if op == "write" {
				source = "GiftCommitmentSchedule r=new GiftCommitmentSchedule(); r.GenerationalCohort=null;"
			}
			if op == "query" {
				source = "List<GiftCommitmentSchedule> rows=[SELECT GenerationalCohort FROM GiftCommitmentSchedule LIMIT 1];"
			}
			root := b5b9AccessProject(t, "gift", row[0])
			path := filepath.Join(root, "force-app/main/default/classes/OwnedReferenceStatic.cls")
			writeFile(t, path, "public class OwnedReferenceStatic {public static void probe(){"+source+"}}")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			namedRejected := index.HasErrors() || sema.Analyze(index).HasErrors()
			anonRejected := sema.AnalyzeAnonymous(index, source, row[0]).HasErrors()
			wantRejected := row[4] == "rejected"
			if namedRejected != wantRejected || anonRejected != wantRejected {
				t.Errorf("native rejected=%t named=%t anonymous=%t", wantRejected, namedRejected, anonRejected)
			}
		})
	}
}

// The source field is absent at 62 and present at 63-67 in the captured
// version matrix, including direct reads/writes, query and dynamic operations.
func TestB5B9NPCReferenceVersionNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := b5b9AccessProject(t, "gift", api)
			want := "true"
			if api == "62.0" {
				want = "false"
			}
			body := `Map<String,Schema.SObjectType> m=Schema.getGlobalDescribe();SObject s=m.get('GiftCommitmentSchedule').newSObject();Boolean ok;System.assertEquals(` + want + `,m.get('GiftCommitmentSchedule').getDescribe().fields.getMap().containsKey('GenerationalCohort'));` +
				`ok=true;try{s.get('GenerationalCohort');}catch(SObjectException e){ok=false;}System.assertEquals(` + want + `,ok);` +
				`ok=true;try{s.put('GenerationalCohort',null);}catch(SObjectException e){ok=false;}System.assertEquals(` + want + `,ok);` +
				`ok=true;try{Database.query('SELECT GenerationalCohort FROM GiftCommitmentSchedule LIMIT 1');}catch(QueryException e){ok=false;}System.assertEquals(` + want + `,ok);`
			runStandardFieldEvidenceRoutes(t, root, api, body)
		})
	}
}
