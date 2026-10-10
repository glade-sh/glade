package vm

import "testing"

func TestExecInstanceOfDecimalListDoubleFormatterRoute(t *testing.T) {
	program, err := CompileAnonymous(`
Object values = new List<Decimal>{5.5, 6.6};
String before = JSON.serialize(values);
System.assert(values instanceof List<Decimal>);
System.assert(values instanceof List<Double>);
List<Double> doubles = (List<Double>) values;
System.assertEquals('5.5', String.valueOf(doubles[0]));
System.assertEquals('6.6', String.valueOf(doubles[1]));
System.assertEquals(before, JSON.serialize(values));

Object uniqueValues = new Set<Decimal>{7.7, 8.8};
System.assert(uniqueValues instanceof Set<Decimal>);
System.assert(!(uniqueValues instanceof Set<Double>));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
