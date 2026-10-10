package server

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestObjectMetadataNameLayoutUsesSchemaMembership(t *testing.T) {
	// r_layout_Contact_Full_View captures this order. These mutations check
	// schema integrity without claiming additional native layout observations.
	def := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"Name":       {Type: storage.FieldString},
		"Salutation": {CompoundFieldName: "Name"},
		"FirstName":  {CompoundFieldName: "Name"},
		"LastName":   {CompoundFieldName: "Name"},
	}}
	want := []string{"Salutation", "FirstName", "LastName"}
	if got := objectMetadataLayoutComponentNames(def, "Name"); !reflect.DeepEqual(got, want) {
		t.Fatalf("captured presentation: got %v, want %v", got, want)
	}
	for _, unbound := range want {
		def.Fields[unbound] = storage.Field{CompoundFieldName: "Different__c"}
		expected := []string{}
		for _, name := range want {
			if name != unbound {
				expected = append(expected, name)
			}
		}
		if got := objectMetadataLayoutComponentNames(def, "Name"); !reflect.DeepEqual(got, expected) {
			t.Errorf("unbound %s: got %v, want %v", unbound, got, expected)
		}
		def.Fields[unbound] = storage.Field{CompoundFieldName: "Name"}
	}
	def.Fields = map[string]storage.Field{"Name": {Type: storage.FieldString}}
	if got := objectMetadataLayoutComponentNames(def, "Name"); !reflect.DeepEqual(got, []string{"Name"}) {
		t.Fatalf("name components synthesized without metadata: %v", got)
	}
}

func TestObjectMetadataAddressLayoutUsesSchemaMembership(t *testing.T) {
	// The display order is captured by r_layout_Account_Full_Create; changing
	// the schema binding must exclude a field even when its name looks right.
	want := []string{"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry"}
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"BillingAddress":     {Type: storage.FieldAddress},
		"BillingLatitude":    {CompoundFieldName: "BillingAddress"},
		"BillingCountryCode": {CompoundFieldName: "BillingAddress"},
	}}
	for _, name := range want {
		def.Fields[name] = storage.Field{CompoundFieldName: "BillingAddress"}
	}
	if got := objectMetadataLayoutComponentNames(def, "BillingAddress"); !reflect.DeepEqual(got, want) {
		t.Fatalf("captured presentation: got %v, want %v", got, want)
	}
	def.Fields["BillingStreet"] = storage.Field{CompoundFieldName: "ShippingAddress"}
	if got := objectMetadataLayoutComponentNames(def, "BillingAddress"); !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("unrelated schema member included: got %v, want %v", got, want[1:])
	}
	delete(def.Fields, "BillingCity")
	if got := objectMetadataLayoutComponentNames(def, "BillingAddress"); !reflect.DeepEqual(got, want[2:]) {
		t.Fatalf("absent component synthesized: got %v, want %v", got, want[2:])
	}
}

func TestObjectMetadataAddressLayoutDoesNotRequireNameSuffix(t *testing.T) {
	// This is a schema-integrity control, not a claim of native custom layout parity.
	def := storage.ObjectDefinition{APIName: "Custom__c", Fields: map[string]storage.Field{
		"Residence__c":    {Type: storage.FieldAddress},
		"Postal__c":       {CompoundFieldName: "Residence__c"},
		"Line__c":         {CompoundFieldName: "Residence__c"},
		"ResidenceStreet": {CompoundFieldName: "Different__c"},
	}}
	want := []string{"Line__c", "Postal__c"}
	if got := objectMetadataLayoutComponentNames(def, "Residence__c"); !reflect.DeepEqual(got, want) {
		t.Fatalf("schema components: got %v, want %v", got, want)
	}
	def.Fields = map[string]storage.Field{"Residence__c": {Type: storage.FieldAddress}}
	if got := objectMetadataLayoutComponentNames(def, "Residence__c"); !reflect.DeepEqual(got, []string{"Residence__c"}) {
		t.Fatalf("components synthesized without metadata: %v", got)
	}
}
