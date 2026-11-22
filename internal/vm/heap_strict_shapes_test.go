package vm_test

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
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
	org, index := strictHeapShapesOrg(t)
	return runStrictShapeOnOrg(t, source, caps, plansOff, org, index)
}

type strictShapeFixture struct {
	org   *storage.OrgState
	index typesys.Index
}

func newStrictShapeFixture(t *testing.T) strictShapeFixture {
	t.Helper()
	org, index := strictHeapShapesOrg(t)
	return strictShapeFixture{org: org, index: index}
}

func (fixture strictShapeFixture) run(t *testing.T, source string, caps vm.LimitCaps, plansOff bool) (strictShapeOutcome, int, int) {
	t.Helper()
	org := fixture.org.CloneRuntime()
	return runStrictShapeOnOrg(t, source, caps, plansOff, &org, fixture.index)
}

func runStrictShapeOnOrg(t *testing.T, source string, caps vm.LimitCaps, plansOff bool, org *storage.OrgState, index typesys.Index) (strictShapeOutcome, int, int) {
	t.Helper()
	defer vm.SetHeapStatementPlansOffForTest(plansOff)()
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
Map<Id, Integer> idKeys = new Map<Id, Integer>();
Map<String, Integer> plainMap = new Map<String, Integer>();
Map<String, Integer> mapAlias = plainMap;
Map<Account, Integer> objKeys = new Map<Account, Integer>();
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
		func() string { return "a = new Account(Name = 'rebound-padding-' + hits);" },
		func() string { return "a = new Account(Name = 'rb' + hits); byId.put('001000000000001AAA', a);" },
		func() string {
			return "Contact f = new Contact(LastName = 'f', AccountId = '001000000000001AAA'); ids.add(f.AccountId); f = new Contact(LastName = 'g');"
		},
		func() string {
			return "Contact f = new Contact(LastName = 'f'); f.AccountId = '001000000000002AAA'; ids.add(f.AccountId); f.AccountId = null;"
		},
		func() string {
			return "Contact f = new Contact(LastName = 'f', AccountId = '001000000000003AAA'); rows.add(new Contact(LastName = 'r', AccountId = f.AccountId));"
		},
		func() string {
			return "Account g = new Account(Name = 'g', Id = '001000000000004AAA'); ids.add(g.Id); g = null;"
		},
		func() string { return "idKeys.put('001000000000001', hits); idKeys.put('001000000000001AAA', hits);" },
		func() string {
			return "idKeys.put('001000000000' + String.valueOf(Math.mod(hits, 7)) + '00AAA', hits);"
		},
		func() string {
			return "if (hits == 25) { accs[Math.mod(hits, 5)] = new Account(Name = 'idx' + hits); }"
		},
		func() string { return "mapAlias.put('al' + hits, hits);" },
		func() string { return "if (hits > 55) { break; }" },
		func() string {
			return "try { if (Math.mod(hits, 11) == 10) { Integer z = hits / (hits - hits); } } catch (MathException e) { hits++; }"
		},
		func() string { return "objKeys.put(a, hits);" },
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
	if rng.Intn(2) == 0 {
		b.WriteString("Set<Account> accSet = new Set<Account>(accs);\nfor (Account a : accSet) {\n" + body(2) + "}\n")
	}
	b.WriteString("System.debug('end ' + hits + ' ' + Limits.getHeapSize() + ' ' + rows.size() + ' ' + byId.size());\n")
	return b.String()
}

// The priced outcome of a randomized record-building program equals the
// outcome of the run that walks the heap on every statement, with and without
// a heap cap that stops it at many different statements.
func TestStrictHeapShapesMatchFullWalk(t *testing.T) {
	fixture := newStrictShapeFixture(t)
	seeds := 16
	if testing.Short() {
		seeds = 3
	}
	sync, _ := vm.LimitCapsForProfile("default")
	totalPriced, totalStopped := 0, 0
	for seed := 0; seed < seeds; seed++ {
		source := strictShapeProgram(rand.New(rand.NewSource(int64(seed))), 60)
		base, _, _ := fixture.run(t, source, sync, true)
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
			want, _, _ := fixture.run(t, source, c, true)
			got, priced, walked := fixture.run(t, source, c, false)
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

func TestStrictHeapShapesFixtureIsolation(t *testing.T) {
	fixture := newStrictShapeFixture(t)
	caps, _ := vm.LimitCapsForProfile("default")
	source := strictShapeProgram(rand.New(rand.NewSource(0)), 5)
	for _, plansOff := range []bool{false, true, false} {
		want, wantPriced, wantWalked := runStrictShape(t, source, caps, plansOff)
		got, gotPriced, gotWalked := fixture.run(t, source, caps, plansOff)
		if !reflect.DeepEqual(got, want) || gotPriced != wantPriced || gotWalked != wantWalked {
			t.Fatalf("plansOff=%v: reused fixture outcome or counters differ\n got %+v (%d, %d)\nwant %+v (%d, %d)", plansOff, got, gotPriced, gotWalked, want, wantPriced, wantWalked)
		}
	}
}
