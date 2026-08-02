package vm

import (
	"testing"
)

func TestCB170NewDMLOptionsOptAllOrNoneDefaultsToNull(t *testing.T) {
	program, err := CompileAnonymous(`
Database.DMLOptions opts = new Database.DMLOptions();
System.assertEquals(null, opts.optAllOrNone, 'default optAllOrNone lower');
System.assertEquals(null, opts.OptAllOrNone, 'default OptAllOrNone upper');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
