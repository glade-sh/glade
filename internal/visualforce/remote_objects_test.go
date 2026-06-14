package visualforce

import (
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderRemoteObjectsAccountFixturePreservesVisibleContract(t *testing.T) {
	markup := `<apex:page><apex:remoteObjects><apex:remoteObjectModel name="Account" fields="Id,Name"/></apex:remoteObjects><div>Remote Objects Account</div></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	org := remoteObjectsTestOrg()
	runner := vm.New(nil)
	runner.Org = &org

	rendered, err := RenderMarkupTree(tree, &RenderContext{VM: runner, Expression: &ExpressionContext{VM: runner}})
	if err != nil {
		t.Fatalf("RenderMarkupTree err = %v", err)
	}
	assertContains(t, rendered, `<div>Remote Objects Account</div>`)
	assertContains(t, rendered, `var RemoteObjectModel = window.RemoteObjectModel || {};`)
	assertContains(t, rendered, `RemoteObjectModel.Account.fields = ["Id","Name"];`)
	if strings.Contains(rendered, "unsupported Visualforce component apex:remoteObjects") {
		t.Fatalf("rendered still contains unsupported diagnostic: %q", rendered)
	}
	if strings.Contains(strings.ToLower(rendered), "<apex:remoteobjectmodel") {
		t.Fatalf("rendered leaked declaration markup: %q", rendered)
	}
}

func TestRenderRemoteObjectsFieldsFixtureUsesNamespaceAndPreservesVisibleContract(t *testing.T) {
	markup := `<apex:page><apex:remoteObjects jsNamespace="ProbeRemote"><apex:remoteObjectModel name="Contact" fields="Id,FirstName,LastName"/></apex:remoteObjects><div>Remote Objects Fields</div></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}

	rendered, err := RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatalf("RenderMarkupTree err = %v", err)
	}
	assertContains(t, rendered, `<div>Remote Objects Fields</div>`)
	assertContains(t, rendered, `var ProbeRemote = window.ProbeRemote || {};`)
	assertContains(t, rendered, `ProbeRemote.Contact.fields = ["Id","FirstName","LastName"];`)
}

