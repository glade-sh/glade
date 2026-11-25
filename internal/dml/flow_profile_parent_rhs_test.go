package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The controls use a resolvable schema and parent, so they fail if an
// uncaptured shape starts using the new comparison.
func TestFlowProfileParentRHSUncapturedRemainUnsupported(t *testing.T) {
	org := storage.NewOrgState()
	const profileID storage.ID = "00e000000000001AAA"
	const accountID storage.ID = "001000000000001AAA"
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{storage.DefaultUserID: {ID: storage.DefaultUserID, Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profileID)}}}}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{profileID: {ID: profileID, Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}}}}
	account, _ := storage.StandardObjectDefinition("Account")
	contact, _ := storage.StandardObjectDefinition("Contact")
	org.Objects["Account"] = storage.ObjectState{Definition: account, Records: map[storage.ID]storage.Record{accountID: {ID: accountID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator"), "ParentId": storage.IDValue(accountID)}}}}
	source := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(accountID)}}
	e := NewEngine(&org)
	item := storage.WorkflowCriteriaItem{Field: "$Profile.Name", Operation: "equals", SourceField: "$Record.Account.Name"}
	if match, supported, handled := e.flowAfterCreateProfileRecordCriterion(item, source, contact, true); !match || !supported || !handled {
		t.Fatalf("positive control: %v %v %v", match, supported, handled)
	}
	for _, tc := range []struct {
		name, op, ref string
		allow         bool
	}{
		{"other trigger", "equals", "$Record.Account.Name", false},
		{"not equal", "notEqual", "$Record.Account.Name", true},
		{"two parents", "equals", "$Record.Account.Parent.Name", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item.Operation, item.SourceField = tc.op, tc.ref
			_, _, handled := e.flowAfterCreateProfileRecordCriterion(item, source, contact, tc.allow)
			if handled {
				t.Fatal("uncaptured criterion became supported")
			}
		})
	}
	t.Run("polymorphic", func(t *testing.T) {
		d := contact
		d.Fields = make(map[string]storage.Field, len(contact.Fields))
		for k, v := range contact.Fields {
			d.Fields[k] = v
		}
		f := d.Fields["AccountId"]
		f.ReferenceTo = []string{"Account", "Contact"}
		d.Fields["AccountId"] = f
		item.Operation, item.SourceField = "equals", "$Record.Account.Name"
		_, _, handled := e.flowAfterCreateProfileRecordCriterion(item, source, d, true)
		if handled {
			t.Fatal("polymorphic relationship became supported")
		}
	})
	t.Run("case difference", func(t *testing.T) {
		a := org.Objects["Account"]
		r := a.Records[accountID]
		r.Fields["Name"] = storage.StringValue("system administrator")
		a.Records[accountID] = r
		item.Operation, item.SourceField = "equals", "$Record.Account.Name"
		_, _, handled := e.flowAfterCreateProfileRecordCriterion(item, source, contact, true)
		if handled {
			t.Fatal("uncaptured case difference became supported")
		}
	})
}
