package vm

import (
	"strings"
	"testing"
)

// Enum.values() returns the declaration's constants in declaration order. A
// loop variable is introduced after its iterable expression is evaluated.
func TestNestedEnumValuesReceiverInEnhancedFor(t *testing.T) {
	for _, namespace := range []string{"", "owned"} {
		t.Run(namespace, func(t *testing.T) {
			machine := New(nil)
			for _, class := range []Class{
				{Name: "Cycle", Namespace: namespace},
				{Name: "Cycle.Phase", Namespace: namespace, EnumValues: []string{"FIRST", "SECOND"}},
				{Name: "OtherCycle", Namespace: namespace},
				{Name: "OtherCycle.Phase", Namespace: namespace, EnumValues: []string{"OTHER"}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymousWithOptions(`
Integer count = 0;
for (Phase phase : Phase.values()) {
    System.assertEquals(count, phase.ordinal());
    count++;
}

System.assertEquals(2, count);
List<Phase> afterLoop = Phase.values();
System.assertEquals(2, afterLoop.size());
System.assertEquals('FIRST', afterLoop[0].name());
System.assertEquals('SECOND', afterLoop[1].name());
List<OtherCycle.Phase> sibling = OtherCycle.Phase.values();
System.assertEquals(1, sibling.size());
System.assertEquals('OTHER', sibling[0].name());
for (String branch : new List<String>{'NUMBER','ENUM'}) {
    switch on branch {
        when 'NUMBER' {
            Integer phase = 7;
            System.assertEquals(7, phase);
        }
        when 'ENUM' {
            Integer seen = 0;
            for (Phase phase : Phase.values()) {
                System.assertEquals(seen, phase.ordinal());
                seen++;
            }
            System.assertEquals(2, seen);
        }
    }
}
`, CompileOptions{APIVersion: "49.0"})
			if err != nil {
				t.Fatal(err)
			}
			owner := "Cycle"
			if namespace != "" {
				owner = namespace + "." + owner
			}
			if _, err := machine.ExecuteInClass(program, owner); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSwitchWhenBodiesRestoreLocalScope(t *testing.T) {
	for _, tt := range []struct {
		name, selector, arm, signal string
		wantSignal                  controlSignal
	}{
		{name: "literal", selector: "1", arm: "1"},
		{name: "enum", selector: "ScopeMode.FIRST", arm: "FIRST"},
		{name: "null", selector: "null", arm: "null"},
		{name: "else", selector: "2", arm: "else"},
		{name: "type", selector: "new Account(Name='Scoped')", arm: "Account row"},
		{name: "break", selector: "1", arm: "1", signal: "break;"},
		{name: "continue", selector: "1", arm: "1", signal: "continue;", wantSignal: signalContinue},
		{name: "return", selector: "1", arm: "1", signal: "return 9;", wantSignal: signalReturn},
		{name: "throw", selector: "1", arm: "1", signal: "throw new IllegalArgumentException('scoped');", wantSignal: signalThrow},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "ScopeMode", EnumValues: []string{"FIRST"}}); err != nil {
				t.Fatal(err)
			}
			source := strings.NewReplacer("SELECTOR", tt.selector, "ARM", tt.arm, "SIGNAL", tt.signal).Replace(`
Integer totalValue = 1;
switch on SELECTOR {
    when ARM {
        final Integer scoped = 7;
        totalValue++;
        SIGNAL
    }
}
`)
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			out, err := machine.executeProgram(program, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if out.signal != tt.wantSignal {
				t.Fatalf("signal = %q, want %q", out.signal, tt.wantSignal)
			}
			if got := machine.Globals["totalValue"]; got.Kind != ValueInt || got.Int != 2 {
				t.Fatalf("outer mutation = %#v, want Integer 2", got)
			}
			for _, name := range []string{"scoped", "row"} {
				if _, ok := machine.Globals[name]; ok {
					t.Errorf("when-local value %s escaped its block", name)
				}
				if _, ok := machine.VarTypes[name]; ok {
					t.Errorf("when-local type %s escaped its block", name)
				}
				if _, ok := machine.finalLocals[name]; ok {
					t.Errorf("when-local final marker %s escaped its block", name)
				}
			}
		})
	}
}
