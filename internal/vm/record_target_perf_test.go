package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRecordMatchNonRecordTargetsAvoidSourceProbe(t *testing.T) {
	machine := New(nil)
	for _, test := range []struct {
		name, source, target string
		limit                float64
	}{
		{"type token assignment", "Schema.SObjectType", "Schema.SObjectType", 4},
		{"field token assignment", "Schema.SObjectField", "Schema.SObjectField", 4},
		{"describe return", "Schema.DescribeSObjectResult", "Schema.DescribeSObjectResult", 4},
		{"platform argument", "RestRequest", "RestRequest", 0},
		{"ordinary record target", "Account", "Account", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := Object(test.source)
			allocations := testing.AllocsPerRun(100, func() {
				if matched, handled := machine.schemaRecordValueMatches(value, test.target); matched || handled {
					t.Fatal("non-record target entered record provenance matching")
				}
			})
			t.Logf("allocations/check=%.0f budget=%.0f", allocations, test.limit)
			if allocations > test.limit {
				t.Fatalf("non-record target probed source: %.0f allocations, budget %.0f", allocations, test.limit)
			}
		})
	}
}

func TestRecordMatchRecordTargetsRetainProvenance(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine := New(nil)
	machine.SetOrg(&org)
	record := Object("Account")
	instance := Object("Account")
	instance.classInstance = true
	token := sObjectTypeToken("Account")
	for _, test := range []struct {
		name, target     string
		value            Value
		matched, handled bool
	}{
		{"record to Object", "Object", record, true, true},
		{"record to SObject", "SObject", record, true, true},
		{"record to explicit record", "Schema.Account", record, true, true},
		{"record to different record", "Schema.Contact", record, false, true},
		{"class to Object", "Object", instance, false, false},
		{"class to SObject", "SObject", instance, false, true},
		{"class to explicit record", "Schema.Account", instance, false, true},
		{"type token to Object", "Object", token, false, false},
		{"type token to SObject", "SObject", token, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			matched, handled := machine.schemaRecordValueMatches(test.value, test.target)
			if matched != test.matched || handled != test.handled {
				t.Fatalf("matched/handled=%v/%v, want %v/%v", matched, handled, test.matched, test.handled)
			}
		})
	}
}

func TestRecordMatchTokenPublicEntryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct{ name, source string }{
			{"assignment", `Schema.SObjectType assigned = token; System.assertEquals(token, assigned);`},
			{"cast", `Schema.SObjectType casted = (Schema.SObjectType)view; System.assertEquals(token, casted);`},
			{"instanceof", `System.assert(view instanceof Schema.SObjectType); System.assert(!(view instanceof Schema.SObjectField));`},
			{"Assert.isInstanceOfType", `Assert.isInstanceOfType(view, Schema.SObjectType.class);`},
			{"Assert.isNotInstanceOfType", `Assert.isNotInstanceOfType(view, Schema.SObjectField.class);`},
		} {
			t.Run(api+"/"+route.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`Schema.SObjectType token = Account.SObjectType; Object view = token;`+route.source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func BenchmarkRecordMatchTokenAssignment(b *testing.B) {
	machine := New(nil)
	value := sObjectTypeToken("Account")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := machine.coerceAssignable("Schema.SObjectType", value); err != nil {
			b.Fatal(err)
		}
	}
}
