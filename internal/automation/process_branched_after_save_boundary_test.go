package automation

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native afterperson rows cover the captured graph; unobserved metadata
// variations must continue to warn and leave the process unrun.
func TestProcessBuilderBranchedAfterSaveBoundaries(t *testing.T) {
	base := filepath.Join("..", "apextest", "testdata", "conformance")
	for _, tc := range []struct {
		name, file, before, after string
		runs                      bool
	}{
		{name: "captured", runs: true},
		{name: "different filtered date", file: "flows/Fern_Faf_D.flow-meta.xml", before: "<field>DayE__c</field><operator>IsNull</operator>", after: "<field>DayD__c</field><operator>IsNull</operator>"},
		{name: "different prior delete", file: "flows/Fern_Faf_A.flow-meta.xml", before: "$Record__Prior.Who__c</elementReference>", after: "$Record.Who__c</elementReference>"},
		{name: "different alert recipient", file: "workflows/Fern_Faf__c.workflow-meta.xml", before: "<type>userLookup</type>", after: "<type>owner</type>"},
		{name: "different related target", file: "flows/Fern_Faf_C.flow-meta.xml", before: "<field>PersonBirthdate</field>", after: "<field>Name</field>"},
		{name: "different date expression", file: "flows/Fern_Faf_D.flow-meta.xml", before: "DATEVALUE(", after: "DATETIMEVALUE("},
		{name: "person feature absent", file: "config/project-scratch-def.json", before: "PersonAccounts", after: "OtherFeature"},
		{name: "different alert comparison", file: "flows/Fern_Faf_B.flow-meta.xml", before: "<operator>NotEqualTo</operator>", after: "<operator>EqualTo</operator>"},
		{name: "different Apex input source", file: "flows/Fern_Faf_B.flow-meta.xml", before: "<elementReference>$Record</elementReference>", after: "<elementReference>$Record.Id</elementReference>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(base, "process_builder_shapes", "project"))); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"process_builder_review", "process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson"} {
				overlay := filepath.Join(base, name, "project")
				err := filepath.WalkDir(overlay, func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(overlay, path)
					if err != nil {
						return err
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					dest := filepath.Join(root, rel)
					if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
						return err
					}
					return os.WriteFile(dest, data, 0644)
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.file != "" {
				p := filepath.Join(root, "force-app/main/default", tc.file)
				if strings.HasPrefix(tc.file, "config/") {
					p = filepath.Join(root, tc.file)
				}
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				next := strings.Replace(string(raw), tc.before, tc.after, 1)
				if next == string(raw) {
					t.Fatal("mutation missed")
				}
				if err := os.WriteFile(p, []byte(next), 0644); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			found, runs, warned := false, false, false
			for _, f := range idx.Flows {
				if filepath.Base(f.File) == "Fern_Faf_Process.flow-meta.xml" {
					found = true
					runs = f.ObjectName != ""
				}
			}
			for _, d := range idx.Diagnostics {
				if tc.runs && d.Code == "GLADEAUTO002" && filepath.Base(d.File) == "Fern_Faf_D.flow-meta.xml" && strings.Contains(d.Message, "start filter logic with a filtered triggering-record update") {
					t.Fatalf("captured date-filter warning remains: %s", d.Message)
				}
				if d.Code == "GLADEAUTO002" && filepath.Base(d.File) == "Fern_Faf_Process.flow-meta.xml" {
					warned = true
				}
			}
			if !found || runs != tc.runs || warned == tc.runs {
				t.Fatalf("found=%v runs=%v warned=%v want runs=%v", found, runs, warned, tc.runs)
			}
		})
	}
}
