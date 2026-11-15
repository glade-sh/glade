package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The source-backed direct query stays at its origin/main result for an
// uncaptured neighbor, even if the physical inventory is made eager-equivalent.
func TestSourceSpreadDirectQueryPreservesNeighbor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
	}{
		{"default", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}},
		{"optional", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.EnableStandardSpreadSource()
			storage.ApplyProjectOrgShape(&org, tc.features)
			query, err := Parse("SELECT Id FROM CareProgram LIMIT 1")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(org, query); err == nil {
				t.Fatal("origin/main rejected the uncaptured neighbor query")
			}
		})
	}
}
