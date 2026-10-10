package vm

import "testing"

func TestSerializationExceptionConstructorPreservesExplicitMessage(t *testing.T) {
	program, err := CompileAnonymous(`
Exception value = new System.SerializationException('rollup proof');
System.assertEquals('System.SerializationException', value.getTypeName());
System.assertEquals('rollup proof', value.getMessage());
System.assertEquals('System.SerializationException: rollup proof', value.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
