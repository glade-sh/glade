package vm

import (
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
db.remove(0.25); db.add(0.250);
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
for (Integer i = 0; i < 18; i++) { na.add(Decimal.valueOf(i) + 0.50); nb.add(Decimal.valueOf(i) + 0.5); }
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
List<Set<Integer>> nested1 = new List<Set<Integer>>{ a };
List<Set<Integer>> nested2 = new List<Set<Integer>>{ a.clone() };
System.debug(nested1 == nested2);
`

// The rows were produced by train D2 (1318923037) before the bucketed
// equality: the comparisons must be identical.
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
true false false 20
true true
false false true 19
true {3, 5, 29} true {3, 29}
false false false 22 21
false 22 true 18
true true
true {1, 2}
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

func randomEqualityScalar(rng *rand.Rand) Value {
	switch rng.Intn(9) {
	case 0:
		return Null
	case 1:
		return Bool(rng.Intn(2) == 0)
	case 2:
		return Int(int64(rng.Intn(6) - 2))
	case 3:
		value := Int(int64(rng.Intn(6) - 2))
		value.Type = "Long"
		return value
	case 4:
		text := []string{"0", "0.0", "1", "1.0", "1.00", "-1", "2.5", "2.50", "-0.0", "0.5"}[rng.Intn(10)]
		value, err := decimalFromText(text)
		if err != nil {
			panic(err)
		}
		return value
	case 5:
		return decimalAsDouble(Decimal([]float64{0, math.Copysign(0, -1), 1, 2.5, 0.5, -1}[rng.Intn(6)]))
	case 6:
		return String([]string{"a", "A", "b", "1", "001000000000001", "001000000000001AAA"}[rng.Intn(6)])
	case 7:
		value := String([]string{"001000000000001", "001000000000001AAA", "001000000000002"}[rng.Intn(3)])
		value.Type = "Id"
		return value
	default:
		return Int(int64(rng.Intn(40)))
	}
}

// The bucketed Set equality and the retainAll/removeAll membership index must
// agree with the element-by-element scans for every scalar family, including
// the cross-kind numeric, signed-zero and Id cases.
func TestSetEqualityIndexMatchesScanRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(312))
	machine := New(nil)
	for round := 0; round < 3000; round++ {
		n := setEqualityIndexMin + rng.Intn(20)
		left, right := make([]Value, n), make([]Value, n)
		for i := range left {
			left[i], right[i] = randomEqualityScalar(rng), randomEqualityScalar(rng)
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
		keys := newScalarMembership(right)
		for _, needle := range left {
			got, decided := keys.contains(needle)
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
