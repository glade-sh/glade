package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Runtime counterparts of the native STANDARD/CUSTOM rows. Construct values
// directly so this checks VM typing independently of semantic analysis and DML.
func TestAddressLocationRuntimeTyping(t *testing.T) {
	for _, addressType := range []string{"Address", "System.Address"} {
		machine := New(nil)
		address := Object(addressType)
		address.Runtime = "System.Address"
		address.Fields["latitude"] = Decimal(12.5)
		address.Fields["longitude"] = Decimal(25)
		for _, target := range []string{"Location", "System.Location"} {
			t.Run(addressType+"/"+target, func(t *testing.T) {
				if _, err := machine.coerceAssignable(target, address); err != nil {
					t.Fatal(err)
				}
				if !machine.evalInstanceOf(address, target).Bool {
					t.Fatal("Address must be a Location")
				}
				if _, err := machine.coerceCast(target, address); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestAddressLocationRuntimeIdentityBoundaries(t *testing.T) {
	for _, shadow := range []string{"Address", "Location"} {
		machine := New(nil)
		if err := machine.RegisterClass(Class{Name: shadow}); err != nil {
			t.Fatal(err)
		}
		if machine.typeAssignableTo("Address", "Location") {
			t.Fatal("project class gained platform inheritance")
		}
		if !machine.typeAssignableTo("System.Address", "System.Location") {
			t.Fatal("qualified platform types were shadowed")
		}
		from, to := "Address", "System.Location"
		if shadow == "Location" {
			from, to = "System.Address", "Location"
		}
		if machine.typeAssignableTo(from, to) || machine.typeMatches(from, to, make(map[string]bool)) {
			t.Fatalf("shadow gained platform subtype: %s -> %s", from, to)
		}
	}
	machine := New(nil)
	for _, pair := range [][2]string{{"Schema.Address", "System.Location"}, {"System.Address", "Schema.Location"}, {"System.Location", "System.Address"}} {
		if machine.typeAssignableTo(pair[0], pair[1]) {
			t.Fatalf("invalid subtype edge %v", pair)
		}
	}
}

// Native: internal/apextest/testdata/conformance/address_location/org.tsv,
// APIs 62.0/67.0, CUSTOM, PASS (anonymous and @IsTest). These rows cover a
// queried custom compound with populated coordinates and its component schema.
// The custom Address rule (P02) resolves components through their
// CompoundFieldName links, which project schema loading always sets, so this
// test uses linked component definitions only.
func TestAddressLocationCustomCompound(t *testing.T) {
	for _, components := range []bool{true} {
		org := testDataOrg()
		account := org.Objects["Account"]
		account.Definition.Fields["Site__c"] = storage.Field{APIName: "Site__c", Type: storage.FieldAddress}
		account.Definition.Fields["Text__c"] = storage.Field{APIName: "Text__c", Type: storage.FieldString}
		for _, name := range []string{"Site__Latitude__s", "Site__Longitude__s"} {
			field := storage.Field{APIName: name, Type: storage.FieldDecimal}
			if components {
				field.CompoundFieldName = "Site__c"
			}
			account.Definition.Fields[name] = field
		}
		org.Objects["Account"] = account
		machine := New(nil)
		machine.SetOrg(&org)
		program, err := CompileAnonymous(`
Account r = new Account(Name='Custom', Site__Latitude__s=12.5, Site__Longitude__s=25.0);
insert r;
r = [SELECT Site__c FROM Account WHERE Id=:r.Id];
Address a = r.Site__c;
Location l = a;
System.assertEquals(12.5, a.getLatitude());
System.assertEquals(25.0, a.getLongitude());
System.assertEquals(0.0, l.getDistance(a, 'mi'));
Object o = a;
System.assert(o instanceof System.Location);
`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatalf("component definitions=%v: %v", components, err)
		}
		// Component-shaped fields cannot turn an unrelated custom field into Address.
		receiver := Object("Account")
		receiver.Fields["Text__Latitude__s"] = Decimal(12.5)
		for _, name := range []string{"Text__c", "Missing__c"} {
			if _, ok := machine.sObjectCompoundAddressValue(receiver, name); ok {
				t.Fatalf("%s was inferred to be Address", name)
			}
		}
	}
}
