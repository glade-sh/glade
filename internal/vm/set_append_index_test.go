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

// A shallow holder of a Set (a stale local, the static snapshot an async drain
// restores) shares the backing with the live Set until the live Set grows past
// its capacity, exactly as a List holder does. An in-place element refresh on
// the live collection (alias propagation writes value.Set[i] / value.List[i])
// is then visible through the holder, and a restore of the holder carries the
// refreshed element. 013d8af346 copied a non-String, non-Id Set on every add, so
// such a holder kept the element it had. Natively the Set holds the object
// itself, and a mutation of it is visible through every holder (surf_collections
// cs03, APIs 62.0 and 67.0: "setIn=true|setAfter=false|setSize=1"), so the shared
// view is the native-consistent one; no row restores a snapshot directly.
func TestSetSnapshotSharesInPlaceRefreshLikeList(t *testing.T) {
	account := func(name string) Value {
		value := Object("Account")
		value.Fields["Name"] = String(name)
		return value
	}
	a, b, c := account("a"), account("b"), account("c")
	live := Set()
	live.Type = "Set<Account>"
	live.Set = make([]Value, 0, 8)
	live.setInsertionHashes = make([]setInsertionHash, 0, 8)
	live.primitiveSetState = &primitiveSetState{items: live.Set, hashes: live.setInsertionHashes}
	list := List()
	list.List = make([]Value, 0, 8)
	for _, item := range []Value{a, b} {
		live = appendSetEntry(live, item, setInsertionHash{key: mapKey(item), tracked: true})
		list.List = append(list.List, item)
	}
	setHolder, listHolder := live, list
	live = appendSetEntry(live, c, setInsertionHash{key: mapKey(c), tracked: true})
	list.List = append(list.List, c)

	refreshed := a
	refreshed.Fields = map[string]Value{"Name": String("refreshed")}
	live.Set[0] = refreshed
	list.List[0] = refreshed
	if got := setHolder.Set[0].Fields["Name"].Text; got != "refreshed" {
		t.Fatalf("Set holder element 0 = %q, want the refreshed element as for a List", got)
	}
	if got := listHolder.List[0].Fields["Name"].Text; got != "refreshed" {
		t.Fatalf("List holder element 0 = %q, want refreshed", got)
	}
	if len(setHolder.Set) != 2 || len(listHolder.List) != 2 {
		t.Fatal("a holder must keep its own length")
	}

	// The holder no longer owns the backing: its own add copies, so the live
	// Set keeps its third element.
	d := account("d")
	setHolder = appendSetEntry(setHolder, d, setInsertionHash{key: mapKey(d), tracked: true})
	if live.Set[2].Fields["Name"].Text != "c" || setHolder.Set[2].Fields["Name"].Text != "d" || &setHolder.Set[0] == &live.Set[0] {
		t.Fatal("a stale holder's add wrote into the live Set's backing")
	}
}