func TestRenderRemoteObjectsFallsBackToDeclarationWhenLocalSchemaIsIncomplete(t *testing.T) {
	markup := `<apex:page><apex:remoteObjects><apex:remoteObjectModel name="Account" fields="Id,Name"/></apex:remoteObjects><div>Remote Objects Account</div></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
	}
	runner := vm.New(nil)
	runner.Org = &org

	rendered, err := RenderMarkupTree(tree, &RenderContext{VM: runner, Expression: &ExpressionContext{VM: runner}})
	if err != nil {
		t.Fatalf("RenderMarkupTree err = %v", err)
	}
	assertContains(t, rendered, `<div>Remote Objects Account</div>`)
	assertContains(t, rendered, `RemoteObjectModel.Account.fields = ["Id","Name"];`)
}

func TestRemoteObjectsDescriptorGeneratesNamespaceAndModels(t *testing.T) {
	tree, err := ParseMarkupTree(`<apex:remoteObjects jsNamespace="Remote">
  <apex:remoteObjectModel name="Account" jsShorthand="Acct" fields="Name,Industry">
    <apex:remoteObjectField name="Rating" jsShorthand="rate"/>
  </apex:remoteObjectModel>
</apex:remoteObjects>`)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := BuildRemoteObjectsDescriptor(tree, RemoteObjectSchema{
		"Account": {"Name", "Industry", "Rating"},
	})
	if err != nil {
		t.Fatalf("BuildRemoteObjectsDescriptor err = %v", err)
	}
	if descriptor.Namespace != "Remote" || len(descriptor.Models) != 1 {
		t.Fatalf("descriptor = %#v", descriptor)
	}
	model := descriptor.Models[0]
	if model.Name != "Account" || model.JSName != "Acct" {
		t.Fatalf("model = %#v", model)
	}
	script := RenderRemoteObjectsScript(descriptor)
	assertContains(t, script, `var Remote = window.Remote || {};`)
	assertContains(t, script, `Remote.Acct = function(fields)`)
	assertContains(t, script, `Remote.Acct.fields = ["Name","Industry","Rating"];`)
	assertContains(t, script, `Remote.Acct.prototype.create`)
	assertContains(t, script, `Remote.Acct.prototype.retrieve`)
	assertContains(t, script, `Remote.Acct.prototype.update`)
	assertContains(t, script, `Remote.Acct.prototype.del`)
	assertContains(t, script, `Remote.Acct.describe = function(callback)`)
	assertContains(t, script, `Remote.Acct.query = function(criteria,callback)`)
	assertContains(t, script, `window.__gladeRemoteObjects=window.__gladeRemoteObjects||function`)
	assertContains(t, script, `+"/remoteObjects"`)
	assertContains(t, script, `viewState:read("com.salesforce.visualforce.ViewState")`)
	assertContains(t, script, `csrf:read("__vf_csrf")`)
}

func TestRemoteObjectsCRUDRoundTripUsesDeclaredLocalOrgState(t *testing.T) {
	descriptor := RemoteObjectsDescriptor{Models: []RemoteObjectModelDescriptor{{
		Name:   "Account",
		JSName: "Acct",
		Fields: []RemoteObjectFieldDescriptor{
			{Name: "Name", JSName: "Name"},
			{Name: "Industry", JSName: "Industry"},
		},
	}}}
	org := remoteObjectsTestOrg()

	created := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "create",
		ObjectName: "Account",
		Fields:     map[string]any{"Name": "Acme", "Industry": "Energy"},
	})
	if !created.Success || len(created.IDs) != 1 || !strings.HasPrefix(created.IDs[0], "001") {
		t.Fatalf("created = %#v", created)
	}

	updated := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "update",
		ObjectName: "Account",
		IDs:        created.IDs,
		Fields:     map[string]any{"Name": "Acme Updated"},
	})
	if !updated.Success {
		t.Fatalf("updated = %#v", updated)
	}

	retrieved := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "retrieve",
		ObjectName: "Account",
		IDs:        created.IDs,
	})
	if !retrieved.Success || len(retrieved.Records) != 1 || retrieved.Records[0]["Name"] != "Acme Updated" || retrieved.Records[0]["Industry"] != "Energy" {
		t.Fatalf("retrieved = %#v", retrieved)
	}

	deleted := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "delete",
		ObjectName: "Account",
		IDs:        created.IDs,
	})
	if !deleted.Success {
		t.Fatalf("deleted = %#v", deleted)
	}
	afterDelete := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "retrieve",
		ObjectName: "Account",
		IDs:        created.IDs,
	})
	if !afterDelete.Success || len(afterDelete.Records) != 0 {
		t.Fatalf("afterDelete = %#v", afterDelete)
	}
}

func TestRemoteObjectsDescribeAndQueryUseDeclaredLocalOrgState(t *testing.T) {
	descriptor := RemoteObjectsDescriptor{Models: []RemoteObjectModelDescriptor{{
		Name:   "Account",
		JSName: "Acct",
		Fields: []RemoteObjectFieldDescriptor{
			{Name: "Name", JSName: "Name"},
			{Name: "Industry", JSName: "Industry"},
		},
	}}}
	org := remoteObjectsTestOrg()
	created := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "create",
		ObjectName: "Account",
		Fields:     map[string]any{"Name": "Acme", "Industry": "Energy"},
	})
	if !created.Success {
		t.Fatalf("created = %#v", created)
	}

	describe := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "describe",
		ObjectName: "Acct",
	})
	if !describe.Success || describe.Describe == nil || describe.Describe.Name != "Account" || len(describe.Describe.Fields) != 2 {
		t.Fatalf("describe = %#v", describe)
	}

	query := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "query",
		ObjectName: "Account",
	})
	if !query.Success || len(query.Records) != 1 || query.Records[0]["Id"] != created.IDs[0] || query.Records[0]["Name"] != "Acme" {
		t.Fatalf("query = %#v", query)
	}

	filtered := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "query",
		ObjectName: "Account",
		Criteria:   map[string]any{"Name": "Acme"},
	})
	if filtered.Success || len(filtered.Errors) != 1 || filtered.Errors[0].StatusCode != "UNSUPPORTED_FEATURE" || !strings.Contains(filtered.Errors[0].Message, "only supports Id or ids") {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestRemoteObjectsUnsupportedOperationsReturnDiagnostics(t *testing.T) {
	descriptor := RemoteObjectsDescriptor{Models: []RemoteObjectModelDescriptor{{
		Name: "Account",
		Fields: []RemoteObjectFieldDescriptor{
			{Name: "Name", JSName: "Name"},
		},
	}}}
	org := remoteObjectsTestOrg()

	result := DispatchRemoteObjectCRUD(&org, descriptor, RemoteObjectCRUDRequest{
		Operation:  "merge",
		ObjectName: "Account",
	})
	if result.Success || len(result.Errors) != 1 || result.Errors[0].StatusCode != "UNSUPPORTED_FEATURE" || !strings.Contains(result.Errors[0].Message, "unsupported remote object operation merge") {
		t.Fatalf("result = %#v", result)
	}
}

func TestRemoteObjectsRejectsUndeclaredObjectsAndFields(t *testing.T) {
	tree, err := ParseMarkupTree(`<apex:remoteObjects>
  <apex:remoteObjectModel name="Account" fields="Name,Secret__c"/>
</apex:remoteObjects>`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildRemoteObjectsDescriptor(tree, RemoteObjectSchema{"Contact": {"Name"}})
	if err == nil || !strings.Contains(err.Error(), "undeclared remote object Account") {
		t.Fatalf("err = %v, want undeclared object diagnostic", err)
	}

	_, err = BuildRemoteObjectsDescriptor(tree, RemoteObjectSchema{"Account": {"Name"}})
	if err == nil || !strings.Contains(err.Error(), "undeclared remote field Account.Secret__c") {
		t.Fatalf("err = %v, want undeclared field diagnostic", err)
	}
}

func remoteObjectsTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Id":       {APIName: "Id", Type: storage.FieldID, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"Name":     {APIName: "Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
				"Industry": {APIName: "Industry", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	return org
}
