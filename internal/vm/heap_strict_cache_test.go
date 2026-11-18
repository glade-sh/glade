package vm

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

type strictHeapOutcome struct {
	err        string
	heap       int
	fullWalk   int
	violations []LimitViolation
	lists      map[string]int
	scalars    map[string]string
	priced     int
}

// runStrictHeapCase runs source in strict mode with every check either priced
// where a plan holds or walked in full. The package tests also compare every
// priced check with the full walk (heapVerifyAgainstFullWalk).
func runStrictHeapCase(t *testing.T, source string, heapCap int, setup func(*testing.T, *VM), plansOff bool) strictHeapOutcome {
	t.Helper()
	previous := heapStatementPlansOff
	heapStatementPlansOff = plansOff
	defer func() { heapStatementPlansOff = previous }()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	if heapCap > 0 {
		caps := defaultLimitCaps()
		caps.HeapSize = heapCap
		machine.SetLimitCaps(caps)
	}
	if setup != nil {
		setup(t, machine)
	}
	_, runErr := machine.Execute(program)
	out := strictHeapOutcome{
		heap:       machine.limits.HeapSize,
		fullWalk:   machine.currentHeapSizeFullWalk(),
		violations: append([]LimitViolation(nil), machine.limitViolations...),
		lists:      map[string]int{},
		scalars:    map[string]string{},
		priced:     machine.heapCheck.priced,
	}
	if runErr != nil {
		out.err = runErr.Error()
	}
	for name, value := range machine.Globals {
		switch {
		case value.Kind == ValueList:
			out.lists[name] = len(value.List)
		case heapScalarKind(value.Kind):
			out.scalars[name] = fmt.Sprintf("%s:%d:%s:%v:%v", value.Kind, value.Int, value.Text, value.Decimal, value.Bool)
		}
	}
	return out
}

func registerHeapAppendHolder(t *testing.T, machine *VM) {
	t.Helper()
	statics := map[string]Field{
		"items": {Name: "items", Static: true, Type: "List<Account>", Value: List()},
	}
	if err := machine.RegisterClass(Class{Name: "HeapAppendHolder", StaticFields: statics}); err != nil {
		t.Fatal(err)
	}
}

