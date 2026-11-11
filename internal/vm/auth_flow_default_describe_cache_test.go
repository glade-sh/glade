package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native field-map observations: auth_flow_default_presence/org.tsv.
// The mixed-version sequence checks that one VM does not reuse another
// source version's cached field map.
func TestAuthFlowDefaultDescribeWarmVersionBoundary(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "AuthProvider")
	machine := New(nil)
	machine.SetOrg(&org)
	for _, row := range []struct {
		api     string
		present bool
	}{{"62.0", false}, {"64.0", true}, {"63.0", false}, {"67.0", true}, {"62.0", false}} {
		machine.currentMethod = Method{APIVersion: row.api, SourceContextBound: true}
		fields := fieldMapGet(t, machine, fieldMapWrapper(t, machine, "AuthProvider"))
		for _, field := range []string{"FlowDefaultAccountId", "FlowDefaultProfileId"} {
			_, present := fields.Map[mapKey(String(strings.ToLower(field)))]
			if present != row.present {
				t.Errorf("api=%s field=%s present=%t, want %t", row.api, field, present, row.present)
			}
		}
	}
}
