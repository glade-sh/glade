package visualforce

import (
	"strings"
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

	diagnostics := applyFormValues(controller, map[string]string{
		"isActive": "true",
		"quantity": "7",
		"amount":   "12.50",
		"dueDate":  "2026-06-14",
		"name":     "posted",
	}, nil)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}

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

	diagnostics := applyFormValues(controller, map[string]string{"quantity": "not-a-number"}, nil)

	if got := controller.Fields["quantity"]; got.Kind != vm.ValueString || got.Text != "not-a-number" {
		t.Fatalf("quantity = %#v, want conservative string fallback", got)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "quantity") || !strings.Contains(diagnostics[0], "INTEGER") {
		t.Fatalf("diagnostics = %#v, want quantity integer conversion message", diagnostics)
	}
}

func TestVisualforceTypedFormValueConvertsMultiSelectAndDatetimeLocal(t *testing.T) {
	existing := vm.List(vm.String("old"))
	got := visualforceTypedFormValue("Retail; Services ;", existing, nil)
	if got.Kind != vm.ValueList || len(got.List) != 2 || got.List[0].Text != "Retail" || got.List[1].Text != "Services" {
		t.Fatalf("multi-select list = %#v, want Retail and Services", got)
	}

	field := storage.Field{APIName: "Segments__c", Type: storage.FieldMultiPicklist}
	got = visualforceTypedFormValue("Retail;Services", vm.Null, &field)
	if got.Kind != vm.ValueString || got.Text != "Retail;Services" {
		t.Fatalf("multi-picklist field = %#v, want semicolon storage text", got)
	}

	field = storage.Field{APIName: "StartsAt__c", Type: storage.FieldDateTime}
	got = visualforceTypedFormValue("2026-06-14T09:30", vm.Null, &field)
	if got.Kind != vm.ValueObject || got.Type != "Datetime" || got.String() != "2026-06-14T09:30" {
		t.Fatalf("datetime-local = %#v, want Datetime scalar", got)
	}
}

func TestVisualforceTypedFormValueReturnsFieldConversionDiagnostic(t *testing.T) {
	field := storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	got, diagnostic := visualforceTypedFormValueWithDiagnostic("not-a-decimal", vm.Null, &field, "Account.Amount__c")

	if got.Kind != vm.ValueString || got.Text != "not-a-decimal" {
		t.Fatalf("value = %#v, want conservative string fallback", got)
	}
	if diagnostic == nil {
		t.Fatal("diagnostic = nil, want conversion diagnostic")
	}
	for _, want := range []string{"Account.Amount__c", "DECIMAL", "not-a-decimal"} {
		if !strings.Contains(diagnostic.Message, want) {
			t.Fatalf("diagnostic %q missing %q", diagnostic.Message, want)
		}
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
			"StartsAt__c": {APIName: "StartsAt__c", Type: storage.FieldDateTime},
			"Segments__c": {APIName: "Segments__c", Type: storage.FieldMultiPicklist},
		},
	}}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	record := vm.Object("Account")
	record.Fields["ExistingFlag__c"] = vm.Bool(false)
	controller := vm.Object("ApexPages.StandardController")
	controller.Fields["record"] = record

	diagnostics := applyStandardControllerFormValues(&controller, map[string]string{
		"ExistingFlag__c": "true",
		"IsActive__c":     "true",
		"Count__c":        "9",
		"Amount__c":       "101.25",
		"CloseDate":       "2026-06-14",
		"StartsAt__c":     "2026-06-14T09:30",
		"Segments__c":     "Retail;Services",
	}, nil, machine)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}

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
	if got := record.Fields["StartsAt__c"]; got.Kind != vm.ValueObject || got.Type != "Datetime" || got.String() != "2026-06-14T09:30" {
		t.Fatalf("StartsAt__c = %#v, want Datetime 2026-06-14T09:30 from schema", got)
	}
	if got := record.Fields["Segments__c"]; got.Kind != vm.ValueString || got.Text != "Retail;Services" {
		t.Fatalf("Segments__c = %#v, want semicolon storage text from schema", got)
	}
}
