package vm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

var fieldDescribeScalarGetters = []string{
	"getName", "getLabel", "getInlineHelpText", "getLocalName", "getCompoundFieldName",
	"isNillable", "isExternalId", "isUnique", "isEncrypted", "isCalculated", "getCalculatedFormula",
	"isAutoNumber", "isCaseSensitive", "isNameField", "isCustom", "getLength", "getByteLength",
	"getPrecision", "getScale", "getDigits", "isHtmlFormatted", "isSortable", "isDefaultedOnCreate",
	"getRelationshipName", "getRelationshipOrder", "isAccessible", "isCreateable", "isUpdateable",
	"getType", "getSoapType",
}

func scalarDescribeChain(method string) ir.Expr {
	return ir.Expr{Kind: ir.ExprCall, Callee: method, Left: &ir.Expr{Kind: ir.ExprCall, Callee: "field.getDescribe"}}
}

// These are differential local controls. Native scalar reads remain covered by
// testdata/describe_read_workload/org.tsv:2-6,11-15 at source APIs 62 and 67.
func TestFieldDescribeScalarChainsMatchEager(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespaced := range []bool{false, true} {
			t.Run(api+fmt.Sprint(namespaced), func(t *testing.T) {
				machine := describeShareUnprimedFor(namespaced)
				machine.Org.APIVersion = api
				machine.currentMethod = Method{APIVersion: api, SourceContextBound: true}
				for _, target := range [][2]string{{"Account", "Name"}, {"Account", "Id"}, {"Account", "CreatedById"}, {"Account", "Missing__c"}, {"Widget__c", "Status__c"}, {"Widget__c", "Owner__c"}} {
					field := target[0] + "." + target[1]
					token := machine.sObjectFieldToken(target[0], target[1])
					machine.Globals["field"] = token
					machine.VarTypes["field"] = "Schema.SObjectField"
					for _, getter := range fieldDescribeScalarGetters {
						chain := scalarDescribeChain(getter)
						eagerTrace, chainTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
						describe, eagerErr := machine.eval(*chain.Left, eagerTrace)
						want := Null
						if eagerErr == nil {
							want, _, eagerErr = machine.callValueMember("", describe, getter, nil, eagerTrace)
						}
						heapBefore := machine.currentHeapSize()
						got, chainErr := machine.eval(chain, chainTrace)
						if !reflect.DeepEqual(want, got) || fmt.Sprint(eagerErr) != fmt.Sprint(chainErr) ||
							!reflect.DeepEqual(eagerTrace.Trace, chainTrace.Trace) || heapBefore != machine.currentHeapSize() {
							t.Fatalf("%s.%s: eager=%+v/%v chain=%+v/%v traces equal=%v", field, getter, want, eagerErr, got, chainErr, reflect.DeepEqual(eagerTrace.Trace, chainTrace.Trace))
						}
					}
				}
				// Read current token fields on every call, even after the describe
				// cache is warm. A token Ref is not a valid result-cache key.
				token := machine.Globals["field"]
				token.Fields["field"] = String("Name")
				token.Fields["object"] = String("Contact")
				got, err := machine.eval(scalarDescribeChain("getName"), &Result{})
				if err != nil || got.Text != "Name" {
					t.Fatalf("mutated token: %v %v", got, err)
				}
			})
		}
	}
}

