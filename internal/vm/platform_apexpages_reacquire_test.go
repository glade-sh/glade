package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// VF59.1, retained API-67 StandardController reset/addFields source, standard-controller-call profile.
func TestStandardControllerResetAddFieldsReacquiresStoredFieldAPI67(t *testing.T) {
	const accountID = storage.ID("001000000000001AAA")
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":        {APIName: "Name", Type: storage.FieldString},
			"Rating":      {APIName: "Rating", Type: storage.FieldString},
			"Phone":       {APIName: "Phone", Type: storage.FieldString},
			"Description": {APIName: "Description", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{accountID: {
			ID: accountID, Object: "Account", Fields: map[string]storage.Value{
				"Name":   storage.StringValue("Persisted"),
				"Rating": storage.StringValue("Hot"),
				"Phone":  storage.StringValue("555-0100"),
			},
		}},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "67.0"}
	partial := Object("Account")
	partial.Fields["Id"] = platformScalar("Id", string(accountID))
	partial.Fields["Name"] = String("Persisted")
	partial.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Account", map[string]bool{"name": true})
	cleanPartial := cloneValue(partial)
	controller := Object("ApexPages.StandardController")
	controller.Fields["record"] = partial

	call := func(method string, args ...Value) Value {
		t.Helper()
		pending := controller.Fields["__glade_add_fields_pending"].Bool
		value, next, changed, handled, err := machine.callStandardControllerMember(controller, method, args, &Result{})
		if err != nil || !handled {
			t.Fatalf("%s: handled=%t err=%v", method, handled, err)
		}
		if method == "getRecord" && pending && !changed {
			t.Fatal("getRecord did not request controller receiver writeback after reacquisition")
		}
		controller = next
		return value
	}
	before := call("getRecord")
	if _, _, loaded := objectFieldValue(before, "Rating"); loaded {
		t.Fatal("Rating was present before addFields; fixture cannot prove reacquisition")
	}
	changed := controller.Fields["record"]
	changed.Fields["Name"] = String("UNSAVED")
	controller.Fields["record"] = changed
	call("reset")
	call("addFields", List(String("Rating")))
	after := call("getRecord")
	if _, name, ok := objectFieldValue(after, "Name"); !ok || name.Text != "Persisted" {
		t.Fatalf("Name after reset = %#v, present=%t; want persisted value", name, ok)
	}
	if _, rating, ok := objectFieldValue(after, "Rating"); !ok || rating.Text != "Hot" {
		t.Fatalf("Rating after reset/addFields = %#v, present=%t; want stored Hot", rating, ok)
	}
	if !machine.queriedSObjectFieldsIncludes(after, "Rating") || machine.queriedSObjectFieldsIncludes(after, "Phone") {
		t.Errorf("field visibility after addFields: Rating=%t Phone=%t; want true,false",
			machine.queriedSObjectFieldsIncludes(after, "Rating"), machine.queriedSObjectFieldsIncludes(after, "Phone"))
	}
	if _, _, loaded := objectFieldValue(after, "Phone"); loaded {
		t.Fatal("unrequested Phone was loaded")
	}
	after.Fields["Rating"] = String("Warm")
	controller.Fields["record"] = after
	again := call("getRecord")
	if _, rating, ok := objectFieldValue(again, "Rating"); !ok || rating.Text != "Warm" {
		t.Fatalf("repeated getRecord Rating = %#v, present=%t; want in-memory Warm", rating, ok)
	}
	if got := org.Objects["Account"].Records[accountID].Fields["Name"].String; got != "Persisted" {
		t.Fatalf("reset mutated stored Name to %q", got)
	}

	// An edit between constructor addFields and the first getRecord must not
	// become the clean baseline used by a later reset.
	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanPartial)
	call("addFields", List(String("Rating")))
	edited := controller.Fields["record"]
	edited.Fields["Name"] = String("UNSAVED-BEFORE-LOAD")
	controller.Fields["record"] = edited
	call("getRecord")
	call("reset")
	clean := call("getRecord")
	if _, name, _ := objectFieldValue(clean, "Name"); name.Text != "Persisted" {
		t.Errorf("reset Name after pre-load edit = %q; want clean Persisted", name.Text)
	}

	// Each addFields call adds references; a later call must not erase an
	// earlier pending request.
	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanPartial)
	call("addFields", List(String("Rating")))
	call("addFields", List(String("Phone")))
	both := call("getRecord")
	if _, rating, ok := objectFieldValue(both, "Rating"); !ok || rating.Text != "Hot" {
		t.Errorf("first addFields Rating after second call = %#v, present=%t", rating, ok)
	}
	if _, phone, ok := objectFieldValue(both, "Phone"); !ok || phone.Text != "555-0100" {
		t.Errorf("second addFields Phone = %#v, present=%t", phone, ok)
	}

	// A caller's explicit null is an in-memory edit, not a missing projection.
	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanPartial)
	call("addFields", List(String("Rating")))
	edited = controller.Fields["record"]
	setExplicitSObjectField(&edited, "Rating", Null)
	controller.Fields["record"] = edited
	withNull := call("getRecord")
	if rating, ok := standardControllerFieldPathValue(withNull, "Rating"); !ok || rating.Kind != ValueNull {
		t.Errorf("explicit null Rating = %#v, present=%t; want preserved null", rating, ok)
	}
	call("reset")
	clean = call("getRecord")
	if _, rating, ok := objectFieldValue(clean, "Rating"); !ok || rating.Text != "Hot" {
		t.Errorf("reset after explicit null Rating = %#v, present=%t; want stored Hot", rating, ok)
	}

	// A failed projection must leave the pending request and record untouched.
	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanPartial)
	call("addFields", List(String("Rating"), String("Missing_Field__c")))
	_, next, didChange, handled, err := machine.callStandardControllerMember(controller, "getRecord", nil, &Result{})
	if err == nil || !handled || didChange {
		t.Errorf("invalid projection: handled=%t changed=%t err=%v; want handled error without writeback", handled, didChange, err)
	}
	if _, _, ok := objectFieldValue(next.Fields["record"], "Rating"); ok || !next.Fields["__glade_add_fields_pending"].Bool {
		t.Error("failed projection loaded a field or cleared pending state")
	}

	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanPartial)
	call("addFields", List(String("Description")))
	absent := call("getRecord")
	value, handled, getErr := machine.callSObjectMember(absent, "get", []Value{String("Description")})
	if getErr != nil || !handled || value.Kind != ValueNull {
		t.Errorf("queried absent Description: value=%#v handled=%t err=%v; want visible null", value, handled, getErr)
	}
}

