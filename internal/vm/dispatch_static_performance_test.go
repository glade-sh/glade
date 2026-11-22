package vm

import (
	"strings"
	"testing"
)

// canonicalBuiltinStaticCallParent is the origin/main lookup, kept as an oracle.
func canonicalBuiltinStaticCallParent(callee string) (string, bool) {
	if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(callee)]; ok {
		return canonical, true
	}
	if rest, ok := stripLeadingSystemNamespace(callee); ok {
		if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(rest)]; ok {
			return canonical, true
		}
		if typeName, member, ok := splitDottedTypeMember(rest); ok {
			if canonicalType, typeOK := canonicalSystemNamespaceType(typeName); typeOK {
				callee := canonicalType + "." + member
				if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(callee)]; ok {
					return canonical, true
				}
				return callee, true
			}
		}
	}
	return "", false
}

func TestCanonicalBuiltinStaticCallMatchesParent(t *testing.T) {
	inputs := []string{"", "System", "System.Missing.call", "System.String.missing", "Math.missing", "System.Ω.foo", "math.sqrt ", "System.System.debug"}
	for lower, canonical := range canonicalBuiltinStaticCalls {
		inputs = append(inputs, lower, canonical, strings.ToUpper(canonical), "System."+canonical, canonical+"_neighbor")
	}
	for _, input := range inputs {
		got, ok := canonicalBuiltinStaticCall(input)
		want, wantOK := canonicalBuiltinStaticCallParent(input)
		if got != want || ok != wantOK {
			t.Errorf("%q: got %q %t, want %q %t", input, got, ok, want, wantOK)
		}
	}
}

const builtinDispatchWorkload = `
Integer total = 0;
for (Integer i = 0; i < 50; i++) {
    total += Integer.valueOf('2');
    if (String.isNotBlank('x')) total += 1;
    total += Math.abs(-3);
    if (string.ISBLANK(' ')) total += 1;
}
System.debug('dispatch:' + total);
`

func TestBuiltinDispatchWorkloadOutcome(t *testing.T) {
	program, err := CompileAnonymous(builtinDispatchWorkload)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "dispatch:350" {
		t.Fatalf("debug = %q", result.Debug)
	}
}

func BenchmarkBuiltinDispatchWorkload(b *testing.B) {
	program, err := CompileAnonymous(builtinDispatchWorkload)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := Execute(program, nil)
		if err != nil || len(result.Debug) != 1 || result.Debug[0] != "dispatch:350" {
			b.Fatalf("result = %q, error = %v", result.Debug, err)
		}
	}
}

func TestBuiltinPrecedenceAfterRegistration(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = api
			if !machine.shouldUseBuiltinStaticPrecedence("Math.abs", "Math.abs") {
				t.Fatal("builtin did not win before registration")
			}
			if err := machine.RegisterClass(Class{Name: "Math"}); err != nil {
				t.Fatal(err)
			}
			if machine.shouldUseBuiltinStaticPrecedence("Math.abs", "Math.abs") {
				t.Fatal("late registered class did not take precedence")
			}
			if !machine.shouldUseBuiltinStaticPrecedence("System.Math.abs", "Math.abs") {
				t.Fatal("qualified builtin lost precedence")
			}
		})
	}
}
