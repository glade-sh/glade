package visualforce

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceFormBindingsForFieldsRejectsUnrenderedSubmittedFields(t *testing.T) {
	markup := `<apex:page>
  <apex:form>
    <apex:inputText value="{!allowed}"/>
    <apex:inputField value="{!Account.Name}"/>
    <apex:inputFile id="upload" value="{!body}" fileName="{!fileName}" contentType="{!mimeType}" fileSize="{!byteCount}"/>
  </apex:form>
</apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	allowed := VisualforceFormFieldNames(tree)
	bindings := VisualforceFormBindingsForFields(map[string]string{
		"allowed":  "yes",
		"Name":     "Acme",
		"body":     "bytes",
		"fileName": "invoice.txt",
		"hidden":   "no",
	}, allowed)

	got := map[string]string{}
	for _, binding := range bindings {
		got[binding.FieldName] = binding.Value
	}
	for field, want := range map[string]string{
		"allowed":  "yes",
		"Name":     "Acme",
		"body":     "bytes",
		"fileName": "invoice.txt",
	} {
		if got[field] != want {
			t.Fatalf("field %s = %q, want %q in %#v", field, got[field], want, got)
		}
	}
	if _, ok := got["hidden"]; ok {
		t.Fatalf("unrendered field was bound: %#v", got)
	}
}

func TestVisualforceFormBindingsForFieldsRejectsAllFieldsWhenPageHasNoInputs(t *testing.T) {
	tree, err := ParseMarkupTree(`<apex:page><apex:form><apex:outputText value="{!name}"/></apex:form></apex:page>`)
	if err != nil {
		t.Fatal(err)
	}
	bindings := VisualforceFormBindingsForFields(map[string]string{"name": "posted"}, VisualforceFormFieldNames(tree))
	if len(bindings) != 0 {
		t.Fatalf("bindings = %#v, want none", bindings)
	}
}

func TestApplyFormValuesConvertsFromExistingControllerFieldTypes(t *testing.T) {
	controller := vm.Object("EditController")
	controller.Fields["isActive"] = vm.Bool(false)
	controller.Fields["quantity"] = vm.Int(1)
	controller.Fields["amount"] = vm.Decimal(1.25)
	controller.Fields["dueDate"] = vmPlatformScalar("Date", "2024-01-01")
	controller.Fields["name"] = vm.String("old")

	applyFormValues(controller, map[string]string{
		"isActive": "true",
		"quantity": "7",
		"amount":   "12.50",
		"dueDate":  "2026-06-14",
		"name":     "posted",
	}, nil)

	if got := controller.Fields["isActive"]; got.Kind != vm.ValueBool || !got.Bool {
		t.Fatalf("isActive = %#v, want true bool", got)
	}
	if got := controller.Fields["quantity"]; got.Kind != vm.ValueInt || got.Int != 7 {
		t.Fatalf("quantity = %#v, want int 7", got)
	}
	if got := controller.Fields["amount"]; got.Kind != vm.ValueDecimal || got.String() != "12.50" {
		t.Fatalf("amount = %#v, want decimal 12.50", got)
	}
	if got := controller.Fields["dueDate"]; got.Kind != vm.ValueObject || got.Type != "Date" || got.String() != "2026-06-14" {
		t.Fatalf("dueDate = %#v, want Date 2026-06-14", got)
	}
	if got := controller.Fields["name"]; got.Kind != vm.ValueString || got.Text != "posted" {
		t.Fatalf("name = %#v, want posted string", got)
	}
}

func TestApplyFormValuesLeavesInvalidTypedPostsAsStrings(t *testing.T) {
	controller := vm.Object("EditController")
	controller.Fields["quantity"] = vm.Int(1)

	applyFormValues(controller, map[string]string{"quantity": "not-a-number"}, nil)

	if got := controller.Fields["quantity"]; got.Kind != vm.ValueString || got.Text != "not-a-number" {
		t.Fatalf("quantity = %#v, want conservative string fallback", got)
	}
}

func TestApplyStandardControllerFormValuesUsesRecordValuesAndSchema(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Account",
		Fields: map[string]storage.Field{
			"IsActive__c": {APIName: "IsActive__c", Type: storage.FieldBoolean},
			"Count__c":    {APIName: "Count__c", Type: storage.FieldInteger},
			"Amount__c":   {APIName: "Amount__c", Type: storage.FieldDecimal},
			"CloseDate":   {APIName: "CloseDate", Type: storage.FieldDate},
		},
	}}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	record := vm.Object("Account")
	record.Fields["ExistingFlag__c"] = vm.Bool(false)
	controller := vm.Object("ApexPages.StandardController")
	controller.Fields["record"] = record

	applyStandardControllerFormValues(&controller, map[string]string{
		"ExistingFlag__c": "true",
		"IsActive__c":     "true",
		"Count__c":        "9",
		"Amount__c":       "101.25",
		"CloseDate":       "2026-06-14",
	}, nil, machine)

	record = controller.Fields["record"]
	if got := record.Fields["ExistingFlag__c"]; got.Kind != vm.ValueBool || !got.Bool {
		t.Fatalf("ExistingFlag__c = %#v, want true bool", got)
	}
	if got := record.Fields["IsActive__c"]; got.Kind != vm.ValueBool || !got.Bool {
		t.Fatalf("IsActive__c = %#v, want true bool from schema", got)
	}
	if got := record.Fields["Count__c"]; got.Kind != vm.ValueInt || got.Int != 9 {
		t.Fatalf("Count__c = %#v, want int 9 from schema", got)
	}
	if got := record.Fields["Amount__c"]; got.Kind != vm.ValueDecimal || got.String() != "101.25" {
		t.Fatalf("Amount__c = %#v, want decimal 101.25 from schema", got)
	}
	if got := record.Fields["CloseDate"]; got.Kind != vm.ValueObject || got.Type != "Date" || got.String() != "2026-06-14" {
		t.Fatalf("CloseDate = %#v, want Date 2026-06-14 from schema", got)
	}
}
