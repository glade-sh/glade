package apextest

import (
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestLinkedCheckboxReviewProjectRoutes replays the captured values through
// anonymous, named, @IsTest project runner, and repeated warm executions.
// Native data and @IsTest outcomes: ../vm/testdata/linked_checkbox_review/org.tsv.
func TestLinkedCheckboxReviewProjectRoutes(t *testing.T) {
	const fixture = "../vm/testdata/linked_checkbox_review"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	rows := map[string]map[string][]string{}
	for n := 0; ; n++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			continue
		}
		if len(rec) < 4 {
			t.Fatalf("malformed native row %d", n+1)
		}
		if rows[rec[0]] == nil {
			rows[rec[0]] = map[string][]string{}
		}
		rows[rec[0]][rec[1]+"/"+rec[2]] = rec[3:]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			data := rows[api]
			value := func(id string, col int) string {
				t.Helper()
				row := data["anonymous/"+id]
				if len(row) <= col {
					t.Fatalf("missing native row %s/%s", api, id)
				}
				v := row[col]
				if v != "true" && v != "false" && v != "null" {
					t.Fatalf("invalid literal %q", v)
				}
				return v
			}
			if got := data["istest/project-observe"]; len(got) != 1 || got[0] != "Pass" {
				t.Fatalf("native @IsTest outcome: %v", got)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["B2BCommerce","Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true}}}`)
			objects := filepath.Join(fixture, "objects")
			err := filepath.WalkDir(objects, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(objects, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/objects", rel), string(raw))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			cell := 0
			check := func(expected, actual string) string {
				cell++
				return fmt.Sprintf("System.assertEquals(%s,%s,'cell-%d');\n", expected, actual, cell)
			}
			body := `Parent__c p=new Parent__c(Name='parent',Flag__c=null,FalseFlag__c=null); Child__c child=new Child__c(Parent__r=p);` + "\n"
			for _, c := range []struct{ id, dotted, dynamic string }{
				{"custom-null-true", "child.Parent__r.Flag__c", "child.Parent__r.get('Flag__c')"},
				{"custom-null-false", "child.Parent__r.FalseFlag__c", "child.Parent__r.get('FalseFlag__c')"},
			} {
				body += check(value(c.id, 0), c.dotted) + check(value(c.id, 1), c.dynamic)
			}
			body += `p.put('Flag__c',null);` + check(value("custom-null-true", 0), "child.Parent__r.Flag__c") + check(value("custom-null-true", 1), "child.Parent__r.get('Flag__c')")
			body += `Contact ct=new Contact(LastName='control',ContactFlag__c=null,DoNotCall=null);` + "\n"
			body += check(value("contact-custom-null", 0), "ct.ContactFlag__c") + check(value("contact-custom-null", 1), "ct.get('ContactFlag__c')")
			body += check(value("contact-standard-null", 0), "ct.DoNotCall") + check(value("contact-standard-null", 1), "ct.get('DoNotCall')")
			body += `SObject w=Schema.getGlobalDescribe().get('WebStore').newSObject(); w.put('OptionsAutoFacetingEnabled',null);` + check(value("webstore-null", 0), "w.get('OptionsAutoFacetingEnabled')")
			body += `Parent__c saved=new Parent__c(Name='saved');insert saved;` + check(value("caller-omitted", 0), "saved.Flag__c") + check(value("caller-omitted", 1), "saved.get('Flag__c')")
			body += `Parent__c byId=new Parent__c(Id=saved.Id);` + check(value("id-only", 0), "byId.Flag__c") + check(value("id-only", 1), "byId.get('Flag__c')")
			body += `Parent__c q=[SELECT Flag__c FROM Parent__c WHERE Id=:saved.Id];` + check(value("queried", 0), "q.Flag__c") + check(value("queried", 1), "q.get('Flag__c')")
			body += `Parent__c copied=(Parent__c)saved.clone(true);` + check(value("clone", 0), "copied.Flag__c") + check(value("clone", 1), "copied.get('Flag__c')")
			status := data["anonymous/null-insert"]
			if len(status) != 1 {
				t.Fatal("missing rejection")
			}
			body += `Boolean rejected=false; try {insert p;} catch(DmlException e) {rejected=true;` + check("'"+status[0]+"'", "String.valueOf(e.getDmlStatusCode(0))") + `} System.assert(rejected);`
			runStandardFieldEvidenceRoutes(t, root, api, body)
		})
	}
}
