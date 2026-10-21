package vm

import (
	"fmt"
	"testing"
)

// Preserve the aggregate JSON behavior covered by the existing owned mock
// test, including type tokens carried through lists and helper parameters.
func TestJSONAggregateResultTypeTokenRoundTrip(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, isTest := range []bool{false, true} {
			for _, method := range []string{"deserialize", "deserializeStrict"} {
				for name, source := range map[string]string{
					"scalar":      `AggregateResult row=(AggregateResult)JSON.%s('{}',AggregateResult.class);System.assertEquals(Schema.AggregateResult.SObjectType,row.getSObjectType());System.assertEquals(null,row.Id);`,
					"list":        `String body=JSON.serialize(new List<Map<String,Object>>{new Map<String,Object>{'Id'=>(Id)'001000000000001'}});List<AggregateResult> rows=(List<AggregateResult>)JSON.%s(body,List<AggregateResult>.class);System.assertEquals(1,rows.size());System.assertEquals((Id)'001000000000001',rows[0].Id);System.assertEquals(Schema.AggregateResult.SObjectType,rows.getSObjectType());`,
					"helper":      `List<AggregateResult> rows=(List<AggregateResult>)AggregateJSONBridge.%s('[{"Id":"001000000000001AAA"}]',List<AggregateResult>.class);System.assertEquals((Id)'001000000000001',rows[0].Id);System.assertEquals(Schema.AggregateResult.SObjectType,rows[0].getSObjectType());`,
					"classFields": `AggregateJSONEnvelope decoded=(AggregateJSONEnvelope)JSON.%s('{"row":{"Id":"001000000000001AAA"},"rows":[{"Id":"001000000000001AAA"}]}',AggregateJSONEnvelope.class);System.assertEquals(Schema.AggregateResult.SObjectType,decoded.row.getSObjectType());System.assertEquals(1,decoded.rows.size());System.assertEquals((Id)'001000000000001',decoded.row.Id);System.assertEquals((Id)'001000000000001',decoded.rows[0].Id);`,
				} {
					t.Run(fmt.Sprintf("%s/isTest=%t/%s/%s", api, isTest, method, name), func(t *testing.T) {
						machine := aggregateJSONTestMachine(t, api)
						if isTest {
							machine.EnableTestContext()
						}
						program, err := CompileAnonymousWithOptions(fmt.Sprintf(source, method), CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

func aggregateJSONTestMachine(t *testing.T, api string) *VM {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AggregateJSONEnvelope", Fields: map[string]Field{
		"row": {Name: "row", Type: "AggregateResult"}, "rows": {Name: "rows", Type: "List<AggregateResult>"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "AggregateJSONBridge"}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"deserialize", "deserializeStrict"} {
		program, err := CompileAnonymousWithOptions("return JSON."+method+"(body,target);", CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: "AggregateJSONBridge." + method, ClassName: "AggregateJSONBridge", IsStatic: true, ReturnType: "Object", Params: []Param{{Name: "body", Type: "String"}, {Name: "target", Type: "Type"}}, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}
