package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The queue-owner and status-transition capture bounds this graph.
func TestProcessBuilderCaseFormulaAfterSaveBoundaries(t *testing.T) {
	source := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_aftercase", "project")
	for _, tc := range []struct {
		name, file, before, after string
		runs                      bool
	}{
		{name: "captured", runs: true},
		{name: "other assignment target", file: "Fern_Case_A.flow-meta.xml", before: "$Record.OwnerId", after: "$Record.Subject"},
		{name: "other lookup sort", file: "Fern_Case_A.flow-meta.xml", before: "<sortOrder>Asc</sortOrder>", after: "<sortOrder>Desc</sortOrder>"},
		{name: "other prior field", file: "Fern_Case_C.flow-meta.xml", before: "$Record__Prior.Status", after: "$Record__Prior.Origin"},
		{name: "different formula branch", file: "Fern_Case_C.flow-meta.xml", before: `!= "Working"`, after: `!= "New"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(source)); err != nil {
				t.Fatal(err)
			}
			overlay := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_aftercaseformula", "project", "force-app", "main", "default", "flows")
			for _, name := range []string{"Fern_Case_A.flow-meta.xml", "Fern_Case_C.flow-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(overlay, name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "force-app", "main", "default", "flows", name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.file != "" {
				p := filepath.Join(root, "force-app/main/default/flows", tc.file)
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				changed := strings.Replace(string(raw), tc.before, tc.after, 1)
				if changed == string(raw) {
					t.Fatal("mutation missed")
				}
				if err := os.WriteFile(p, []byte(changed), 0644); err != nil {
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
				if filepath.Base(flow.File) == "Fern_Case_Process.flow-meta.xml" {
					found = true
					runs = flow.ObjectName != ""
				}
			}
			for _, d := range idx.Diagnostics {
				if d.Code == "GLADEAUTO002" && filepath.Base(d.File) == "Fern_Case_Process.flow-meta.xml" && strings.Contains(d.Message, "several after-save flows") {
					warned = true
				}
			}
			if !found || runs != tc.runs || warned == tc.runs {
				t.Fatalf("found=%v runs=%v warned=%v want runs=%v", found, runs, warned, tc.runs)
			}
		})
	}
}
