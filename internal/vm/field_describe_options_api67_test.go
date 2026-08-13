package vm

import "testing"

func TestExecAPI67FieldDescribeOptionsHashCode(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.FieldDescribeOptions option = Schema.FieldDescribeOptions.DEFAULT;
System.assertEquals(true, option.equals(Schema.FieldDescribeOptions.valueOf('DEFAULT')));
System.assertEquals(-611082246, option.hashCode());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
