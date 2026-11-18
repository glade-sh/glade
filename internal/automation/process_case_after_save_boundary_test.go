package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native process_builder_review_aftercase rows cover the admitted graph;
// adjacent lookup, update, input and decision variants remain unrun.
func TestProcessBuilderCaseAfterSaveBoundaries(t *testing.T) {
	source := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_aftercase", "project")
	for _, tc := range []struct {
		name, file, before, after string
		runs                      bool
	}{
		{name: "captured", runs: true},
		{name: "uncaptured lookup type", file: "Fern_Case_A.flow-meta.xml", before: "<stringValue>Queue</stringValue>", after: "<stringValue>Regular</stringValue>"},
		{name: "uncaptured update value", file: "Fern_Case_A.flow-meta.xml", before: "<stringValue>New</stringValue>", after: "<stringValue>Working</stringValue>"},
		{name: "uncaptured action input", file: "Fern_Case_B.flow-meta.xml", before: "$Record.Id", after: "$Record.Subject"},
		{name: "uncaptured relationship field", file: "Fern_Case_C.flow-meta.xml", before: "$Record.Fern_Person__r.Email", after: "$Record.Fern_Person__r.Phone"},
		{name: "uncaptured custom error merge", file: "Fern_Case_C.flow-meta.xml", before: "Missing person", after: "Missing {!$Record.Subject}"},
		{name: "uncaptured lookup object", file: "Fern_Case_A.flow-meta.xml", before: "<object>Group</object>", after: "<object>Account</object>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(source)); err != nil {
				t.Fatal(err)
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