func TestFieldDescribeScalarChainWarmGenerationAndPermissions(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["field"] = machine.sObjectFieldToken("Widget__c", "Status__c")
	machine.VarTypes["field"] = "Schema.SObjectField"
	read := func(getter string) Value {
		t.Helper()
		got, err := machine.eval(scalarDescribeChain(getter), &Result{})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := read("getLabel"); got.Text != "Status" {
		t.Fatal(got)
	}
	definition, ok := storage.EnsureMutableObjectDefinition(machine.Org, "Widget__c")
	if !ok {
		t.Fatal("missing fixture definition")
	}
	field := definition.Fields["Status__c"]
	field.Label = "Changed"
	definition.Fields["Status__c"] = field
	if got := read("getLabel"); got.Text != "Changed" {
		t.Fatalf("warm scalar retained old generation: %v", got)
	}
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String("005000000000001aaa")
	// The fixture enables test context, whose current user takes precedence over
	// the request user. Both routes must see the profile being changed below.
	machine.testContext.CurrentUser = machine.executionUser
	for i, allowed := range []bool{false, true, false} {
		profile := fmt.Sprintf("00e00000000000%d", i+1)
		permissionIdentityPut(machine, "ObjectPermissions", fmt.Sprintf("11000000000000%d", i+1), map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(profile)), "SObjectType": storage.StringValue("Widget__c"), "PermissionsRead": storage.BooleanValue(true),
		})
		permissionIdentityPut(machine, "FieldPermissions", fmt.Sprintf("0FP00000000000%d", i+1), map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(profile)), "SObjectType": storage.StringValue("Widget__c"), "Field": storage.StringValue("Widget__c.Status__c"), "PermissionsRead": storage.BooleanValue(allowed),
		})
		machine.executionUser.Fields["ProfileId"] = String(profile)
		describe, err := machine.eval(*scalarDescribeChain("isAccessible").Left, &Result{})
		if err != nil {
			t.Fatal(err)
		}
		eager, _, err := machine.callValueMember("", describe, "isAccessible", nil, &Result{})
		if err != nil || eager.Kind != ValueBool || eager.Bool != allowed {
			t.Fatalf("eager permission context %d: got=%v error=%v want=%v", i, eager, err, allowed)
		}
		if got := read("isAccessible"); got.Kind != ValueBool || got.Bool != allowed {
			t.Fatalf("permission context %d: got=%v want=%v", i, got, allowed)
		}
	}
}

func TestFieldDescribeScalarChainRefAndErrorControls(t *testing.T) {
	machine := describeShareUnprimed()
	machine.VarTypes["field"] = "Schema.SObjectField"
	for _, field := range []string{"Name", "CreatedById", "Id"} {
		machine.Globals["field"] = machine.sObjectFieldToken("Account", field)
		chain := scalarDescribeChain("getName")
		// Warm declaration/schema preparation before comparing observable Ref
		// allocation, including the synthetic correction's list/token headers.
		if _, err := machine.eval(chain, &Result{}); err != nil {
			t.Fatal(err)
		}
		before := nextValueRef.Load()
		if _, err := machine.eval(*chain.Left, &Result{}); err != nil {
			t.Fatal(err)
		}
		eager := nextValueRef.Load() - before
		before = nextValueRef.Load()
		if _, err := machine.eval(chain, &Result{}); err != nil {
			t.Fatal(err)
		}
		if chained := nextValueRef.Load() - before; chained != eager {
			t.Fatalf("%s: eager Ref delta=%d chain=%d", field, eager, chained)
		}
	}
	for _, field := range []string{"object", "field"} {
		for _, invalid := range []Value{Null, Int(1)} {
			token := machine.sObjectFieldToken("Account", "Name")
			token.Fields[field] = invalid
			machine.Globals["field"] = token
			chain := scalarDescribeChain("getName")
			_, want := machine.eval(*chain.Left, &Result{})
			_, got := machine.eval(chain, &Result{})
			if want == nil || fmt.Sprint(want) != fmt.Sprint(got) {
				t.Fatalf("malformed %s: eager=%v chain=%v", field, want, got)
			}
		}
	}
}

