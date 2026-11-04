package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCase349BusinessTypeIgnoresPersonSignals(t *testing.T) {
	id := storage.ID("012000000000101")
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"IsPersonAccount": {APIName: "IsPersonAccount", Type: storage.FieldBoolean}}, RecordTypes: []storage.RecordTypeInfo{{ID: id, DeveloperName: "Synthetic"}}}
	for _, field := range []string{"FirstName", "LastName"} {
		t.Run(field, func(t *testing.T) {
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), field: storage.StringValue("Synthetic")}}
			normalizePersonAccountFields("Account", def, &record)
			if value, ok := record.GetField("IsPersonAccount"); ok && value.Kind == storage.ValueBoolean && value.Boolean {
				t.Error("explicit business family changed to person")
			}
		})
	}
}

func TestCase349BusinessTypeNotOverriddenByStoredPersonFlag(t *testing.T) {
	id := storage.ID("012000000000101")
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"IsPersonAccount": {APIName: "IsPersonAccount", Type: storage.FieldBoolean}, "PersonHomePhone": {APIName: "PersonHomePhone", Type: storage.FieldString}}, RecordTypes: []storage.RecordTypeInfo{{ID: id, DeveloperName: "Synthetic"}}}
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		t.Run(field, func(t *testing.T) {
			current := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), field: storage.StringValue("4155550123")}}
			existing := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), "IsPersonAccount": storage.BooleanValue(true)}}
			if err := validateBusinessAccountContacts("Account", def, current, existing); err == nil {
				t.Error("stored value overrode explicit business metadata")
			}
		})
	}
}

func TestCase349SparseBusinessTypeNotOverriddenByStoredPersonFlag(t *testing.T) {
	id := storage.ID("012000000000101")
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"IsPersonAccount": {APIName: "IsPersonAccount", Type: storage.FieldBoolean}}, RecordTypes: []storage.RecordTypeInfo{{ID: id, DeveloperName: "Synthetic"}}}
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		t.Run(field, func(t *testing.T) {
			current := storage.Record{Object: "Account", Fields: map[string]storage.Value{field: storage.StringValue("4155550123")}}
			existing := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(id), "IsPersonAccount": storage.BooleanValue(true)}}
			if err := validateBusinessAccountContacts("Account", def, current, existing); err == nil {
				t.Error("sparse update used a legacy family override")
			}
		})
	}
}

func TestCase349SelectedPersonTypeOverridesExistingBusiness(t *testing.T) {
	business := storage.ID("012000000000101")
	person := storage.ID("012000000000102")
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"IsPersonAccount": {APIName: "IsPersonAccount", Type: storage.FieldBoolean}}, RecordTypes: []storage.RecordTypeInfo{{ID: business, DeveloperName: "Business"}, {ID: person, DeveloperName: "Person", PersonAccount: true}}}
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		t.Run(field, func(t *testing.T) {
			current := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(person), field: storage.StringValue("4155550123")}}
			existing := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(business), "IsPersonAccount": storage.BooleanValue(false)}}
			if err := validateBusinessAccountContacts("Account", def, current, existing); err != nil {
				t.Errorf("explicit person type ignored: %v", err)
			}
		})
	}
}
