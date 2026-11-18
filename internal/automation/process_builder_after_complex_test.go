package automation

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native afteraccept and afteruser rows pin this graph. Adjacent unsupported
// recipient and formula shapes must still leave the process unrun.
func TestProcessBuilderHeterogeneousAfterSaveBoundaries(t *testing.T) {
	base := filepath.Join("..", "apextest", "testdata", "conformance")
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(base, "process_builder_shapes", "project"))); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"process_builder_review", "process_builder_review_aftercomplex"} {
		overlay := filepath.Join(base, name, "project")
		if err := filepath.WalkDir(overlay, func(path string, entry fs.DirEntry, err error) error {
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
		}); err != nil {
			t.Fatal(err)
		}
	}
	workflow := filepath.Join(root, "force-app/main/default/workflows/Fern_Faf__c.workflow-meta.xml")
	dateFlow := filepath.Join(root, "force-app/main/default/flows/Fern_Faf_C.flow-meta.xml")
	workflowSource, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatal(err)
	}
	dateSource, err := os.ReadFile(dateFlow)
	if err != nil {
		t.Fatal(err)
	}
	dateField := filepath.Join(root, "force-app/main/default/objects/Fern_Faf__c/fields/DayA__c.field-meta.xml")
	dateFieldSource, err := os.ReadFile(dateField)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, before, after string
		runs                      bool
	}{
		{name: "captured", runs: true},
		{name: "uncaptured alert recipient", path: workflow, before: "<type>email</type>", after: "<type>owner</type>"},
		{name: "uncaptured date formula", path: dateFlow, before: "DATEVALUE", after: "DATETIMEVALUE"},
		{name: "uncaptured date field type", path: dateField, before: "<type>Date</type>", after: "<type>DateTime</type>"},
		{name: "uncaptured alert cc", path: workflow, before: "<description>", after: "<ccEmails>none@example.invalid</ccEmails><description>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(workflow, workflowSource, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dateFlow, dateSource, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dateField, dateFieldSource, 0644); err != nil {
				t.Fatal(err)
			}
			if tc.path != "" {
				b, err := os.ReadFile(tc.path)
				if err != nil {
					t.Fatal(err)
				}
				changed := strings.Replace(string(b), tc.before, tc.after, 1)
				if changed == string(b) {
					t.Fatal("mutation missed")
				}
				if err := os.WriteFile(tc.path, []byte(changed), 0644); err != nil {
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
			for _, flow := range idx.Flows {
				if filepath.Base(flow.File) == "Fern_Faf_Process.flow-meta.xml" {
					found = true
					runs = flow.ObjectName != ""
				}
			}
			for _, item := range idx.Diagnostics {
				if item.Code == "GLADEAUTO002" && filepath.Base(item.File) == "Fern_Faf_Process.flow-meta.xml" && strings.Contains(item.Message, "several after-save flows") {
					warned = true
				}
			}
			if !found || runs != tc.runs || warned == tc.runs {
				t.Fatalf("found=%v runs=%v warned=%v want runs=%v", found, runs, warned, tc.runs)
			}
		})
	}
}