func TestFieldDescribeScalarChainFallbacks(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		machine := describeShareUnprimed()
		source := `Schema.SObjectField field = Account.Name;
Schema.DescribeFieldResult saved = field.getDescribe(); saved.name = 'private';
System.assertEquals('Name', field.getDescribe().getName());
System.assertEquals('Name', field.getDescribe(Schema.FieldDescribeOptions.DEFAULT).getName());
Schema.SObjectField missing = null;
System.assertEquals(null, missing?.getDescribe().getName());
System.assertEquals('Name', Account.SObjectType.fields.getMap().get('Name').getDescribe().getName());
System.assertEquals(Schema.DisplayType.STRING, field.getDescribe().getType());
Schema.SObjectField owner = Account.OwnerId;
System.assert(owner.getDescribe().getReferenceTo().size() > 0);`
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatalf("API %s: %v", api, err)
		}
	}
}

func TestFieldDescribeScalarChainRegisteredGetterKeepsDispatch(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["field"] = machine.sObjectFieldToken("Account", "Name")
	machine.VarTypes["field"] = "Schema.SObjectField"
	program, err := CompileAnonymous("return 'override';")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Schema.DescribeFieldResult", Methods: map[string]Method{
		"getName": {Name: "Schema.DescribeFieldResult.getName", ClassName: "Schema.DescribeFieldResult", Access: "public", ReturnType: "String", Program: program},
	}}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.eval(scalarDescribeChain("getName"), &Result{})
	if err != nil || value.Text != "override" {
		t.Fatalf("registered getter was bypassed: %v %v", value, err)
	}
}

func TestFieldDescribeScalarChainAvoidsRootCopy(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["field"] = machine.sObjectFieldToken("Account", "Name")
	machine.VarTypes["field"] = "Schema.SObjectField"
	chain := scalarDescribeChain("getName")
	result := &Result{}
	eager := testing.AllocsPerRun(20, func() {
		describe, err := machine.eval(*chain.Left, result)
		if err != nil {
			t.Fatal(err)
		}
		if value, _, err := machine.callValueMember("", describe, chain.Callee, nil, result); err != nil || value.Text != "Name" {
			t.Fatalf("eager: %v %v", value, err)
		}
	})
	chained := testing.AllocsPerRun(20, func() {
		if value, err := machine.eval(chain, result); err != nil || value.Text != "Name" {
			t.Fatalf("chain: %v %v", value, err)
		}
	})
	t.Logf("eager allocations=%.0f chained allocations=%.0f", eager, chained)
	if chained >= eager/2 {
		t.Fatalf("scalar chain still materialized a describe root: eager=%.0f chain=%.0f", eager, chained)
	}
}

