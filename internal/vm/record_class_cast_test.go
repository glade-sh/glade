package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The record_target_checks native rows classRecordCast and recordClassCast
// reject both directions at API 62/67, through anonymous and named test entry.
// Exercise both callers and both operand modes of the shared cast path here;
// TestRecordTargetChecksNativeConformance pins the complete project routes.
func TestRecordClassCastOperandRoutesRespectProvenance(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Account"}); err != nil {
		t.Fatal(err)
	}
	record := Object("Account")
	record.Static = "Object"
	instance := Object("Account")
	instance.classInstance = true
	instance.Static = "Object"
	for _, route := range []struct {
		name string
		cast func(string, Value) (Value, error)
	}{
		{"coerceCast", machine.coerceCast},
		{"expressionOperand", func(target string, value Value) (Value, error) {
			return machine.coerceCastForOperand(target, value, false)
		}},
		{"directQueryOperand", func(target string, value Value) (Value, error) {
			return machine.coerceCastForOperand(target, value, true)
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			for _, test := range []struct {
				name, target string
				value        Value
				reject       bool
			}{
				{"recordToClass", "Account", record, true},
				{"classToRecord", "Schema.Account", instance, true},
				{"classToClass", "Account", instance, false},
				{"recordToRecord", "Schema.Account", record, false},
				{"recordToObject", "Object", record, false},
				{"recordToSObject", "SObject", record, false},
				{"classToObject", "Object", instance, false},
				{"nullToClass", "Account", Null, false},
				{"nullToRecord", "Schema.Account", Null, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					got, err := route.cast(test.target, test.value)
					if test.reject {
						var thrown *apexThrowError
						if !errors.As(err, &thrown) || thrown.value.Type != "System.TypeException" {
							t.Fatalf("cast accepted or wrong exception: value=%#v error=%v; want System.TypeException", got, err)
						}
						return
					}
					wantType := test.value.Type
					if test.value.Kind == ValueNull {
						wantType = test.target
					}
					if err != nil || got.Kind != test.value.Kind || got.Type != wantType || got.classInstance != test.value.classInstance {
						t.Fatalf("valid cast changed provenance: value=%#v error=%v", got, err)
					}
				})
			}
		})
	}
}

// Local preservation controls: a dependency schema stub is not a user class,
// and an unshadowed record still casts to its ordinary unqualified type.
func TestRecordClassCastUnshadowedRecordOperandRoutes(t *testing.T) {
	for _, route := range []struct {
		name       string
		dependency bool
	}{
		{"ordinaryRecord", false},
		{"dependencySchemaStub", true},
	} {
		t.Run(route.name, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			machine.SetOrg(&org)
			if route.dependency {
				if err := machine.RegisterClass(Class{Name: "Account", Dependency: true}); err != nil {
					t.Fatal(err)
				}
			}
			for _, operand := range []struct {
				name        string
				directQuery bool
			}{
				{"expressionOperand", false},
				{"directQueryOperand", true},
			} {
				t.Run(operand.name, func(t *testing.T) {
					record := Object("Account")
					got, err := machine.coerceCastForOperand("Account", record, operand.directQuery)
					if err != nil || got.Kind != ValueObject || got.Type != "Account" || got.classInstance || got.Ref != record.Ref {
						t.Fatalf("ordinary record cast=%#v, error=%v", got, err)
					}
				})
			}
		})
	}
}
