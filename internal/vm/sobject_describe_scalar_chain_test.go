package vm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func objectScalarChain(getter string) ir.Expr {
	return ir.Expr{Kind: ir.ExprCall, Callee: getter, Left: &ir.Expr{Kind: ir.ExprCall, Callee: "objectType.getDescribe"}}
}

func TestSObjectDescribeScalarChainMatchesEager(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		machine := describeShareUnprimedFor(namespaced)
		machine.VarTypes["objectType"] = "Schema.SObjectType"
		for _, object := range []string{"Account", "Widget__c", "Missing__c"} {
			machine.Globals["objectType"] = sObjectTypeToken(object)
			for _, getter := range []string{"getName", "getLabel", "getLabelPlural", "getKeyPrefix", "isCustom", "isCustomSetting", "isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable", "getRecordTypeInfos"} {
				chain := objectScalarChain(getter)
				eagerTrace, chainTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
				describe, eagerErr := machine.eval(*chain.Left, eagerTrace)
				want := Null
				if eagerErr == nil {
					want, _, eagerErr = machine.callValueMember("", describe, getter, nil, eagerTrace)
				}
				got, gotErr := machine.eval(chain, chainTrace)
				// Collection getters allocate separate outer references; compare their contents.
				if getter == "getRecordTypeInfos" {
					want.Ref, got.Ref = 0, 0
				}
				if !reflect.DeepEqual(want, got) || fmt.Sprint(eagerErr) != fmt.Sprint(gotErr) || !reflect.DeepEqual(eagerTrace.Trace, chainTrace.Trace) {
					t.Fatalf("namespace=%v %s.%s: eager=%v/%v chain=%v/%v", namespaced, object, getter, want, eagerErr, got, gotErr)
				}
			}
		}
	}
}

func BenchmarkSObjectDescribeScalarChain64Fields(b *testing.B) {
	machine := New(nil)
	org := describeOverlayOrg(64)
	org.Namespace = ""
	object := org.Objects["pkg__Widget__c"]
	object.Definition.APIName = "Widget__c"
	org.Objects["Widget__c"] = object
	machine.SetOrg(&org)
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	machine.Globals["objectType"] = sObjectTypeToken("Widget__c")
	chain := objectScalarChain("getLabel")
	result := &Result{}
	if _, err := machine.eval(chain, result); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := machine.eval(chain, result)
		if err != nil || value.Kind != ValueString {
			b.Fatalf("%v %v", value, err)
		}
	}
}

func TestSObjectDescribeScalarChainFallbackAndAliases(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["objectType"] = sObjectTypeToken("Account")
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	for _, getter := range []string{"getRecordTypeInfos", "getFields", "getSObjectType"} {
		_, handled, err := machine.evalSObjectDescribeScalarChain(&[]ir.Expr{objectScalarChain(getter)}[0], &Result{})
		if handled || err != nil {
			t.Fatalf("%s unexpectedly optimized: %v", getter, err)
		}
	}
	chain := objectScalarChain("getName")
	chain.Left.Args = []ir.Expr{{Kind: ir.ExprLiteral, Value: "DEFERRED"}}
	if _, handled, _ := machine.evalSObjectDescribeScalarChain(&chain, &Result{}); handled {
		t.Fatal("option argument optimized")
	}

	program, err := CompileAnonymous(`
Schema.SObjectType objectType = Account.SObjectType;
Schema.DescribeSObjectResult saved = objectType.getDescribe();
Schema.DescribeSObjectResult alias = saved;
String before = JSON.serialize(saved);
System.assertEquals(before, JSON.serialize(alias));
saved.name = 'Changed';
System.assertEquals('Changed', alias.getName());
System.assertEquals('Account', objectType.getDescribe().getName());
System.assertEquals('Account', objectType.getDescribe().getName());
System.assertEquals(JSON.serialize(saved), JSON.serialize(alias));
System.assert(before != JSON.serialize(saved));
List<Schema.RecordTypeInfo> records = objectType.getDescribe().getRecordTypeInfos();
Integer count = records.size();
records.clear();
System.assertEquals(count, objectType.getDescribe().getRecordTypeInfos().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSObjectDescribeScalarChainReferenceAndErrors(t *testing.T) {
	machine := describeShareUnprimed()
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	machine.Globals["objectType"] = sObjectTypeToken("Account")
	chain := objectScalarChain("getName")
	if _, handled, err := machine.evalSObjectDescribeScalarChain(&chain, &Result{}); !handled || err != nil {
		t.Fatalf("scalar chain not handled: %v", err)
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
	if got := nextValueRef.Load() - before; got != eager {
		t.Fatalf("reference allocations: eager %d, chain %d", eager, got)
	}
	for _, invalid := range []Value{Null, Int(1)} {
		token := sObjectTypeToken("Account")
		token.Fields["object"] = invalid
		machine.Globals["objectType"] = token
		_, want := machine.eval(*chain.Left, &Result{})
		_, got := machine.eval(chain, &Result{})
		if want == nil || fmt.Sprint(want) != fmt.Sprint(got) {
			t.Fatalf("malformed token: eager %v, chain %v", want, got)
		}
	}
}

func TestSObjectDescribeScalarChainRegisteredGetterKeepsDispatch(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["objectType"] = sObjectTypeToken("Account")
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	program, err := CompileAnonymous("return 'override';")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Schema.DescribeSObjectResult", Methods: map[string]Method{
		"getName": {Name: "Schema.DescribeSObjectResult.getName", ClassName: "Schema.DescribeSObjectResult", Access: "public", ReturnType: "String", Program: program},
	}}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.eval(objectScalarChain("getName"), &Result{})
	if err != nil || value.Text != "override" {
		t.Fatalf("registered getter: %v %v", value, err)
	}
}