// Statements priced from the cached check must give every strict outcome the
// full walk gives: the heap count, the LimitException text and the statement
// it stops on, recorded violations and the final variables.
func TestStrictHeapPricedChecksMatchFullWalk(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		heapCap   int
		setup     func(*testing.T, *VM)
		wantErr   string
		minPriced int
	}{
		{
			name:      "records",
			source:    strictListAppendSource(300) + ` Integer h = Limits.getHeapSize();`,
			minPriced: 600,
		},
		{
			name:      "local cap stops on the same add",
			source:    strictListAppendSource(2000),
			heapCap:   30000,
			wantErr:   "Too many heapSize: ",
			minPriced: 100,
		},
		{
			name:      "scalar locals and while",
			source:    `String s = ''; List<String> names = new List<String>(); Integer j = 0; while (j < 200) { s = s + 'ab'; names.add('n' + j); j++; }`,
			minPriced: 600,
		},
		{
			name:    "string growth passes a local cap",
			source:  `String s = ''; for (Integer i = 0; i < 1000; i++) { s = s + 'xxxxxxxxxx'; }`,
			heapCap: 5000,
			wantErr: "Too many heapSize: ",
		},
		{
			name:      "scalar local declared in the body",
			source:    `List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 200; i++) { Integer k = i * 2; l.add(k); l.add(-k); }`,
			minPriced: 600,
		},
		{
			name:      "local alias of the list",
			source:    `List<Account> l = new List<Account>(); List<Account> l2 = l; for (Integer i = 0; i < 200; i++) { l.add(new Account(Name = 'a' + i)); } Integer h = Limits.getHeapSize();`,
			minPriced: 200,
		},
		{
			name:   "static alias of the list",
			source: `List<Account> l = new List<Account>(); HeapAppendHolder.items = l; for (Integer i = 0; i < 200; i++) { l.add(new Account(Name = 'a' + i)); } Integer n = HeapAppendHolder.items.size();`,
			setup:  registerHeapAppendHolder,
		},
		{
			name:   "list nested in another local",
			source: `List<Account> l = new List<Account>(); List<List<Account>> nest = new List<List<Account>>{ l }; for (Integer i = 0; i < 200; i++) { l.add(new Account(Name = 'a' + i)); } Integer n = nest[0].size();`,
		},
		{
			name:   "condition calls a list method",
			source: `List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 300 && l.size() < 150; i++) { l.add(i); }`,
		},
		{
			// The condition shrinks another list between statement checks;
			// a priced add must not reuse the cache from before it.
			name:   "condition mutates another list",
			source: `List<String> src = new List<String>(); for (Integer i = 0; i < 100; i++) { src.add('payload-' + i); } List<Integer> l = new List<Integer>(); while (!src.isEmpty() && src.remove(0) != null) { l.add(1); }`,
		},
		{
			// Map.put grows a value in place, leaving the map's identity and
			// size as they were; only the evaluation count shows it ran.
			name:   "condition grows a map value in place",
			source: `Map<String, String> m = new Map<String, String>{ 'k' => 'v' }; List<Integer> l = new List<Integer>(); while (m.put('k', m.get('k') + 'xxxxxxxx') != null && l.size() < 60) { l.add(1); }`,
		},
		{
			name:   "catch binds an exception",
			source: `List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 50; i++) { try { Integer z = 1 / (i - 25); } catch (MathException e) { l.add(-1); } l.add(i); }`,
		},
		{
			name:   "body writes a record field",
			source: `List<Account> l = new List<Account>(); Account a = new Account(Name = 'seed'); for (Integer i = 0; i < 100; i++) { l.add(new Account(Name = 'a' + i)); a.Name = a.Name + 'x'; }`,
		},
		{
			name:   "list rebound in the loop",
			source: `List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 100; i++) { l.add(i); if (Math.mod(i, 10) == 0) { l = new List<Integer>(); } }`,
		},
		{
			name:      "record local appended",
			source:    `List<Account> l = new List<Account>(); for (Integer i = 0; i < 200; i++) { Account a = new Account(Name = 'a' + i); l.add(a); } Integer h = Limits.getHeapSize();`,
			minPriced: 600,
		},
		{
			name:      "record fields written then appended",
			source:    `List<Account> l = new List<Account>(); for (Integer i = 0; i < 200; i++) { Account a = new Account(); a.Name = 'a' + i; a.Description = 'longer description ' + i; l.add(a); }`,
			minPriced: 1000,
		},
		{
			name:      "record local rebound without an append",
			source:    `List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 200; i++) { Account scratch = new Account(Name = 'scratch' + i); scratch.Description = 'd' + i; l.add(i); }`,
			minPriced: 800,
		},
		{
			name:      "appended record written and rebound",
			source:    `List<Account> l = new List<Account>(); Account a = new Account(Name = 'seed'); for (Integer i = 0; i < 200; i++) { a = new Account(Name = 'a' + i); l.add(a); a.Name = 'renamed after add ' + i; l.add(a); }`,
			minPriced: 800,
		},
		{
			name:      "record appended to two lists",
			source:    `List<Account> l = new List<Account>(); List<SObject> m = new List<SObject>(); for (Integer i = 0; i < 200; i++) { Account a = new Account(Name = 'a' + i); l.add(a); m.add(a); }`,
			minPriced: 800,
		},
		{
			name:      "local cap stops on the same record bind",
			source:    `List<Account> l = new List<Account>(); for (Integer i = 0; i < 2000; i++) { Account a = new Account(Name = 'a' + i); a.Description = 'description ' + i; l.add(a); }`,
			heapCap:   30000,
			wantErr:   "Too many heapSize: ",
			minPriced: 100,
		},
		{
			name:   "record local set to null",
			source: `List<Account> l = new List<Account>(); for (Integer i = 0; i < 100; i++) { Account a = new Account(Name = 'a' + i); a = null; l.add(new Account(Name = 'b' + i)); }`,
		},
		{
			// The record bound before the loop stays reachable through keep,
			// so rebinding its local must not drop its size.
			name:   "rebound record still held by a list from before the loop",
			source: `Account a = new Account(Name = 'seed'); List<Account> keep = new List<Account>{ a }; List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 50; i++) { a = new Account(Name = 'a' + i); l.add(i); }`,
		},
		{
			name:   "if in the loop body",
			source: `List<Account> l = new List<Account>(); List<Integer> odd = new List<Integer>(); for (Integer i = 0; i < 200; i++) { if (Math.mod(i, 2) == 1) { odd.add(i); } if (i > 50) { l.add(new Account(Name = 'a' + i)); } else { Account a = new Account(Name = 'b' + i); l.add(a); } }`,
			// Math.mod is a call, so the first if walks; the second prices.
			minPriced: 600,
		},
		{
			name:      "nested loops with break and continue",
			source:    `List<Account> l = new List<Account>(); for (Integer i = 0; i < 20; i++) { Integer j = 0; while (true) { j++; if (j > 15) { break; } if (j == 3) { continue; } l.add(new Account(Name = 'a' + i + '-' + j)); } }`,
			minPriced: 1000,
		},
		{
			name:    "local cap stops inside an if",
			source:  `List<String> l = new List<String>(); for (Integer i = 0; i < 2000; i++) { if (i >= 0) { l.add('row-' + i); } }`,
			heapCap: 10000,
			wantErr: "Too many heapSize: ",
		},
		{
			// The if condition grows a map in place and runs no statement,
			// so the if's own check must walk.
			name:   "if condition grows a map value in place",
			source: `Map<String, String> m = new Map<String, String>{ 'k' => 'v' }; List<Integer> l = new List<Integer>(); for (Integer i = 0; i < 60; i++) { if (m.put('k', m.get('k') + 'xxxxxxxx') == null) { l.add(0); } l.add(i); }`,
		},
		{
			name:   "loop ends with records bound in its body",
			source: `List<Account> l = new List<Account>(); for (Integer i = 0; i < 100; i++) { Account kept = new Account(Name = 'k' + i); Account dropped = new Account(Name = 'd' + i); l.add(kept); } Integer h = Limits.getHeapSize();`,
		},
		{
			name:   "two lists",
			source: `List<Account> l = new List<Account>(); List<Contact> c = new List<Contact>(); for (Integer i = 0; i < 150; i++) { l.add(new Account(Name = 'a' + i)); c.add(new Contact(LastName = 'c' + i)); }`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := runStrictHeapCase(t, tc.source, tc.heapCap, tc.setup, true)
			got := runStrictHeapCase(t, tc.source, tc.heapCap, tc.setup, false)
			if want.priced != 0 {
				t.Fatalf("plans off priced %d checks", want.priced)
			}
			if tc.wantErr != "" && !strings.Contains(want.err, tc.wantErr) {
				t.Fatalf("full walk err = %q, want %q", want.err, tc.wantErr)
			}
			if tc.wantErr == "" && want.err != "" {
				t.Fatalf("full walk err = %q", want.err)
			}
			if got.priced < tc.minPriced {
				t.Fatalf("priced %d checks, want at least %d", got.priced, tc.minPriced)
			}
			got.priced, want.priced = 0, 0
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("priced outcome differs from full walk:\n got  %+v\n want %+v", got, want)
			}
		})
	}
}