func TestStandardControllerAddFieldsRejectsDeniedProjectionAPI67(t *testing.T) {
	const accountID = storage.ID("001000000000001")
	org := stripInaccessibleTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldString}
	account.Records[accountID] = storage.Record{
		ID: accountID, Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme"), "Secret__c": storage.StringValue("Hidden")},
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "67.0"}
	machine.executionUser = stripInaccessibleTestUser()
	partial := Object("Account")
	partial.Fields["Id"] = platformScalar("Id", string(accountID))
	partial.Fields["Name"] = String("Acme")
	controller := Object("ApexPages.StandardController")
	controller.Fields["record"] = partial
	_, controller, _, handled, err := machine.callStandardControllerMember(controller, "addFields", []Value{List(String("Secret__c"))}, &Result{})
	if err != nil || !handled {
		t.Fatalf("addFields: handled=%t err=%v", handled, err)
	}
	_, next, changed, handled, err := machine.callStandardControllerMember(controller, "getRecord", nil, &Result{})
	var thrown *apexThrowError
	if !handled || changed || !errors.As(err, &thrown) {
		t.Fatalf("denied field: handled=%t changed=%t err=%v; want catchable QueryException", handled, changed, err)
	}
	if _, _, present := objectFieldValue(next.Fields["record"], "Secret__c"); present || !next.Fields["__glade_add_fields_pending"].Bool {
		t.Error("denied projection changed record or pending state")
	}

	account = org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	row := account.Records[accountID]
	row.System.OwnerID = "005000000000999"
	account.Records[accountID] = row
	org.Objects["Account"] = account
	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(partial)
	_, controller, _, _, err = machine.callStandardControllerMember(controller, "addFields", []Value{List(String("Name"))}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	_, next, changed, handled, err = machine.callStandardControllerMember(controller, "getRecord", nil, &Result{})
	if !handled || changed || !errors.As(err, &thrown) {
		t.Fatalf("private row: handled=%t changed=%t err=%v; want catchable QueryException", handled, changed, err)
	}
	if !next.Fields["__glade_add_fields_pending"].Bool {
		t.Error("private-row denial cleared pending state")
	}
}

