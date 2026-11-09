package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestApprovalProjectHostedRoutingBoundaries(t *testing.T) {
	for _, shape := range []string{"multiple", "unsupported"} {
		t.Run(shape, func(t *testing.T) {
			org := testDataOrg()
			testSeedApprovalMetadata(t, &org)
			state := org.Objects["ProcessDefinition"]
			first := state.Records["04a000000000001AAA"]
			first.Fields["GladeApprovalCriteria"] = storage.StringValue("TRUE")
			first.Fields["GladeApprovalRouting"] = storage.StringValue("owner")
			if shape == "unsupported" {
				first.Fields["GladeApprovalRouting"] = storage.StringValue("unsupported")
			}
			state.Records[first.ID] = first
			if shape == "multiple" {
				second := first.Clone()
				second.ID = "04a000000000002AAA"
				second.Fields["Id"] = storage.IDValue(second.ID)
				second.Fields["DeveloperName"] = storage.StringValue("Other")
				state.Records[second.ID] = second
			}
			org.Objects["ProcessDefinition"] = state
			machine := New(nil)
			machine.SetOrg(&org)
			request := Object("Approval.ProcessSubmitRequest")
			_, _, err := machine.approvalProcessDefinitionForRequest(request, "Account")
			if err == nil || !strings.Contains(err.Error(), "hosted approval engine routing") {
				t.Fatalf("%s routing error = %v", shape, err)
			}
		})
	}
}
