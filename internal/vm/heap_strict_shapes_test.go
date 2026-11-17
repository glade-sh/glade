package vm_test

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/vm"
)

type strictShapeOutcome struct {
	err        string
	errType    string
	stack      []vm.StackFrame
	debug      []string
	limits     vm.Limits
	violations []vm.LimitViolation
}

func runStrictShape(t *testing.T, source string, caps vm.LimitCaps, plansOff bool) (strictShapeOutcome, int, int) {
	t.Helper()
	defer vm.SetHeapStatementPlansOffForTest(plansOff)()
	org, index := strictHeapShapesOrg(t)
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, source)
	}
	machine := vm.New(nil)
	machine.SetOrg(org)
	if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
		t.Fatal(err)
	}
	machine.SetLimitMode(vm.LimitModeStrict)
	machine.SetLimitCaps(caps)
	result, runErr := machine.Execute(program)
	out := strictShapeOutcome{debug: result.Debug, limits: result.Limits, violations: result.LimitViolations}
	if runErr != nil {
		out.err = runErr.Error()
		var rt *vm.RuntimeError
		if errors.As(runErr, &rt) {
			out.errType = rt.Type
			out.stack = rt.Stack
		}
	}
	return out, machine.HeapChecksPricedForTest(), machine.HeapChecksWalkedForTest()
}

// strictShapeProgram builds a random heap-heavy program from statement shapes
// that the strict heap cache prices (record builds with field reads, list and
// map appends, loop variables) and shapes it walks.
func strictShapeProgram(rng *rand.Rand, n int) string {
	var b strings.Builder
	b.WriteString(`List<Account> accs = new List<Account>();
for (Integer i = 0; i < ` + fmt.Sprint(n) + `; i++) { accs.add(new Account(Name = 'acct' + i, Description = 'd' + i)); }
insert accs;
List<Contact> rows = new List<Contact>();
List<Id> ids = new List<Id>();
List<Contact> alias = rows;
Map<Id, Account> byId = new Map<Id, Account>();
Map<String, Contact> byKey = new Map<String, Contact>();
Integer hits = 0;
String tag = 'tg';
`)
	newRec := []string{
		"new Contact(LastName = a.Name, AccountId = a.Id)",
		"new Contact(LastName = 'x' + hits, AccountId = a.Id, Title = tag)",
		"new Contact(AccountId = a.Id)",
		"new Contact(LastName = a.Description, Email = 'e' + hits + '@x.test', AccountId = a.Id, Title = a.Name)",
		"new Contact(LastName = 'lit')",
	}
	stmts := []func() string{
		func() string { return "rows.add(" + newRec[rng.Intn(len(newRec))] + ");" },
		func() string { return "ids.add(a.Id);" },
		func() string { return "byId.put(a.Id, a);" },
		func() string { return "byKey.put('k' + hits, " + newRec[rng.Intn(len(newRec))] + ");" },
		func() string { return "Contact c = " + newRec[rng.Intn(len(newRec))] + "; rows.add(c);" },
		func() string {
			return "Contact c = " + newRec[rng.Intn(len(newRec))] + "; c.Title = 't' + hits; byKey.put('c' + hits, c);"
		},
		func() string { return "a.Description = 'w' + hits;" },
		func() string { return "tag = tag + 'x';" },
		func() string {
			return "if (Math.mod(hits, 3) == 0) { rows.add(" + newRec[rng.Intn(len(newRec))] + "); }"
		},
		func() string { return "alias.add(" + newRec[rng.Intn(len(newRec))] + ");" },
		func() string { return "byId.put(a.Id, new Account(Name = a.Name));" },
		func() string { return "Account tmp = new Account(Name = 'tmp' + hits); tmp = null;" },
		func() string { return "if (hits > 40 && Math.mod(hits, 5) == 0) { continue; }" },
	}
	body := func(depth int) string {
		var sb strings.Builder
		for k := 0; k < 2+rng.Intn(3); k++ {
			sb.WriteString("    " + stmts[rng.Intn(len(stmts))]() + "\n    hits++;\n")
		}
		sb.WriteString("    if (Math.mod(hits, 17) == 0) { System.debug('h ' + hits + ' ' + Limits.getHeapSize()); }\n")
		return sb.String()
	}
	b.WriteString("for (Account a : accs) {\n" + body(0) + "}\n")
	if rng.Intn(2) == 0 {
		b.WriteString("for (Integer r = 0; r < 2; r++) {\n  for (Account a : accs) {\n" + body(1) + "  }\n}\n")
	}
	b.WriteString("System.debug('end ' + hits + ' ' + Limits.getHeapSize() + ' ' + rows.size() + ' ' + byId.size());\n")
	return b.String()
}

// The priced outcome of a randomized record-building program equals the
// outcome of the run that walks the heap on every statement, with and without
// a heap cap that stops it at many different statements.
func TestStrictHeapShapesMatchFullWalk(t *testing.T) {
	seeds := 10
	if testing.Short() {
		seeds = 3
	}
	sync, _ := vm.LimitCapsForProfile("default")
	totalPriced, totalStopped := 0, 0
	for seed := 0; seed < seeds; seed++ {
		source := strictShapeProgram(rand.New(rand.NewSource(int64(seed))), 60)
		base, _, _ := runStrictShape(t, source, sync, true)
		if base.err != "" {
			t.Fatalf("seed %d: uncapped run err = %q\n%s", seed, base.err, source)
		}
		end := base.limits.HeapSize
		caps := []int{0, end / 9, end / 4, end / 2, end * 3 / 4, end * 19 / 20, end - 1}
		for _, cap := range caps {
			c := sync
			if cap != 0 {
				c.HeapSize = cap
			}
			want, _, _ := runStrictShape(t, source, c, true)
			got, priced, walked := runStrictShape(t, source, c, false)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("seed %d cap %d: priced outcome differs from full walk:\n got  %+v\n want %+v\n%s", seed, cap, got, want, source)
			}
			totalPriced += priced
			if cap != 0 && strings.Contains(want.err, "Too many heapSize: ") {
				totalStopped++
			}
			if cap == 0 && priced < walked {
				t.Errorf("seed %d: priced %d checks, walked %d; shapes should mostly price", seed, priced, walked)
			}
		}
	}
	if totalPriced == 0 || totalStopped < seeds*3 {
		t.Fatalf("priced %d, stopped %d of %d seeds", totalPriced, totalStopped, seeds)
	}
}
