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
			for _, getter := range []string{"getName", "getLabel", "getLabelPlural", "getKeyPrefix", "isCustom", "isCustomSetting", "isAccessible", "getRecordTypeInfos"} {
				chain := objectScalarChain(getter)
				eagerTrace, chainTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
				describe, eagerErr := machine.eval(*chain.Left, eagerTrace)
				want := Null
				if eagerErr == nil {
					want, _, eagerErr = machine.callValueMember("", describe, getter, nil, eagerTrace)
				}
				got, gotErr := machine.eval(chain, chainTrace)
				// Collection getters allocate separate references; compare their serialized form.
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
	machine.SetOrg(&org)
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	machine.Globals["objectType"] = sObjectTypeToken("pkg__Widget__c")
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
