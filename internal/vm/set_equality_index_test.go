package vm

import (
	"maps"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// setEqualityProgram compares Sets past the bucketed-equality threshold, for
// every element family: Integer, Long, String, Id, Boolean with null, SObject,
// Decimal next to Integer, and mixed Object Sets, plus containsAll, retainAll
// and removeAll.
const setEqualityProgram = `
Set<Integer> a = new Set<Integer>();
Set<Integer> b = new Set<Integer>();
for (Integer i = 0; i < 40; i++) { a.add(i); b.add(39 - i); }
System.debug((a == b) + ' ' + a.equals(b) + ' ' + (a != b) + ' ' + a.containsAll(b));
b.remove(7); b.add(700);
System.debug((a == b) + ' ' + a.equals(b) + ' ' + a.containsAll(b) + ' ' + b.containsAll(a));
Set<Long> la = new Set<Long>();
Set<Long> lb = new Set<Long>();
for (Long i = 0; i < 30; i++) { la.add(i * 5000000000L); lb.add((29 - i) * 5000000000L); }
System.debug((la == lb) + ' ' + la.containsAll(lb));
Set<String> sa = new Set<String>();
Set<String> sb = new Set<String>();
for (Integer i = 0; i < 30; i++) { sa.add('k' + i); sb.add('k' + (29 - i)); }
System.debug((sa == sb) + ' ' + sa.containsAll(sb));
sb.remove('k3'); sb.add('K3');
System.debug((sa == sb) + ' ' + sa.containsAll(sb) + ' ' + sa.retainAll(sb) + ' ' + sa.size());
Set<Id> ia = new Set<Id>();
Set<Id> ib = new Set<Id>();
for (Integer i = 0; i < 20; i++) {
	String suffix = String.valueOf(i).leftPad(3, '0');
	ia.add(Id.valueOf('001000000000' + suffix));
	ib.add(Id.valueOf('001000000000' + suffix + 'AAA'));
}
System.debug((ia == ib) + ' ' + ia.containsAll(ib));
Set<Object> oa = new Set<Object>();
Set<Object> ob = new Set<Object>();
for (Integer i = 0; i < 20; i++) { oa.add(i); ob.add(Decimal.valueOf(i)); }
System.debug((oa == ob) + ' ' + oa.containsAll(ob) + ' ' + ob.containsAll(oa));
Set<Object> ma = new Set<Object>{ null, true, false, 'x', 1, 2.5 };
Set<Object> mb = new Set<Object>{ 2.5, 1, 'x', false, true, null };
for (Integer i = 0; i < 20; i++) { ma.add('m' + i); mb.add('m' + i); }
System.debug((ma == mb) + ' ' + ma.containsAll(mb));
mb.remove(1); mb.add(1.0);
System.debug((ma == mb) + ' ' + ma.containsAll(mb) + ' ' + mb.containsAll(ma));
Set<Decimal> da = new Set<Decimal>();
Set<Decimal> db = new Set<Decimal>();
for (Integer i = 0; i < 20; i++) { da.add(Decimal.valueOf(i) / 4); db.add(Decimal.valueOf(19 - i) / 4); }
System.debug((da == db) + ' ' + da.containsAll(db));
db.remove(0.25); db.add(0.75);
System.debug((da == db) + ' ' + da.containsAll(db) + ' ' + da.retainAll(db) + ' ' + da.size());
Set<Account> aa = new Set<Account>();
Set<Account> ab = new Set<Account>();
List<Account> rows = new List<Account>();
for (Integer i = 0; i < 20; i++) { Account r = new Account(Name = 'A' + i); rows.add(r); aa.add(r); ab.add(new Account(Name = 'A' + (19 - i))); }
System.debug((aa == ab) + ' ' + aa.containsAll(ab));
rows[2].Name = 'moved';
System.debug((aa == ab) + ' ' + aa.containsAll(ab) + ' ' + aa.retainAll(ab) + ' ' + aa.size());
Set<Integer> x = new Set<Integer>();
for (Integer i = 0; i < 30; i++) { x.add(i); }
Set<Integer> keep = new Set<Integer>{ 3, 5, 29, 100 };
System.debug(x.retainAll(keep) + ' ' + x + ' ' + x.removeAll(new Set<Integer>{ 5 }) + ' ' + x);
Set<Object> na = new Set<Object>();
Set<Object> nb = new Set<Object>();
for (Integer i = 0; i < 18; i++) { na.add(Decimal.valueOf(i) + 0.5); nb.add(Decimal.valueOf(i) + 0.5); }
na.add(0); nb.add(0.0); na.add(Double.valueOf('-0.0')); nb.add(Double.valueOf('0.0')); na.add(3L); nb.add(3); na.add(1.5d); nb.add(Decimal.valueOf('1.5'));
System.debug((na == nb) + ' ' + na.containsAll(nb) + ' ' + nb.containsAll(na) + ' ' + na.size() + ' ' + nb.size());
Set<Object> nc = na.clone();
System.debug(nc.retainAll(nb) + ' ' + nc.size() + ' ' + nc.removeAll(new List<Object>{ 0, 3, 2.5 }) + ' ' + nc.size());
Set<Double> dd1 = new Set<Double>();
Set<Double> dd2 = new Set<Double>();
for (Integer i = 0; i < 20; i++) { dd1.add(i * 0.1); dd2.add((19 - i) * 0.1); }
System.debug((dd1 == dd2) + ' ' + dd1.containsAll(dd2));
List<Object> keepMixed = new List<Object>();
for (Integer i = 0; i < 20; i++) { keepMixed.add(i); }
keepMixed.add(new Account(Name = 'k'));
Set<Integer> y = new Set<Integer>{ 1, 2, 50, 60 };
System.debug(y.retainAll(keepMixed) + ' ' + y);
Set<Double> pd1 = new Set<Double>();
Set<Double> pd2 = new Set<Double>();
for (Integer i = 1; i < 25; i++) { pd1.add(i * 0.25); pd2.add((25 - i) * 0.25); }
pd1.add(0.0); pd2.add(-0.0);
System.debug((pd1 == pd2) + ' ' + pd1.containsAll(pd2) + ' ' + pd1.size() + ' ' + pd2.size());
pd2.remove(-0.0); pd2.add(0.0);
System.debug((pd1 == pd2) + ' ' + pd1.retainAll(pd2) + ' ' + pd1.removeAll(new List<Double>{ 0.25, 0.0 }) + ' ' + pd1.size());
Set<Object> po1 = new Set<Object>();
Set<Object> po2 = new Set<Object>();
for (Integer i = 0; i < 20; i++) { po1.add(i); po2.add(Double.valueOf(i)); }
System.debug((po1 == po2) + ' ' + (po2 == po1) + ' ' + po1.containsAll(po2));
Set<Account> pa1 = new Set<Account>();
Set<Account> pa2 = new Set<Account>();
List<Account> pr = new List<Account>();
for (Integer i = 0; i < 25; i++) {
	Account r = new Account(Name = 'P' + Math.mod(i, 20), Description = 'd' + i);
	pr.add(r);
	pa1.add(r);
	pa2.add(new Account(Description = 'd' + (24 - i), Name = 'P' + Math.mod(24 - i, 20)));
}
System.debug((pa1 == pa2) + ' ' + pa1.size() + ' ' + pa2.size());
pr[0].Name = 'moved';
System.debug((pa1 == pa2) + ' ' + pa1.equals(pa2));
Set<Account> pa3 = pa1.clone();
System.debug((pa1 == pa3) + ' ' + pa3.containsAll(pa1));
insert pr;
Set<Account> pa4 = new Set<Account>();
for (Account r : [SELECT Id, Name, Description FROM Account ORDER BY Description]) { pa4.add(r); }
Set<Account> pa5 = new Set<Account>(pa4);
System.debug((pa4 == pa5) + ' ' + (pa4 == pa1) + ' ' + pa4.size());
Set<Integer> big = new Set<Integer>();
List<Integer> drop = new List<Integer>();
for (Integer i = 0; i < 40; i++) { big.add(i); if (Math.mod(i, 2) == 0) { drop.add(i); } }
drop.add(500);
System.debug(big.removeAll(drop) + ' ' + big.size() + ' ' + big.contains(3) + ' ' + big.contains(4) + ' ' + big.removeAll(drop) + ' ' + big);
Set<String> words = new Set<String>();
List<String> keepWords = new List<String>();
for (Integer i = 0; i < 30; i++) { words.add('w' + i); if (i < 20) { keepWords.add('w' + i); } }
keepWords.add('W1');
System.debug(words.retainAll(keepWords) + ' ' + words.size() + ' ' + words.contains('w19') + ' ' + words.contains('w20'));
List<Set<Integer>> nested1 = new List<Set<Integer>>{ a };
List<Set<Integer>> nested2 = new List<Set<Integer>>{ a.clone() };
System.debug(nested1 == nested2);
`

// The rows were produced by train D2 (1318923037) before the bucketed
// equality, and v0312-parallel-rows (71f4c46288, Decimal scale in collections)
// gives the same rows: the program has no Decimal pair that differs only in
// scale. The comparisons must be identical.
const setEqualityWant = `true true false true
false false false false
true true
true true
false false true 29
true true
true false false
true true
true false false
true true
false true true 19
true true
false false true 19
true {3, 5, 29} true {3, 29}
false false false 22 21
false 22 true 18
true true
true {1, 2}
true true 25 25
true false true 23
true true false
true 25 25
false false
true false
true true 25
true 20 true false false {1, 3, 5, 7, 9, 11, 13, 15, 17, 19, ...}
true 20 true false
true`

func TestSetEqualityIndexMatchesScan(t *testing.T) {
	program, err := CompileAnonymous(setEqualityProgram)
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
	if got := strings.Join(result.Debug, "\n"); got != setEqualityWant {
		t.Fatalf("debug lines:\n%s\nwant:\n%s", got, setEqualityWant)
	}
}

func randomEqualityScalar(rng *rand.Rand, textDecimals bool) Value {
	switch rng.Intn(11) {
	case 0:
		return Null
	case 1:
		return Bool(rng.Intn(2) == 0)
	case 2:
		return Int(int64(rng.Intn(6) - 2))
	case 3:
		value := Int([]int64{-2, 0, 1, 3, 1 << 53, 1<<53 + 1}[rng.Intn(6)])
		value.Type = "Long"
		return value
	case 4:
		if !textDecimals {
			return Int(int64(rng.Intn(4)))
		}
		text := []string{"0", "0.0", "1", "1.0", "1.00", "-1", "2.5", "2.50", "-0.0", "0.5", "0.1", "0.10000000000000001"}[rng.Intn(12)]
		value, err := decimalFromText(text)
		if err != nil {
			panic(err)
		}
		return value
	case 5, 6:
		return decimalAsDouble(Decimal([]float64{0, math.Copysign(0, -1), 1, 2.5, 0.5, -1, 0.1, math.NaN(), math.Inf(1), 1 << 53, 1<<53 + 2, 3}[rng.Intn(12)]))
	case 7:
		return String([]string{"a", "A", "b", "1", "", "001000000000001", "001000000000001AAA"}[rng.Intn(7)])
	case 8:
		value := String([]string{"001000000000001", "001000000000001AAA", "001000000000002"}[rng.Intn(3)])
		value.Type = "Id"
		return value
	default:
		return Int(int64(rng.Intn(40)))
	}
}

// randomEqualityRecord builds Accounts whose fields come from a small pool, so
// equal records, records that differ in one field, records with namespaced or
// case-duplicate fields (unkeyed) and copies sharing a ref all occur.
func randomEqualityRecord(rng *rand.Rand, textDecimals bool, pool []Value) Value {
	if len(pool) > 0 && rng.Intn(5) == 0 && pool[len(pool)-1].Kind == ValueObject {
		record := pool[len(pool)-1]
		copied := record
		copied.Fields = maps.Clone(record.Fields)
		if rng.Intn(2) == 0 {
			copied.Fields["Name"] = String("changed")
		}
		return copied
	}
	record := Object([]string{"Account", "Account", "Contact"}[rng.Intn(3)])
	if rng.Intn(4) != 0 {
		record.Fields["Name"] = String([]string{"a", "b", "A"}[rng.Intn(3)])
	}
	if rng.Intn(3) == 0 {
		record.Fields["NumberOfEmployees"] = randomEqualityScalar(rng, textDecimals)
	}
	if rng.Intn(4) == 0 {
		record.Fields["Id"] = String([]string{"001000000000001", "001000000000001AAA", "001000000000002AAA"}[rng.Intn(3)])
	}
	switch rng.Intn(10) {
	case 0:
		record.Fields["ns__Rating__c"] = String("x")
	case 1:
		record.Fields["name"] = String("dup")
	case 2:
		record.Fields["Phone"] = Null
	}
	return record
}

// The bucketed Set equality and the retainAll/removeAll membership index must
// agree with the element-by-element scans for every scalar family, including
// the cross-kind numeric, signed-zero, NaN, 2^53, Id cases and SObjects.
func TestSetEqualityIndexMatchesScanRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(312))
	machine := New(nil)
	for round := 0; round < 4000; round++ {
		textDecimals := rng.Intn(2) == 0
		records := round%3 == 0
		n := setEqualityIndexMin + rng.Intn(20)
		left, right := make([]Value, n), make([]Value, n)
		for i := range left {
			if records && rng.Intn(4) != 0 {
				left[i] = randomEqualityRecord(rng, textDecimals, left[:i])
				right[i] = randomEqualityRecord(rng, textDecimals, left[:i])
				continue
			}
			left[i], right[i] = randomEqualityScalar(rng, textDecimals), randomEqualityScalar(rng, textDecimals)
		}
		if rng.Intn(2) == 0 {
			copy(right, left)
			rng.Shuffle(len(right), func(i, j int) { right[i], right[j] = right[j], right[i] })
		}
		want := true
		for _, value := range left {
			if !containsValue(right, value) {
				want = false
				break
			}
		}
		if got := setMembersContained(left, right); got != want {
			t.Fatalf("round %d: setMembersContained = %v, scan %v\nleft %v\nright %v", round, got, want, left, right)
		}
		if records {
			continue
		}
		keys := newScalarMembership(right, left)
		for _, needle := range left {
			got, decided, err := keys.contains(machine, needle, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if !decided {
				continue
			}
			scan, err := machine.collectionContainsValue(right, needle, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if got != scan {
				t.Fatalf("round %d: membership of %#v = %v, scan %v in %v", round, needle, got, scan, right)
			}
		}
	}
}

// Fixed pairs at the edges of each key: equal elements must never land in
// different groups, a scalar key shared by two elements must mean they are
// equal, and both Set paths must agree with the scan for each pair.
func TestSetEqualityKeyEdges(t *testing.T) {
	double := func(d float64) Value { return decimalAsDouble(Decimal(d)) }
	long := func(n int64) Value { value := Int(n); value.Type = "Long"; return value }
	account := func(fields map[string]Value) Value {
		value := Object("Account")
		for name, field := range fields {
			value.Fields[name] = field
		}
		return value
	}
	shared := account(map[string]Value{"Name": String("a")})
	stale := shared
	stale.Fields = map[string]Value{"Name": String("b")}
	pairs := [][2]Value{
		{Int(0), double(0)}, {Int(0), double(math.Copysign(0, -1))}, {double(0), double(math.Copysign(0, -1))},
		{double(math.NaN()), double(math.NaN())}, {Int(3), double(3)}, {long(3), double(3)},
		{long(1 << 53), double(1 << 53)}, {long(1<<53 + 1), double(1 << 53)}, {double(0.5), double(0.5)},
		{double(0.1), double(0.1)}, {double(math.Inf(1)), double(math.Inf(1))}, {double(2.5), Int(2)},
		{account(map[string]Value{"Name": String("a")}), account(map[string]Value{"NAME": String("a")})},
		{account(map[string]Value{"Id": String("001000000000001")}), account(map[string]Value{"Id": String("001000000000001AAA")})},
		{account(map[string]Value{"Name": String("a"), "NumberOfEmployees": Int(3)}), account(map[string]Value{"Name": String("a"), "NumberOfEmployees": double(3)})},
		{account(map[string]Value{"Name": String("a"), "Phone": Null}), account(map[string]Value{"Name": String("a")})},
		{shared, stale},
		{account(map[string]Value{"ns__Rating__c": String("x")}), account(map[string]Value{"Rating__c": String("x")})},
		{account(map[string]Value{"Name": String("a"), "name": Null}), account(map[string]Value{"Name": String("a")})},
		{account(map[string]Value{"Name": Null, "name": String("a")}), account(map[string]Value{"Name": String("a")})},
		{account(map[string]Value{"Name": String("a")}), account(map[string]Value{"Name": String("A")})},
	}
	for _, pair := range pairs {
		a, b := pair[0], pair[1]
		equal := a.Equal(b)
		keyer := newSetEqualityKeyer([]Value{a}, []Value{b})
		ka, okA := keyer.groupKey(a)
		kb, okB := keyer.groupKey(b)
		// Objects that share a ref are equal at once; setMembersContained
		// finds those through the ref index, not the group key.
		sameRef := a.Kind == ValueObject && a.Ref != 0 && a.Ref == b.Ref
		if equal && !sameRef && okA && okB && ka != kb {
			t.Errorf("%v and %v are equal but group keys differ: %q %q", a, b, ka, kb)
		}
		sa, sOKA := keyer.scalarKey(a)
		sb, sOKB := keyer.scalarKey(b)
		if sOKA && sOKB && (sa == sb) != equal {
			t.Errorf("%v and %v: scalar keys %q %q but Equal %v", a, b, sa, sb, equal)
		}
		for _, order := range [][2]Value{{a, b}, {b, a}} {
			left, right := []Value{order[0]}, []Value{order[1]}
			if got, want := setMembersContained(left, right), containsValue(right, order[0]); got != want {
				t.Errorf("setMembersContained(%v, %v) = %v, scan %v", order[0], order[1], got, want)
			}
			keep := make([]Value, setEqualityIndexMin)
			needles := make([]Value, setEqualityIndexMin)
			for i := range keep {
				keep[i], needles[i] = order[1], order[0]
			}
			if got, decided, err := newScalarMembership(keep, needles).contains(New(nil), order[0], &Result{}); err != nil {
				t.Fatal(err)
			} else if decided {
				scan, err := New(nil).collectionContainsValue(keep, order[0], &Result{})
				if err != nil {
					t.Fatal(err)
				}
				if got != scan {
					t.Errorf("membership of %v among %v = %v, scan %v", order[0], order[1], got, scan)
				}
			}
		}
	}
}
