package vm

import (
	"fmt"
	"testing"
)

func runForBodyScopeProgram(t *testing.T, source string) []string {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	return result.Debug
}

// TestForBodyDeclarationsAvoidPerIterationAllocation guards the loop-body
// binding churn: a body that declares locals must not allocate per iteration
// for the Globals entries (each fresh key allocates one 312-byte Value).
func TestForBodyDeclarationsAvoidPerIterationAllocation(t *testing.T) {
	allocs := func(iterations int) float64 {
		program, err := CompileAnonymous(fmt.Sprintf(`Integer sum=0; for(Integer i=0;i<%d;i++){Integer a=i&3; Integer b=a+1; sum=sum+b;}`, iterations))
		if err != nil {
			t.Fatal(err)
		}
		return testing.AllocsPerRun(3, func() {
			machine := New(nil)
			machine.SetTraceEnabled(false)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	small, large := allocs(100), allocs(1100)
	perIteration := (large - small) / 1000
	t.Logf("ROW allocs/iteration=%.2f", perIteration)
	if perIteration > 3 {
		t.Fatalf("loop body declarations allocate %.2f times per iteration", perIteration)
	}
}

// Scope outcomes the loop-level binding reuse must preserve.
func TestForBodyScopeOutcomeRows(t *testing.T) {
	cases := map[string]struct{ source, want string }{
		"shadow-outer":          {`Integer a=7; for(Integer i=0;i<3;i++){Integer b=i; b++; a=a+b;} System.debug('ROW '+a);`, "13"},
		"after-loop-redeclare":  {`for(Integer i=0;i<3;i++){Integer a=i; String s='x'+a;} Integer a=9; String s='y'; System.debug('ROW '+a+s);`, "9y"},
		"break-continue":        {`Integer t=0; for(Integer i=0;i<10;i++){Integer a=i; if(a==2){continue;} if(a==5){break;} t+=a;} System.debug('ROW '+t);`, "8"},
		"nested":                {`Integer t=0; for(Integer i=0;i<3;i++){Integer a=i; for(Integer j=0;j<3;j++){Integer b=a+j; t+=b;} } System.debug('ROW '+t);`, "18"},
		"uninit-each-iteration": {`String r=''; for(Integer i=0;i<3;i++){Integer a; r+=String.valueOf(a); a=i;} System.debug('ROW '+r);`, "nullnullnull"},
		"final":                 {`Integer t=0; for(Integer i=0;i<3;i++){final Integer a=i; t+=a;} Integer a=4; a=5; System.debug('ROW '+t+a);`, "35"},
		"exception":             {`Integer t=0; try{for(Integer i=0;i<3;i++){Integer a=i; if(a==2){throw new MathException();} t+=a;}}catch(Exception e){} Integer a=3; System.debug('ROW '+t+a);`, "13"},
		"return-in-loop":        {`Integer t=0; for(Integer i=0;i<3;i++){Integer a=i; t+=a;} Integer a=1; System.debug('ROW '+t+a);`, "31"},
		"case-insensitive":      {`Integer t=0; for(Integer i=0;i<3;i++){Integer Abc=i; t+=ABC;} Integer abc=2; System.debug('ROW '+t+abc);`, "32"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := runForBodyScopeProgram(t, tc.source)
			if len(got) != 1 || got[0] != "ROW "+tc.want {
				t.Fatalf("got %v want ROW %s", got, tc.want)
			}
		})
	}
}