// Permissive mode never checks the heap per statement, so it never prices.
func TestPermissiveHeapNeverPrices(t *testing.T) {
	program, err := CompileAnonymous(strictListAppendSource(50))
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if machine.heapCheck.priced != 0 || machine.heapCheck.valid {
		t.Fatalf("permissive heap cache used: priced %d valid %v", machine.heapCheck.priced, machine.heapCheck.valid)
	}
}

// A host write between two Execute calls is not an Apex statement; the next
// run must not price from the previous run's check.
func TestStrictHeapCacheDoesNotCrossExecute(t *testing.T) {
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	first, err := CompileAnonymous(`List<String> l = new List<String>(); l.add('a');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(first); err != nil {
		t.Fatal(err)
	}
	held := machine.Globals["l"]
	held.List[0] = String(strings.Repeat("y", 4000))
	second, err := CompileAnonymous(`l.add('b');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(second); err != nil {
		t.Fatal(err)
	}
	if got, want := machine.limits.HeapSize, machine.currentHeapSizeFullWalk(); got != want {
		t.Fatalf("heap %d != full walk %d", got, want)
	}
}

// armedStrictMachine runs setup in strict mode and leaves its locals armed
// as the cache of an exact check.
func armedStrictMachine(t *testing.T, setup string) *VM {
	t.Helper()
	program, err := CompileAnonymous(setup)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	machine.heapSizeAndArm()
	return machine
}

// Each condition of a priced check refuses the price when it fails, so the
// check walks instead. Most cannot fail from Apex source today; they guard
// the cache against runtime paths that would make them fail.
func TestStrictHeapPlanGuards(t *testing.T) {
	const setup = `List<Account> l = new List<Account>(); List<Integer> other = new List<Integer>{ 1 }; Account a = new Account(Name = 'x'); Integer n = 0;`
	appendPlan := func(m *VM, item string) heapStatementPlan {
		list := m.Globals["l"]
		return heapStatementPlan{kind: heapPlanAppend, target: "l", item: item, ref: list.Ref, oldLen: len(list.List), refMark: nextValueRef.Load(), instructions: m.work.instructions}
	}
	appendValue := func(m *VM, item Value) {
		list := m.Globals["l"]
		list.List = append(list.List, item)
		m.Globals["l"] = list
	}
	cases := []struct {
		name  string
		price func(*VM) bool
	}{
		{"unchanged scalar plan prices", func(m *VM) bool {
			_, ok := m.heapSizeFromPlan(heapStatementPlan{kind: heapPlanScalar, instructions: m.work.instructions})
			return !ok
		}},
		{"container local turned scalar", func(m *VM) bool {
			m.Globals["a"] = Null
			_, ok := m.heapSizeFromPlan(heapStatementPlan{kind: heapPlanScalar, instructions: m.work.instructions})
			return ok
		}},
		{"class store", func(m *VM) bool {
			m.heapStaticFlatOK = false
			_, ok := m.heapSizeFromPlan(heapStatementPlan{kind: heapPlanScalar, instructions: m.work.instructions})
			return ok
		}},
		{"nested statement", func(m *VM) bool {
			plan := heapStatementPlan{kind: heapPlanScalar, instructions: m.work.instructions}
			m.work.instructions++
			_, ok := m.heapSizeFromPlan(plan)
			return ok
		}},
		{"statement since the check", func(m *VM) bool {
			m.work.instructions += 2
			return m.planHeapStatement(ir.Instruction{Op: ir.OpAssign, Name: "n", Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "1"}}).kind != heapPlanNone
		}},
		{"other scope", func(m *VM) bool {
			m.Globals = maps.Clone(m.Globals)
			m.work.instructions++
			return m.planHeapStatement(ir.Instruction{Op: ir.OpAssign, Name: "n", Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "1"}}).kind != heapPlanNone
		}},
		{"record field written outside the window", func(m *VM) bool {
			m.work.instructions++
			return m.planHeapStatement(ir.Instruction{Op: ir.OpAssign, Name: "a.Name", Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "'y'"}}).kind != heapPlanNone
		}},
		{"another local replaced during an append", func(m *VM) bool {
			plan := appendPlan(m, "")
			appendValue(m, Int(1))
			m.Globals["other"] = List(Int(1), Int(2))
			_, ok := m.heapSizeFromPlan(plan)
			return ok
		}},
		{"append added two values", func(m *VM) bool {
			plan := appendPlan(m, "")
			appendValue(m, Int(1))
			appendValue(m, Int(2))
			_, ok := m.heapSizeFromPlan(plan)
			return ok
		}},
		{"appended record older than the statement", func(m *VM) bool {
			plan := appendPlan(m, "")
			appendValue(m, m.Globals["a"])
			_, ok := m.heapSizeFromPlan(plan)
			return ok
		}},
		{"appended record local changed", func(m *VM) bool {
			plan := appendPlan(m, "a")
			record := m.Globals["a"]
			appendValue(m, record)
			record.Fields = map[string]Value{"Name": String("z")}
			m.Globals["a"] = record
			_, ok := m.heapSizeFromPlan(plan)
			return ok
		}},
		{"record local append stored another record", func(m *VM) bool {
			plan := appendPlan(m, "a")
			other := Object("Account")
			other.Fields["Name"] = String("other")
			appendValue(m, other)
			_, ok := m.heapSizeFromPlan(plan)
			return ok
		}},
		{"appended record and its list left scope", func(m *VM) bool {
			plan := appendPlan(m, "a")
			appendValue(m, m.Globals["a"])
			if _, ok := m.heapSizeFromPlan(plan); !ok {
				t.Fatal("append of a record local did not price")
			}
			m.armHeapCheck()
			delete(m.Globals, "l")
			delete(m.Globals, "a")
			_, ok := m.heapSizeFromPlan(heapStatementPlan{kind: heapPlanScalar, instructions: m.work.instructions})
			return ok
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.price(armedStrictMachine(t, setup)) {
				t.Fatal("guard did not hold")
			}
		})
	}
}
