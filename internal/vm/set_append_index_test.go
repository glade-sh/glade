package vm

import (
	"strings"
	"testing"
)

// setAppendIndexProgram exercises Sets past the tracked-index threshold:
// numeric, SObject and mixed Object elements, duplicates, removal, clones,
// shared references, nesting and iteration order.
const setAppendIndexProgram = `
Set<Integer> a = new Set<Integer>();
for (Integer i = 0; i < 100; i++) { a.add(Math.mod(i * 37, 61)); }
System.debug(a.size() + ' ' + a);
System.debug(a.add(5) + ' ' + a.add(1000) + ' ' + a.contains(1000) + ' ' + a.contains(-1));
a.remove(5);
System.debug(a.contains(5) + ' ' + a.add(5) + ' ' + a.size());
Set<Integer> b = a;
b.add(2000);
System.debug(a.contains(2000) + ' ' + a.size());
Set<Integer> c = a.clone();
c.add(3000);
a.add(4000);
System.debug(a.contains(3000) + ' ' + c.contains(4000) + ' ' + a.size() + ' ' + c.size());
List<Set<Integer>> holder = new List<Set<Integer>>{ a };
holder[0].add(5000);
System.debug(a.contains(5000) + ' ' + holder[0].size());
Map<String, Set<Integer>> byKey = new Map<String, Set<Integer>>{ 'k' => a };
a.add(6000);
System.debug(byKey.get('k').contains(6000) + ' ' + byKey.get('k').size());
Set<Decimal> d = new Set<Decimal>();
for (Integer i = 0; i < 50; i++) { d.add(Decimal.valueOf(i) / 4); }
System.debug(d.size() + ' ' + d.contains(1.0) + ' ' + d.contains(1.00) + ' ' + d.add(0.25) + ' ' + d.add(0.250));
Set<Long> l = new Set<Long>();
for (Long i = 0; i < 40; i++) { l.add(i * 3000000000L); }
System.debug(l.size() + ' ' + l.contains(3000000000L) + ' ' + l.contains(3L));
Set<Account> s = new Set<Account>();
List<Account> rows = new List<Account>();
for (Integer i = 0; i < 40; i++) { Account r = new Account(Name = 'A' + Math.mod(i, 35)); rows.add(r); s.add(r); }
System.debug(s.size());
rows[3].Name = 'changed';
System.debug(s.contains(rows[3]) + ' ' + s.contains(new Account(Name = 'A4')) + ' ' + s.contains(new Account(Name = 'changed')) + ' ' + s.add(new Account(Name = 'A4')));
Set<Object> o = new Set<Object>();
for (Integer i = 0; i < 40; i++) { o.add(i); o.add('s' + i); }
System.debug(o.size() + ' ' + o.contains(7) + ' ' + o.contains('s7') + ' ' + o.add(7) + ' ' + o.add('s7') + ' ' + o.add(7.0));
Integer sum = 0;
for (Integer x : a) { sum = sum * 31 + x; }
System.debug(sum);
a.retainAll(new Set<Integer>{ 1, 2, 3, 1000, 6000 });
a.add(7000);
System.debug(a);
a.clear();
for (Integer i = 0; i < 40; i++) { a.add(i); }
System.debug(a.size() + ' ' + a.contains(39));
`

// The outcome rows were produced by train D2 (013d8af346) before the owned
// append and tracked index: the Sets must behave identically.
const setAppendIndexWant = `61 {0, 37, 13, 50, 26, 2, 39, 15, 52, 28, ...}
false true true false
false true 62
true 63
false false 64 64
true 65
true 66
50 false false false true
40 true false
35
false true false false
80 true true false false true
868837190
{2, 1, 3, 1000, 6000, 7000}
40 true`

func TestSetAppendIndexMatchesScan(t *testing.T) {
	program, err := CompileAnonymous(setAppendIndexProgram)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(result.Debug, "\n"); got != setAppendIndexWant {
		t.Fatalf("debug lines:\n%s\nwant:\n%s", got, setAppendIndexWant)
	}
}
