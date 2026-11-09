package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestApprovalOwnerWorkitemActorBoundary(t *testing.T) {
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	id := storage.ID("001000000000101AAA")
	testSeedApprovalAccount(t, &org, id, "Probe")
	accounts := org.Objects["Account"]
	account := accounts.Records[id]
	account.System.OwnerID = storage.DefaultUserID
	accounts.Records[id] = account
	org.Objects["Account"] = accounts
	defs := org.Objects["ProcessDefinition"]
	definition := defs.Records["04a000000000001AAA"]
	definition.Fields["GladeApprovalRouting"] = storage.StringValue("owner")
	defs.Records[definition.ID] = definition
	org.Objects["ProcessDefinition"] = defs
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: storage.DefaultUserID, Object: "User"})
	submit := Object("Approval.ProcessSubmitRequest")
	submit.Fields["ObjectId"] = platformScalar("Id", string(id))
	result, err := machine.executeApprovalProcess([]Value{submit, Bool(false)})
	if err != nil {
		t.Fatal(err)
	}
	workIDText, ok := idValueText(result.Fields["newWorkitemIds"].List[0])
	if !ok {
		t.Fatal("missing workitem id")
	}
	workID := storage.ID(workIDText)
	items := machine.Org.Objects["ProcessInstanceWorkitem"]
	item := items.Records[workID]
	item.Fields["ActorId"] = storage.IDValue("005000000000002AAA")
	items.Records[workID] = item
	machine.Org.Objects["ProcessInstanceWorkitem"] = items
	_, err = machine.approvalApplyWorkitemAction(workID, "Approve", "", false)
	if err == nil || !strings.Contains(err.Error(), "hosted approval engine routing") {
		t.Fatalf("foreign actor error = %v", err)
	}
}
