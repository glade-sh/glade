package vm

import "testing"

// Native: csnpe_capture/chain-org.tsv:2-9; a chained null field returns null.
func TestChainedNullCustomSettingFieldReturnsNull(t *testing.T) {
	machine := New(nil)
	org := customDataOrg()
	machine.SetOrg(&org)
	holder := Object("ProbeHolder")
	holder.Fields["setting"] = typedNull("Local_Setting__c")
	value, err := machine.lookupPath(holder, []string{"setting", "Optional__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("chained field = %#v, want null", value)
	}
}

// Native: csnpe_capture/org.tsv:3,12,21,30 report the dereference line.
func TestCustomSettingNullFieldExceptionLine(t *testing.T) {
	source := `Local_Setting__c s = Local_Setting__c.getValues('Missing');
Boolean caught = false;
try {
 String value = s.Optional__c;
} catch (NullPointerException e) { caught = true; System.assertEquals(4, e.getLineNumber()); }
System.assert(caught);`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := customDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
