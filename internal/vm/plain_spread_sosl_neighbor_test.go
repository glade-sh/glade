package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// SOSL was not captured for this object. Its local outcome stays at main.
func TestPlainDeveloperSpreadPreservesUnprobedSOSL(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	program, err := CompileAnonymous(`Search.query('FIND {candidate} RETURNING AIFeatureExtractor(Id)');`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("origin/main SOSL outcome changed: %v", err)
	}
}
