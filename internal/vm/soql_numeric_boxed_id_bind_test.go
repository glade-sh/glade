package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecSOQLNumericBoxedIDReferenceBinds(t *testing.T) {
	// Both forms identify the same record. Binding an Id to a reference
	// field must retain that identity instead of parsing its digits as a number.
	for _, tc := range []struct {
		name        string
		declaration string
		predicate   string
	}{
		{"primitive15", `Id wanted = '001000000000001';`, `= :wanted`},
		{"boxed15", `Id wanted = new Account(Id = '001000000000001').Id;`, `= :wanted`},
		{"boxed18", `Id wanted = new Account(Id = '001000000000001AAA').Id;`, `= :wanted`},
		{"boxed_list", `List<Id> wanted = new List<Id>{new Account(Id = '001000000000001').Id};`, `IN :wanted`},
		{"boxed_set", `Set<Id> wanted = new Set<Id>{new Account(Id = '001000000000001').Id};`, `IN :wanted`},
		{"queried15", `Id wanted = [SELECT ParentId FROM Account WHERE Name = 'Meadow holder'].ParentId;`, `= :wanted`},
		{"queried_set", `Set<Id> wanted = new Set<Id>(); for (Account row : [SELECT ParentId FROM Account WHERE Name = 'Meadow holder']) wanted.add(row.ParentId);`, `IN :wanted`},
		{"nonnumeric", `Id wanted = new Account(Id = '00100000000000B').Id;`, `= :wanted`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			account := org.Objects["Account"]
			parentID := storage.ID("001000000000001AAA")
			if tc.name == "nonnumeric" {
				parentID = "00100000000000BAAQ"
			}
			account.Records[parentID] = storage.Record{ID: parentID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Meadow parent")}}
			account.Records["001000000000002"] = storage.Record{ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Meadow child"), "ParentId": storage.IDValue(parentID)}}
			account.Records["001000000000003"] = storage.Record{ID: "001000000000003", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Meadow holder"), "ParentId": storage.IDValue(parentID[:15])}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous(tc.declaration + `System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'Meadow child' AND ParentId ` + tc.predicate + `]);`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSOQLNumericScalarLiteralControls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value Value
		want  string
	}{
		{"integer", Int(42), "42"},
		{"decimal", Decimal(42.5), "42.5"},
		{"primitive_id", Value{Kind: ValueString, Type: "Id", Text: "001000000000001"}, "'001000000000001'"},
		{"numeric_boxed_id", platformScalar("Id", "001000000000001"), "'001000000000001'"},
		{"nonnumeric_boxed_id", platformScalar("Id", "00100000000000B"), "00100000000000B"},
		{"ordinary_text", String("001000000000001"), "'001000000000001'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := soqlLiteral(tc.value); got != tc.want {
				t.Fatalf("literal = %q, want %q", got, tc.want)
			}
		})
	}
}
