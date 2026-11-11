package vm

import (
	"fmt"
	"strings"
	"testing"
)

// loopKinds builds the same four-iteration body (i = 0..3) as a classic for,
// an enhanced for, a while and a do-while loop. The body sees i in every kind.
func loopKinds(body string) map[string]string {
	return map[string]string{
		"for":     `for (Integer i = 0; i < 4; i++) {` + body + `}`,
		"foreach": `for (Integer i : new List<Integer>{0,1,2,3}) {` + body + `}`,
		"while":   `Integer i = -1; while (i < 3) { i++; ` + body + `}`,
		"dowhile": `Integer i = -1; do { i++; ` + body + `} while (i < 3);`,
	}
}

// runLoopInMethod registers a class with static state and helper methods, puts
// code in Host.run() and returns what it returns.
func runLoopInMethod(t *testing.T, code string) string {
	t.Helper()
	machine := New(nil)
	machine.SetTraceEnabled(false)
	if err := machine.RegisterClass(Class{
		Name: "Host",
		StaticFields: map[string]Field{
			"total": {Name: "total", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)},
			"bound": {Name: "bound", Type: "Integer", Static: true, Value: Int(4), InitialValue: Int(4)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Helper"}); err != nil {
		t.Fatal(err)
	}
	register := func(class, name string, static bool, params []Param, ret, source string) {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatalf("%s.%s: %v", class, name, err)
		}
		if err := machine.RegisterMethod(Method{Name: class + "." + name, ClassName: class, IsStatic: static, ReturnType: ret, Params: params, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	register("Helper", "tag", true, nil, "String", `return 'h';`)
	// callee declares the same local names as its callers' loop bodies.
	register("Host", "callee", true, nil, "Integer", `Integer a = 0; for (Integer j = 0; j < 3; j++) { Integer a2 = j; Integer b = 10; a += a2 + b; } return a;`)
	register("Host", "rec", true, []Param{{Name: "d", Type: "Integer"}}, "String",
		`String r = ''; for (Integer k = 0; k < 2; k++) { Integer v = d * 10 + k; if (d > 0) { r += Host.rec(d - 1); } r += v + ','; } return r;`)
	register("Host", "run", true, nil, "String", code)
	program, err := CompileAnonymous(`System.debug('ROW ' + Host.run());`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err != nil {
		return "ERR " + err.Error()
	}
	if len(result.Debug) != 1 {
		t.Fatalf("debug %v", result.Debug)
	}
	return strings.TrimPrefix(result.Debug[0], "ROW ")
}

func runLoopAnonymous(t *testing.T, code string) string {
	t.Helper()
	program, err := CompileAnonymous(code)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	result, err := machine.Execute(program)
	if err != nil {
		return "ERR " + err.Error()
	}
	if len(result.Debug) != 1 {
		t.Fatalf("debug %v", result.Debug)
	}
	return strings.TrimPrefix(result.Debug[0], "ROW ")
}

type loopRow struct {
	name string
	// body runs inside the loop and appends to r. after runs once the loop ended.
	body, after string
	want        string
	// methodOnly rows need the Host and Helper classes.
	methodOnly bool
	// whileLeaks marks rows where while and do-while bodies keep their locals
	// after the loop today (they have no block scope). Those two kinds are not
	// changed by the reuse, so the rows skip them.
	whileLeaks bool
}

// TestLoopBindingOutcomeRows holds the observable outcomes the cross-iteration
// binding reuse must keep, for every loop kind, in anonymous code and in a
// class method. Rows that shadow a static field or a type with a body local
// pin the cases the reuse rule has to refuse.
func TestLoopBindingOutcomeRows(t *testing.T) {
	rows := []loopRow{
		{name: "locals", body: `Integer a = i * 2; String s = 'x' + a; r += s + ';';`, after: ``, want: "x0;x2;x4;x6;"},
		{name: "redeclare-after", body: `Integer a = i; r += a;`, after: `Integer a = 9; String s = 'y'; r += a + s;`, want: "01239y"},
		{name: "break-continue", body: `Integer a = i; if (a == 1) { continue; } if (a == 3) { break; } r += a;`, after: ``, want: "02"},
		{name: "uninit-resets", body: `Integer a; String s; r += String.valueOf(a) + String.valueOf(s) + ','; a = i; s = 'v' + i;`, after: ``, want: "nullnull,nullnull,nullnull,nullnull,"},
		{name: "uninit-list-resets", body: `List<Integer> l; r += l == null ? 'n' : 'x'; l = new List<Integer>{i};`, after: ``, want: "nnnn"},
		{name: "final", body: `final Integer a = i; r += a;`, after: `Integer a = 4; a = 5; r += a;`, want: "01235"},
		{name: "exception", body: `Integer a = i; if (a == 2) { throw new MathException(); } r += a;`, after: ``, want: "THROWN"},
		{name: "exception-caught-per-iteration", body: `Integer a = i; try { if (a == 1) { throw new MathException(); } r += 'ok' + a; } catch (Exception e) { r += 'c' + a; }`, after: ``, want: "ok0c1ok2ok3"},
		{name: "nested-loops", body: `Integer a = i; for (Integer j = 0; j < 2; j++) { Integer b = a + j; r += b; }`, after: ``, want: "01122334"},
		{name: "nested-same-names", body: `Integer a = i; for (Integer j = 0; j < 2; j++) { Integer a2 = a * 10 + j; r += a2 + ','; } r += a + ';';`, after: ``, want: "0,1,0;10,11,1;20,21,2;30,31,3;"},
		{name: "case-insensitive", body: `Integer Abc = i; r += ABC;`, after: `Integer abc = 7; r += ABC;`, want: "01237", whileLeaks: true},
		{name: "late-assign-visible-after-declaration", body: `Integer a = i; a += 1; r += a;`, after: ``, want: "1234"},
		{name: "ternary-shadows-nothing", body: `Integer a = i; r += (a > 1 ? 'b' : 's');`, after: ``, want: "ssbb"},
		{name: "return-from-loop", body: `Integer a = i; if (a == 2) { return r + 'ret' + a; } r += a;`, after: `r += 'end';`, want: "01ret2", methodOnly: true},
		{name: "callee-same-local-names", body: `Integer a = i; Integer b = 5; Integer c = Host.callee(); r += a + ':' + b + ':' + c + ',';`, after: ``, want: "0:5:33,1:5:33,2:5:33,3:5:33,", methodOnly: true},
		{name: "recursion", body: `Integer v = i; r += '<' + Host.rec(1) + '>' + v;`, after: ``, want: "<0,1,10,0,1,11,>0<0,1,10,0,1,11,>1<0,1,10,0,1,11,>2<0,1,10,0,1,11,>3", methodOnly: true},
		{name: "static-field-before-local-declaration", body: `Host.total += 1; Integer total = 50 + i; r += total + ',';`, after: `r += Host.total;`, want: "50,51,52,53,4", methodOnly: true},
		{name: "unqualified-field-before-local", body: `total += 1; Integer total = 50 + i; r += total + ',';`, after: `r += total;`, want: "50,51,52,53,4", methodOnly: true, whileLeaks: true},
		{name: "field-spelled-differently", body: `Host.total += 1; Integer TOTAL = 50 + i; r += TOTAL + ',';`, after: `r += total;`, want: "50,51,52,53,4", methodOnly: true, whileLeaks: true},
		{name: "type-before-local-declaration", body: `String tag = Helper.tag(); Integer helper = i; r += tag + helper;`, after: `r += Helper.tag();`, want: "h0h1h2h3h", methodOnly: true},
		{name: "type-after-local-declaration-reads-local", body: `Integer helper = i; r += helper;`, after: `r += Helper.tag();`, want: "0123h", methodOnly: true},
		{name: "static-field-reads-in-body-after-local", body: `Integer bound = i; r += bound + '/' + Host.bound + ',';`, after: `r += bound;`, want: "0/4,1/4,2/4,3/4,4", methodOnly: true, whileLeaks: true},
	}
	for _, row := range rows {
		for _, kind := range []string{"for", "foreach", "while", "dowhile"} {
			contexts := []string{"method"}
			if !row.methodOnly {
				contexts = append(contexts, "anonymous")
			}
			if row.whileLeaks && (kind == "while" || kind == "dowhile") {
				continue
			}
			for _, context := range contexts {
				t.Run(fmt.Sprintf("%s/%s/%s", row.name, kind, context), func(t *testing.T) {
					loop := loopKinds(row.body)[kind]
					var got string
					if context == "method" {
						got = runLoopInMethod(t, `String r = ''; `+loop+` `+row.after+` return r;`)
					} else {
						got = runLoopAnonymous(t, `String r = ''; `+loop+` `+row.after+` System.debug('ROW ' + r);`)
					}
					if row.want == "THROWN" {
						if !strings.HasPrefix(got, "ERR ") {
							t.Fatalf("expected the exception, got %q", got)
						}
						return
					}
					if got != row.want {
						t.Fatalf("got %q want %q", got, row.want)
					}
				})
			}
		}
	}
}

// TestLoopHeaderNameSharedWithBodyLocal covers a body local that is spelled
// like a name the loop header resolves to a field (class code) or type.
func TestLoopHeaderNameSharedWithBodyLocal(t *testing.T) {
	t.Run("for-condition-static-field", func(t *testing.T) {
		got := runLoopInMethod(t, `String r = ''; for (Integer i = 0; i < bound; i++) { Integer bound = 2; r += i + ','; } return r;`)
		if got != "0,1,2,3," {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("for-update-static-field", func(t *testing.T) {
		got := runLoopInMethod(t, `String r = ''; for (Integer i = 0; i < 6; i += bound - 2) { Integer bound = 1; r += i + ','; } return r;`)
		if got != "0,2,4," {
			t.Fatalf("got %q", got)
		}
	})
}

// TestLoopBindingsAvoidPerIterationAllocation guards the churn for the loop
// kinds and contexts the reuse covers. It compares a body that declares two
// locals with the same loop whose body declares none: the declarations must
// add no per-iteration allocation (a fresh Globals key allocates one 312-byte
// Value).
func TestLoopBindingsAvoidPerIterationAllocation(t *testing.T) {
	loops := map[string]func(n int, body string) string{
		"for": func(n int, body string) string {
			return fmt.Sprintf(`Integer sum=0; for(Integer i=0;i<%d;i++){%s} return String.valueOf(sum);`, n, body)
		},
		"foreach": func(n int, body string) string {
			return fmt.Sprintf(`Integer sum=0; List<Integer> xs = new List<Integer>(); for(Integer q=0;q<%d;q++){xs.add(q);} for(Integer i : xs){%s} return String.valueOf(sum);`, n, body)
		},
	}
	const declaring, plain = `Integer a=i&3; Integer b=a+1; sum=sum+b;`, `sum=sum+(i&3)+1;`
	for kind, build := range loops {
		for _, context := range []string{"anonymous", "method"} {
			t.Run(kind+"/"+context, func(t *testing.T) {
				allocs := func(n int, body string) float64 {
					source := build(n, body)
					if context == "anonymous" {
						source = strings.Replace(source, ` return String.valueOf(sum);`, ``, 1)
						program, err := CompileAnonymous(source)
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
					method, err := CompileAnonymous(source)
					if err != nil {
						t.Fatal(err)
					}
					entry, err := CompileAnonymous(`Host.run();`)
					if err != nil {
						t.Fatal(err)
					}
					return testing.AllocsPerRun(3, func() {
						machine := New(nil)
						machine.SetTraceEnabled(false)
						if err := machine.RegisterClass(Class{Name: "Host"}); err != nil {
							t.Fatal(err)
						}
						if err := machine.RegisterMethod(Method{Name: "Host.run", ClassName: "Host", IsStatic: true, ReturnType: "String", Program: method}); err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(entry); err != nil {
							t.Fatal(err)
						}
					})
				}
				withDecls := (allocs(1100, declaring) - allocs(100, declaring)) / 1000
				without := (allocs(1100, plain) - allocs(100, plain)) / 1000
				t.Logf("ROW allocs/iteration declaring=%.2f plain=%.2f", withDecls, without)
				if extra := withDecls - without; extra > 2.5 {
					t.Fatalf("body declarations add %.2f allocations per iteration", extra)
				}
			})
		}
	}
}