func TestStandardControllerAddFieldsProjectionErrorRunsApexCatchFinallyAPI67(t *testing.T) {
	const accountID = storage.ID("001000000000001AAA")
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{accountID: {
			ID: accountID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
		}},
	}
	partial := Object("Account")
	partial.Fields["Id"] = platformScalar("Id", string(accountID))
	controller := Object("ApexPages.StandardController")
	controller.Fields["record"] = partial
	controller.Fields["fields"] = List(String("Missing_Field__c"))
	controller.Fields["__glade_add_fields_pending"] = Bool(true)
	program, err := CompileAnonymous(`
Boolean caught = false;
Boolean finalized = false;
try {
    controller.getRecord();
} catch (QueryException e) {
    caught = true;
} finally {
    finalized = true;
}
System.assert(caught, 'projection QueryException was not caught');
System.assert(finalized, 'finally was not executed');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "67.0"}
	machine.Globals["controller"] = controller
	machine.VarTypes["controller"] = "ApexPages.StandardController"
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	stored := machine.Globals["controller"]
	if _, _, present := objectFieldValue(stored.Fields["record"], "Missing_Field__c"); present || !stored.Fields["__glade_add_fields_pending"].Bool {
		t.Error("catchable projection failure changed controller state")
	}
}

func TestStandardControllerResetAddFieldsReacquiresRelationshipPathAPI67(t *testing.T) {
	const (
		contactID     = storage.ID("003000000000001AAA")
		nullContactID = storage.ID("003000000000002AAA")
		accountID     = storage.ID("001000000000001AAA")
	)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":  {APIName: "Name", Type: storage.FieldString},
			"Phone": {APIName: "Phone", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{accountID: {
			ID: accountID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent Name")},
		}},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
			Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account"}},
		},
		Records: map[storage.ID]storage.Record{contactID: {
			ID: contactID, Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(accountID)},
		}, nullContactID: {
			ID: nullContactID, Object: "Contact", Fields: map[string]storage.Value{},
		}},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "67.0"}
	partial := Object("Contact")
	partial.Fields["Id"] = platformScalar("Id", string(contactID))
	partial.Fields["AccountId"] = platformScalar("Id", string(accountID))
	partial.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Contact", map[string]bool{"accountid": true})
	cleanContact := cloneValue(partial)
	controller := Object("ApexPages.StandardController")
	controller.Fields["record"] = partial
	call := func(method string, args ...Value) Value {
		t.Helper()
		value, next, _, handled, err := machine.callStandardControllerMember(controller, method, args, &Result{})
		if err != nil || !handled {
			t.Fatalf("%s: handled=%t err=%v", method, handled, err)
		}
		controller = next
		return value
	}
	call("getRecord")
	call("reset")
	call("addFields", List(String("Account.Name")))
	after := call("getRecord")
	parent, ok := standardControllerFieldPathValue(after, "Account.Name")
	if !ok || parent.Text != "Parent Name" {
		t.Errorf("Contact.Account.Name = %#v, present=%t; want stored Parent Name", parent, ok)
	}
	if !machine.queriedSObjectFieldsIncludes(after, "Account") {
		t.Error("Contact.Account relationship was not marked query-visible")
	}

	controller = Object("ApexPages.StandardController")
	noParent := Object("Contact")
	noParent.Fields["Id"] = platformScalar("Id", string(nullContactID))
	noParent.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Contact", map[string]bool{"id": true})
	controller.Fields["record"] = noParent
	call("addFields", List(String("Account.Name")))
	after = call("getRecord")
	if !machine.queriedSObjectFieldsIncludes(after, "Account") {
		t.Error("null Contact.Account relationship was not marked query-visible")
	}
	if _, parent, ok := objectFieldValue(after, "Account"); !ok || parent.Kind != ValueNull {
		t.Errorf("null Contact.Account = %#v, present=%t; want loaded null", parent, ok)
	}

	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanContact)
	call("addFields", List(String("Account.Phone")))
	after = call("getRecord")
	parentRecord, ok := standardControllerFieldPathValue(after, "Account")
	if !ok || parentRecord.Kind != ValueObject || sObjectIDFromFields(parentRecord.Fields) != accountID {
		t.Errorf("parent identity after null leaf: kind=%s id=%s present=%t; want Account Id %s", parentRecord.Kind, sObjectIDFromFields(parentRecord.Fields), ok, accountID)
	}

	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanContact)
	call("addFields", List(String("Account.Name")))
	edited := controller.Fields["record"]
	setExplicitSObjectField(&edited, "Account", Null)
	controller.Fields["record"] = edited
	after = call("getRecord")
	if _, parent, ok := objectFieldValue(after, "Account"); !ok || parent.Kind != ValueNull {
		t.Errorf("explicit null parent after reacquisition: kind=%s present=%t", parent.Kind, ok)
	}
	call("reset")
	after = call("getRecord")
	if parent, ok := standardControllerFieldPathValue(after, "Account.Name"); !ok || parent.Text != "Parent Name" {
		t.Errorf("reset after explicit null parent = %#v, present=%t", parent, ok)
	}

	controller = Object("ApexPages.StandardController")
	preloaded := cloneValue(cleanContact)
	preloadedParent := Object("Account")
	preloadedParent.Fields["Id"] = platformScalar("Id", string(accountID))
	preloadedParent.Fields["Name"] = String("Parent Name")
	preloaded.Fields["Account"] = preloadedParent
	controller.Fields["record"] = preloaded
	call("addFields", List(String("Account.Phone")))
	after = call("getRecord")
	parentRecord, _ = standardControllerFieldPathValue(after, "Account")
	value, handled, getErr := machine.callSObjectMember(parentRecord, "get", []Value{String("Name")})
	if getErr != nil || !handled || value.Text != "Parent Name" {
		t.Errorf("preloaded parent Name after reacquisition: value=%#v handled=%t err=%v", value, handled, getErr)
	}

	// Do not mix the stored parent A's fields into an in-memory replacement B.
	controller = Object("ApexPages.StandardController")
	controller.Fields["record"] = cloneValue(cleanContact)
	call("addFields", List(String("Account.Name")))
	edited = controller.Fields["record"]
	replacement := Object("Account")
	replacement.Fields["Id"] = platformScalar("Id", "001000000000002AAA")
	edited.Fields["Account"] = replacement
	controller.Fields["record"] = edited
	after = call("getRecord")
	parentRecord, _ = standardControllerFieldPathValue(after, "Account")
	if got := sObjectIDFromFields(parentRecord.Fields); got != "001000000000002AAA" {
		t.Errorf("replacement Account Id = %s; want B", got)
	}
	if _, _, present := objectFieldValue(parentRecord, "Name"); present {
		t.Error("stored parent A's Name was merged into replacement B")
	}
	call("reset")
	after = call("getRecord")
	if parent, present := standardControllerFieldPathValue(after, "Account.Name"); !present || parent.Text != "Parent Name" {
		t.Errorf("reset after replacement parent = %#v, present=%t; want stored A", parent, present)
	}

	// Marker creation cannot turn an unqueried null placeholder into a loaded
	// field merely because an earlier requested field was processed first.
	account := org.Objects["Account"]
	row := account.Records[accountID]
	row.Fields["Phone"] = storage.StringValue("555-0100")
	account.Records[accountID] = row
	org.Objects["Account"] = account
	for _, names := range [][]string{{"Account.Name", "Account.Phone"}, {"Account.Phone", "Account.Name"}} {
		controller = Object("ApexPages.StandardController")
		preloaded = cloneValue(cleanContact)
		preloadedParent = Object("Account")
		preloadedParent.Fields["Id"] = platformScalar("Id", string(accountID))
		preloadedParent.Fields["Name"] = String("Parent Name")
		preloadedParent.Fields["Phone"] = Null
		preloaded.Fields["Account"] = preloadedParent
		controller.Fields["record"] = preloaded
		call("addFields", List(String(names[0]), String(names[1])))
		after = call("getRecord")
		if phone, present := standardControllerFieldPathValue(after, "Account.Phone"); !present || phone.Text != "555-0100" {
			t.Errorf("fields %v: Account.Phone = %#v, present=%t; want stored phone", names, phone, present)
		}
		call("reset")
		after = call("getRecord")
		if phone, present := standardControllerFieldPathValue(after, "Account.Phone"); !present || phone.Text != "555-0100" {
			t.Errorf("fields %v after reset: Account.Phone = %#v, present=%t", names, phone, present)
		}
	}
}
