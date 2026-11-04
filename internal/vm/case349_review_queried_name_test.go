package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Queried Name rejects on person updates: case349_query_trigger/org.tsv:12-15,55-58.
func TestCase349QueriedPersonNameIsNotAnAssignment(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		t.Run(field, func(t *testing.T) {
			org := testDataOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			machine := New(nil)
			machine.SetOrg(&org)
			source := fmt.Sprintf(`Account seed=new Account(LastName='Synthetic'); insert seed;
Account a=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; a.%s='4155550123'; Database.SaveResult first=Database.update(a,false); System.assertEquals(false,first.isSuccess());
Account b=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; b.Name=b.Name; b.%s='4155550123'; Database.SaveResult second=Database.update(b,false); System.assertEquals(false,second.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(second.getErrors()[0].getStatusCode()));`, field, field)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The transient marker follows the queried value through the VM-to-DML bridge;
// an explicit same-value assignment must not carry it.
func TestCase349QueriedPersonNameCarriesDMLProvenance(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	storage.EnsureStandardObject(&org, "Contact")
	machine := New(nil)
	machine.SetOrg(&org)
	source := `Account seed=new Account(LastName='Synthetic'); insert seed; Account a=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; Account b=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; b.Name=b.Name;`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want bool
	}{{"a", true}, {"b", false}} {
		records, _, err := machine.recordsFromValue(machine.Globals[tc.name])
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 1 {
			t.Fatal(records)
		}
		marker, ok := any(records[0]).(interface{ DMLNameUnassigned() bool })
		if !ok || marker.DMLNameUnassigned() != tc.want {
			t.Errorf("%s: queried provenance = %v, want %v", tc.name, ok && marker.DMLNameUnassigned(), tc.want)
		}
		if _, present := records[0].GetField("Name"); !present {
			t.Errorf("%s: queried Name lost in DML conversion", tc.name)
		}
	}
}
