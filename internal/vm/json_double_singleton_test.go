package vm

import "testing"

func TestExecJSONSerializeCapturedDoubleSingleton(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals('[1.0]', JSON.serialize(new List<Object>{Double.valueOf('1.0')})); System.assertEquals('[1]', JSON.serialize(new List<Object>{1})); System.assertEquals('[1.0]', JSON.serialize(new List<Object>{Decimal.valueOf('1.0')}));")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