// A source class may declare only a platform short name, with no members.
// Dispatch then has nothing to select, so the proven chain stays eligible and
// must still match the eager route exactly; any member keeps the eager route.
func TestFieldDescribeScalarChainMemberlessShadowClass(t *testing.T) {
	shadows := []string{"SObjectField", "DescribeFieldResult"}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespaced := range []bool{false, true} {
			t.Run(api+fmt.Sprint(namespaced), func(t *testing.T) {
				machine := describeShareUnprimedFor(namespaced)
				machine.Org.APIVersion = api
				machine.currentMethod = Method{APIVersion: api, SourceContextBound: true}
				for _, name := range shadows {
					if err := machine.RegisterClass(Class{Name: name, Access: "public"}); err != nil {
						t.Fatal(err)
					}
				}
				for _, target := range [][2]string{{"Account", "Name"}, {"Account", "CreatedById"}, {"Account", "Missing__c"}, {"Widget__c", "Status__c"}} {
					machine.Globals["field"] = machine.sObjectFieldToken(target[0], target[1])
					machine.VarTypes["field"] = "Schema.SObjectField"
					for _, getter := range fieldDescribeScalarGetters {
						chain := scalarDescribeChain(getter)
						eagerTrace, chainTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
						describe, eagerErr := machine.eval(*chain.Left, eagerTrace)
						want := Null
						if eagerErr == nil {
							want, _, eagerErr = machine.callValueMember("", describe, getter, nil, eagerTrace)
						}
						heapBefore := machine.currentHeapSize()
						got, chainErr := machine.eval(chain, chainTrace)
						if !reflect.DeepEqual(want, got) || fmt.Sprint(eagerErr) != fmt.Sprint(chainErr) ||
							!reflect.DeepEqual(eagerTrace.Trace, chainTrace.Trace) || heapBefore != machine.currentHeapSize() {
							t.Fatalf("%v.%s: eager=%+v/%v chain=%+v/%v", target, getter, want, eagerErr, got, chainErr)
						}
					}
				}
			})
		}
	}
	chainAllocations := func(t *testing.T, shadow Class) (float64, float64) {
		t.Helper()
		machine := describeShareUnprimed()
		if err := machine.RegisterClass(shadow); err != nil {
			t.Fatal(err)
		}
		machine.Globals["field"] = machine.sObjectFieldToken("Account", "Name")
		machine.VarTypes["field"] = "Schema.SObjectField"
		chain := scalarDescribeChain("getLabel")
		result := &Result{}
		eager := testing.AllocsPerRun(20, func() {
			describe, err := machine.eval(*chain.Left, result)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := machine.callValueMember("", describe, chain.Callee, nil, result); err != nil {
				t.Fatal(err)
			}
		})
		chained := testing.AllocsPerRun(20, func() {
			if _, err := machine.eval(chain, result); err != nil {
				t.Fatal(err)
			}
		})
		return eager, chained
	}
	eager, chained := chainAllocations(t, Class{Name: "SObjectField", Access: "public"})
	if chained >= eager/2 {
		t.Fatalf("memberless shadow class kept the eager describe copy: eager=%.0f chain=%.0f", eager, chained)
	}
	program, err := CompileAnonymous("return;")
	if err != nil {
		t.Fatal(err)
	}
	for label, shadow := range map[string]Class{
		"method":      {Name: "SObjectField", Access: "public", Methods: map[string]Method{"helper": {Name: "SObjectField.helper", ClassName: "SObjectField", Access: "public", ReturnType: "void", Program: program}}},
		"supertype":   {Name: "DescribeFieldResult", Access: "public", SuperClass: "Exception"},
		"interface":   {Name: "SObjectField", Access: "public", Interfaces: []string{"Comparable"}},
		"staticField": {Name: "DescribeFieldResult", Access: "public", StaticFields: map[string]Field{"marker": {Type: "Integer"}}},
		"initializer": {Name: "SObjectField", Access: "public", StaticInitializers: []Method{{Name: "SObjectField.<clinit>", ClassName: "SObjectField", Program: program}}},
		"platform":    {Name: "SObjectField", Access: "public", PlatformType: true},
	} {
		eager, chained := chainAllocations(t, shadow)
		if chained < eager*3/4 {
			t.Fatalf("%s: a class with members must keep eager dispatch: eager=%.0f chain=%.0f", label, eager, chained)
		}
	}
}

// Enum getters return the value stored in the describe template, which an
// eager describe copy shares too; the chain must return that same value.
func TestFieldDescribeScalarChainEnumGettersShareEagerValue(t *testing.T) {
	machine := describeShareUnprimed()
	for _, target := range [][2]string{{"Account", "Name"}, {"Account", "CreatedById"}, {"Widget__c", "Status__c"}} {
		machine.Globals["field"] = machine.sObjectFieldToken(target[0], target[1])
		machine.VarTypes["field"] = "Schema.SObjectField"
		for _, getter := range []string{"getType", "getSoapType"} {
			chain := scalarDescribeChain(getter)
			describe, err := machine.eval(*chain.Left, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			want, _, err := machine.callValueMember("", describe, getter, nil, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			again, _, _ := machine.callValueMember("", describe, getter, nil, &Result{})
			stored := want.Fields != nil && sameMapIdentity(again.Fields, want.Fields)
			got, err := machine.eval(chain, &Result{})
			if err != nil || !reflect.DeepEqual(got, want) || (stored && !sameMapIdentity(got.Fields, want.Fields)) {
				t.Fatalf("%v.%s: chain=%+v eager=%+v err=%v", target, getter, got, want, err)
			}
		}
	}
}
