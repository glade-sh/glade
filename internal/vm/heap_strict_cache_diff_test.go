package vm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// strictHeapDiffScripts are heap-heavy loops of the shapes strict mode prices
// (list growth, record locals, field writes, scalar locals) mixed with the
// shapes it walks (maps, sets, nested collections, Object values, calls).
// Each samples Limits.getHeapSize as it runs.
var strictHeapDiffScripts = map[string]string{
	"list records": `List<Account> l = new List<Account>();
for (Integer i = 0; i < 300; i++) {
  l.add(new Account(Name = 'a' + i));
  if (Math.mod(i, 7) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"record locals": `List<Account> l = new List<Account>();
List<SObject> all = new List<SObject>();
for (Integer i = 0; i < 200; i++) {
  Account a = new Account();
  a.Name = 'acct-' + i;
  a.Description = 'description text ' + i;
  l.add(a);
  if (i > 20) { all.add(a); }
  if (Math.mod(i, 11) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"scalars and while": `List<String> l = new List<String>();
List<Integer> n = new List<Integer>();
Integer j = 0;
while (j < 400) {
  l.add('row-' + j);
  n.add(j * 3);
  j++;
  if (Math.mod(j, 13) == 0) { System.debug('s ' + j + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"maps": `Map<String, Account> m = new Map<String, Account>();
Map<Integer, List<String>> buckets = new Map<Integer, List<String>>();
for (Integer i = 0; i < 200; i++) {
  m.put('k' + i, new Account(Name = 'm' + i));
  Integer b = Math.mod(i, 17);
  if (!buckets.containsKey(b)) { buckets.put(b, new List<String>()); }
  buckets.get(b).add('v' + i);
  if (Math.mod(i, 9) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"strings": `String s = '';
String t = 'x';
for (Integer i = 0; i < 400; i++) {
  s = s + 'abcdefghij';
  t = t + i;
  if (Math.mod(i, 17) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + s.length() + ' ' + Limits.getHeapSize());`,
	"nested collections": `List<List<Account>> outerList = new List<List<Account>>();
List<Account> cur = new List<Account>();
outerList.add(cur);
Set<String> names = new Set<String>();
for (Integer i = 0; i < 300; i++) {
  if (Math.mod(i, 25) == 0) { cur = new List<Account>(); outerList.add(cur); }
  cur.add(new Account(Name = 'n' + i));
  names.add('n' + i);
  if (Math.mod(i, 11) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + outerList.size() + ' ' + Limits.getHeapSize());`,
	"nested loops": `List<Account> l = new List<Account>();
for (Integer i = 0; i < 20; i++) {
  Integer j = 0;
  while (true) {
    j++;
    if (j > 20) { break; }
    if (j == 3) { continue; }
    Account keep = new Account(Name = 'k' + i + '-' + j);
    Account drop = new Account(Name = 'd' + i);
    l.add(keep);
  }
  System.debug('s ' + i + ' ' + Limits.getHeapSize());
}
System.debug('end ' + Limits.getHeapSize());`,
	"aliases": `List<Account> l = new List<Account>();
List<Account> l2 = l;
Map<String, List<Account>> holder = new Map<String, List<Account>>{ 'x' => l };
Account seed = new Account(Name = 'seed');
List<Account> keep = new List<Account>{ seed };
for (Integer i = 0; i < 200; i++) {
  l.add(new Account(Name = 'a' + i));
  seed = new Account(Name = 'b' + i);
  seed.Description = 'desc ' + i;
  if (Math.mod(i, 4) == 0) { l2.add(seed); }
  if (Math.mod(i, 9) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize() + ' ' + holder.get('x').size()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"object values": `List<Object> objs = new List<Object>();
Map<String, Object> bag = new Map<String, Object>{ 'objs' => objs };
for (Integer i = 0; i < 300; i++) {
  objs.add('item-' + i);
  if (Math.mod(i, 20) == 0) { objs.add(new List<Integer>{ i }); bag.put('k' + i, new Account(Name = 'o' + i)); }
  if (Math.mod(i, 13) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"rebind and try": `List<Integer> l = new List<Integer>();
List<Account> accs = new List<Account>();
for (Integer i = 0; i < 200; i++) {
  l.add(i);
  try { Integer z = 1 / Math.mod(i, 50); } catch (MathException e) { accs.add(new Account(Name = 'e' + i)); }
  if (Math.mod(i, 40) == 39) { l = new List<Integer>(); }
  Account a = new Account(Name = 'r' + i);
  a = null;
  accs.add(new Account(Name = 'z' + i));
  if (Math.mod(i, 9) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"one string many times": `String big = 'x'.repeat(200);
List<String> l = new List<String>();
for (Integer i = 0; i < 300; i++) {
  l.add(big);
  if (Math.mod(i, 19) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + Limits.getHeapSize());`,
	"pure calls in conditions": `List<Account> l = new List<Account>();
List<Integer> n = new List<Integer>();
Set<String> seen = new Set<String>();
String label = 'row';
Integer k = 0;
while (l.size() < 250) {
  l.add(new Account(Name = 'a' + k));
  if (Math.mod(k, 97) == 0 || Math.abs(k - 50) < 3) { System.debug('s ' + k + ' ' + Limits.getHeapSize()); }
  if (!l.isEmpty() && label.length() == 3) { n.add(Math.max(k, Math.min(k, 7))); }
  if (seen.isEmpty() && k > 100) { seen.add('late'); }
  Integer sz = n.size();
  label = label.length() > 6 ? 'row' : label + 'x';
  k++;
}
System.debug('end ' + sz + ' ' + seen.size() + ' ' + Limits.getHeapSize());`,
	"pure calls fail and shadow": `List<Account> l = new List<Account>();
String nothing;
Integer zero = 0;
for (Integer i = 0; i < 120; i++) {
  l.add(new Account(Name = 'e' + i));
  try { if (nothing.length() > 0) { l.add(new Account()); } } catch (NullPointerException e) { l.add(new Account(Name = 'npe' + i)); }
  try { if (Math.mod(i, zero) == 0) { l.add(new Account()); } } catch (MathException e) { l.add(new Account(Name = 'div' + i)); }
  if (i == 60) { nothing = 'now set'; zero = 5; }
  if (Math.mod(i, 15) == 0) { System.debug('s ' + i + ' ' + Limits.getHeapSize()); }
}
System.debug('end ' + l.size() + ' ' + Limits.getHeapSize());`,
}

type strictHeapDiffOutcome struct {
	err        string
	errType    string
	stack      []StackFrame
	debug      []string
	limits     Limits
	violations []LimitViolation
	heap       int
}

func runStrictHeapDiff(t *testing.T, source string, caps LimitCaps, plansOff bool) (strictHeapDiffOutcome, int) {
	t.Helper()
	defer SetHeapStatementPlansOffForTest(plansOff)()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	machine.SetLimitCaps(caps)
	result, runErr := machine.Execute(program)
	out := strictHeapDiffOutcome{
		debug:      result.Debug,
		limits:     result.Limits,
		violations: result.LimitViolations,
		heap:       machine.limits.HeapSize,
	}
	if runErr != nil {
		out.err = runErr.Error()
		var rt *RuntimeError
		if errors.As(runErr, &rt) {
			out.errType = rt.Type
			out.stack = rt.Stack
		}
	}
	return out, machine.heapCheck.priced
}

// Every strict outcome of a priced run equals the run that walks the heap on
// each statement (the pre-cache algorithm): the LimitException type, text and
// stack line, every Limits.getHeapSize sample, the recorded violations and the
// final heap count. Each script runs with no local cap, under the async
// profile, and under a sweep of local caps that stop it at many different
// statements, including one byte below its final heap.
func TestStrictHeapPricedDifferential(t *testing.T) {
	for name, source := range strictHeapDiffScripts {
		t.Run(name, func(t *testing.T) {
			sync, _ := LimitCapsForProfile("default")
			async, _ := LimitCapsForProfile("strict-async")
			base, _ := runStrictHeapDiff(t, source, sync, true)
			if base.err != "" {
				t.Fatalf("uncapped run err = %q", base.err)
			}
			end := base.heap
			type variant struct {
				label string
				caps  LimitCaps
			}
			variants := []variant{{"default", sync}, {"strict-async", async}}
			for _, cap := range []int{end / 11, end / 7, end / 5, end / 3, end / 2, end * 2 / 3, end * 4 / 5, end * 9 / 10, end - 1} {
				capped := sync
				capped.HeapSize = cap
				variants = append(variants, variant{fmt.Sprintf("cap %d", cap), capped})
			}
			capped := async
			capped.HeapSize = end / 3
			variants = append(variants, variant{fmt.Sprintf("async cap %d", end/3), capped})
			stopped, totalPriced := 0, 0
			for _, v := range variants {
				want, _ := runStrictHeapDiff(t, source, v.caps, true)
				got, priced := runStrictHeapDiff(t, source, v.caps, false)
				totalPriced += priced
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: priced outcome differs from full walk:\n got  %+v\n want %+v", v.label, got, want)
				}
				if strings.Contains(want.err, "Too many heapSize: ") {
					if want.errType != "System.LimitException" || len(want.stack) == 0 || want.stack[0].Line == 0 {
						t.Fatalf("%s: heap exception without a located stack: %+v", v.label, want)
					}
					stopped++
				} else if want.err != "" {
					t.Fatalf("%s: unexpected err %q", v.label, want.err)
				}
			}
			if stopped < 9 {
				t.Fatalf("only %d capped runs threw the heap LimitException", stopped)
			}
			if totalPriced == 0 && name != "maps" {
				t.Fatalf("no check was priced")
			}
		})
	}
}
