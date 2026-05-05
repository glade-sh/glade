package vm

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/trace"
)

func TestExecAssertEquals(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals(2, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteStopsWhenContextCanceled(t *testing.T) {
	program, err := CompileAnonymous("System.assert(true);")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	machine := New(nil)
	machine.SetContext(ctx)
	if _, err := machine.Execute(program); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func traceHas(events []trace.Event, name, category string) bool {
	for _, event := range events {
		if event.Name == name && event.Category == category {
			return true
		}
	}
	return false
}

func TestExecVariablesAndDebug(t *testing.T) {
	program, err := CompileAnonymous(`
Integer x = 1 + 1;
x = x * 3;
System.debug('x=' + x);
System.assertEquals(6, x);
`)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	result, err := Execute(program, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "x=6" {
		t.Fatalf("stdout = %q", got)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "x=6" {
		t.Fatalf("debug = %#v", result.Debug)
	}
}

func TestCompileSkipsCommentsAndSafeNavigation(t *testing.T) {
	program, err := CompileAnonymous(`
String value = 'trail';
// A line comment should not become divide tokens.
/* Nor should a block comment. */
System.assertEquals('trail', value?.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsApexCastSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Object raw = 'trail';
String value = (String)raw;
System.assertEquals('trail', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsListIndexSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>{'spruce', 'birch'};
System.assertEquals('spruce', values[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsTernarySyntax(t *testing.T) {
	program, err := CompileAnonymous(`
String value = true ? 'spruce' : null.toString();
System.assertEquals('spruce', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullCoalescingSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Integer> values = new Map<String, Integer>();
values.put('spruce', (values.get('spruce') ?? 0) + 1);
System.assertEquals(1, values.get('spruce'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsPrefixIncrementInForUpdate(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0; i < 3; ++i) {
	total += i;
}
System.assertEquals(3, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapLiteralInitializer(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, String> params = new Map<String, String> { 'orderId' => '001000000000001AAA' };
System.assertEquals('001000000000001AAA', params.get('orderId'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsPostfixFieldAccess(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'spruce');
Object raw = account;
System.assertEquals('spruce', ((Account)raw).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocalVariablesAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001000000000001AAA';
System.assertEquals(accountId, accountid);
accountID = '001000000000002AAA';
System.assertEquals('001000000000002AAA', accountId);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesCollectionElementType(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return values.size();`)
	if err != nil {
		t.Fatal(err)
	}
	listStringProgram, err := CompileAnonymous(`return values.size() + 10;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'spruce'};
System.assertEquals(11, Util.count(names));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<String>"}}, Program: listStringProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesTypedNullVariables(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	listIDProgram, err := CompileAnonymous(`return 2;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Id> ids = null;
System.assertEquals(2, Util.count(ids));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Id>"}}, Program: listIDProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLListAssignmentKeepsDeclaredType(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	listStringProgram, err := CompileAnonymous(`return 2;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> accounts = [SELECT Id FROM Account LIMIT 1];
System.assertEquals(1, Util.count(accounts));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.Org = &org
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<String>"}}, Program: listStringProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIDsToSObjectsUsesIDPrefix(t *testing.T) {
	machine := New(nil)
	converted, ok := machine.deleteIDsToSObjects(List(platformScalar("Id", "001000000000001AAA")))
	if !ok {
		t.Fatal("delete id list was not converted")
	}
	if converted.Kind != ValueList || len(converted.List) != 1 {
		t.Fatalf("converted = %#v", converted)
	}
	if converted.List[0].Type != "Account" {
		t.Fatalf("type = %q, want Account", converted.List[0].Type)
	}
	if id, _ := platformScalarText(converted.List[0].Fields["Id"], "Id"); id != "001000000000001AAA" {
		t.Fatalf("id = %#v", id)
	}
}

func TestExecCoercesNumericVariablesAndCollections(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 1;
total = 2;
System.assertEquals(2.5, total + 0.5);
List<Decimal> totals = new List<Decimal>();
totals.add(1);
System.assertEquals(1.25, totals.get(0) + 0.25);
Map<String,Decimal> byName = new Map<String,Decimal>();
byName.put('one', 1);
System.assertEquals(1.75, byName.get('one') + 0.75);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRejectsInvalidCoercions(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"decimal to integer assignment", "Integer count = 1.5;"},
		{"string to boolean assignment", "Boolean ready = 'true';"},
		{"decimal list item", "List<Integer> counts = new List<Integer>(); counts.add(1.5);"},
		{"integer map key", "Map<String,Integer> counts = new Map<String,Integer>(); counts.put(1, 2);"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err == nil {
				t.Fatalf("expected coercion error")
			}
		})
	}
}

func TestExecAssertFailure(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals(3, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected assertion failure")
	}
}

func TestExecRuntimeErrorStackUsesStatementSourcePosition(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(3, 1 + 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v", err)
	}
	if len(runtimeErr.Stack) == 0 || runtimeErr.Stack[0].Line != 2 || runtimeErr.Stack[0].Column != 1 {
		t.Fatalf("stack = %#v", runtimeErr.Stack)
	}
}

func TestExecCollectionsAndTrace(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2};
xs.add(3);
Set<String> names = new Set<String>();
names.add('a');
names.add('a');
Map<String,Integer> counts = new Map<String,Integer>();
counts.put('a', xs.size());
System.assertEquals(3, xs.get(2));
System.assertEquals(1, names.size());
System.assertEquals(3, counts.get('a'));
System.assert(counts.containsKey('a'));
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trace) != len(program.Instructions)+1 {
		t.Fatalf("trace length = %d, want %d", len(result.Trace), len(program.Instructions)+1)
	}
	if result.TraceFormat != "chrome-trace-event" {
		t.Fatalf("trace format = %q", result.TraceFormat)
	}
	first := result.Trace[0]
	if first.Name != "apex.statement.declare" || first.Category != "apex.statement" || first.Phase != "i" {
		t.Fatalf("trace event shape = %#v", first)
	}
	if first.Args["sourceOffset"] == 0 {
		t.Fatalf("trace missing source offset: %#v", first)
	}
	if first.Args["line"] != 2 || first.Args["column"] != 1 {
		t.Fatalf("trace source position = %#v", first.Args)
	}
	last := result.Trace[len(result.Trace)-1]
	if last.Name != "apex.limits" || last.Category != "apex.limits" {
		t.Fatalf("trace missing limits summary: %#v", last)
	}
}

func TestExecForBreakContinueDoWhileSwitchAndEnhancedFor(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2, 3, 4};
Integer total = 0;
for (Integer i = 0; i < xs.size(); i++) {
	if (i == 1) {
		continue;
	}
	if (i == 3) {
		break;
	}
	total = total + xs.get(i);
}
Integer seen = 0;
for (Integer x : xs) {
	seen = seen + x;
}
Integer once = 0;
do {
	once++;
} while (once < 1);
String label = '';
switch on total {
	when 1 { label = 'one'; }
	when 4 { label = 'four'; }
	when else { label = 'other'; }
}
System.assertEquals(4, total);
System.assertEquals(10, seen);
System.assertEquals(1, once);
System.assertEquals('four', label);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchBreakOnlyExitsSwitchAndContinueReachesLoop(t *testing.T) {
	program, err := CompileAnonymous(`
Integer seen = 0;
Integer afterSwitch = 0;
for (Integer i = 0; i < 4; i++) {
	switch on i {
		when 0 {
			seen = seen + 1;
			break;
		}
		when 1 {
			continue;
		}
		when else {
			seen = seen + 10;
		}
	}
	afterSwitch++;
}
System.assertEquals(21, seen);
System.assertEquals(3, afterSwitch);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecFinallyPreservesAndOverridesLoopSignals(t *testing.T) {
	program, err := CompileAnonymous(`
Integer cleaned = 0;
Integer seen = 0;
for (Integer i = 0; i < 4; i++) {
	try {
		if (i == 1) {
			continue;
		}
		if (i == 2) {
			break;
		}
		seen++;
	} finally {
		cleaned++;
	}
}
System.assertEquals(1, seen);
System.assertEquals(3, cleaned);
Integer overridden = 0;
while (overridden < 1) {
	try {
		break;
	} finally {
		overridden++;
		continue;
	}
}
System.assertEquals(1, overridden);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnhancedForBreakContinueAndFinallyThrowOverride(t *testing.T) {
	throwingReturn, err := CompileAnonymous(`
try {
	return 7;
} finally {
	throw new DmlException('finally wins');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Integer> values = new List<Integer>{1, 2, 3, 4};
Integer total = 0;
Integer cleaned = 0;
for (Integer value : values) {
	try {
		if (value == 2) {
			continue;
		}
		if (value == 4) {
			break;
		}
		total = total + value;
	} finally {
		cleaned++;
	}
}
String message = '';
try {
	Util.throwingReturn();
} catch (DmlException e) {
	message = e.getMessage();
}
System.assertEquals(4, total);
System.assertEquals(4, cleaned);
System.assertEquals('finally wins', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.throwingReturn", ReturnType: "Integer", Program: throwingReturn}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTryCatchFinallyThrow(t *testing.T) {
	program, err := CompileAnonymous(`
Integer cleaned = 0;
try {
	throw new MyException();
} catch (Exception e) {
	cleaned = cleaned + 1;
} finally {
	cleaned = cleaned + 2;
}
System.assertEquals(3, cleaned);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredCustomExceptionUsesMessageConstructor(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	throw new NUException('boom');
} catch (Exception e) {
	message = e.getMessage();
}
System.assertEquals('boom', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "NUException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumStaticValue(t *testing.T) {
	program, err := CompileAnonymous(`
Object direction = TriggerConstants.Direction.ToCustomer;
System.assertEquals('ToCustomer', String.valueOf(direction));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "TriggerConstants"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "TriggerConstants.Direction", EnumValues: []string{"ToPlatform", "ToCustomer", "Both"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultiCatchAndRethrow(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	try {
		throw new MyException('boom');
	} catch (Exception e) {
		throw;
	}
} catch (OtherException | MyException e) {
	message = e.getMessage();
}
System.assertEquals('boom', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecFinallyRunsOnReturnAndCanOverrideReturn(t *testing.T) {
	var stdout strings.Builder
	firstProgram, err := CompileAnonymous(`
try {
	return 1;
} finally {
	System.debug('clean');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous(`
try {
	return 1;
} finally {
	return 3;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(1, Util.first());
System.assertEquals(3, Util.second());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(&stdout)
	if err := machine.RegisterMethod(Method{Name: "Util.first", ReturnType: "Integer", Program: firstProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.second", ReturnType: "Integer", Program: secondProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "clean\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExecFinallyRunsBeforeUncaughtThrow(t *testing.T) {
	var stdout strings.Builder
	throwProgram, err := CompileAnonymous(`
try {
	throw new MyException('boom');
} finally {
	System.debug('clean');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	Util.thrower();
} catch (MyException e) {
	System.assertEquals('boom', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(&stdout)
	if err := machine.RegisterMethod(Method{Name: "Util.thrower", ReturnType: "void", Program: throwProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "clean\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExecCatchInterfaceExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = 'no';
try {
	throw new MyException();
} catch (Marker e) {
	caught = 'yes';
}
System.assertEquals('yes', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Marker"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "MyException", Interfaces: []string{"Marker"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExceptionHierarchyMultipleCatchAndMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	throw new QueryException('bad query');
} catch (DmlException e) {
	caught = 'wrong';
} catch (System.QueryException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
	System.assert(e.getLineNumber() > 0);
	String trace = e.getStackTraceString();
	System.assert(trace != '');
}
Exception base = new DmlException('blocked');
System.assertEquals('DmlException', base.getTypeName());
System.assertEquals('QueryException:bad query', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRethrowPreservesOriginalExceptionStack(t *testing.T) {
	throwProgram, err := CompileAnonymous(`
throw new DmlException('boom');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String stack = '';
try {
	try {
		Util.thrower();
	} catch (Exception e) {
		throw;
	}
} catch (DmlException e) {
	stack = e.getStackTraceString();
}
System.assert(stack.contains('Util.thrower'));
System.assert(!stack.contains('rethrow outside catch block'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.thrower", ReturnType: "void", Program: throwProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalArithmetic(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 1.5 + 2;
System.assertEquals('3.5', '' + total);
System.assert(total > 3);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
