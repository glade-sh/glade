package vm_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

const strictHeapProbeSeed = `List<Account> accs = new List<Account>();
for (Integer i = 0; i < 30; i++) { accs.add(new Account(Name = 'a' + i)); }
Integer hits = 0;
`

var strictHeapReviewProbes = map[string]string{
	"loopvar-reassigned-then-rebound": strictHeapProbeSeed + `for (Account a : accs) { hits++; a = new Account(Name = 'replacement-with-padding-' + hits); hits++; }
System.debug('end ' + Limits.getHeapSize());`,
	"loopvar-reassigned-first-stmt": strictHeapProbeSeed + `for (Account a : accs) { a = new Account(Name = 'replacement-with-padding-' + hits); hits++; }
System.debug('end ' + Limits.getHeapSize());`,
	"index-set-current-element": strictHeapProbeSeed + `Integer j = 0;
for (Account a : accs) { accs[j] = new Account(Name = 'z' + hits); hits++; j++; }
System.debug('end ' + Limits.getHeapSize());`,
	"list-set-current-element": strictHeapProbeSeed + `Integer j = 0;
for (Account a : accs) { accs.set(j, new Account(Name = 'z' + hits)); hits++; j++; }
System.debug('end ' + Limits.getHeapSize());`,
	"fresh-id-shared-then-rebind": `List<Id> ids = new List<Id>();
Integer k = 0;
Account x = new Account(Name = 'n', Id = '001000000000001AAA');
ids.add(x.Id);
x = new Account(Name = 'm');
k++;
System.debug('end ' + Limits.getHeapSize());`,
	"fresh-id-shared-then-block-exit": `List<Id> ids = new List<Id>();
Integer k = 0;
for (Integer i = 0; i < 5; i++) { Contact c = new Contact(LastName = 'n', AccountId = '001000000000001AAA'); ids.add(c.AccountId); k++; }
k++;
System.debug('end ' + Limits.getHeapSize());`,
	"fresh-id-shared-then-field-overwrite": `List<Id> ids = new List<Id>();
Integer k = 0;
Contact c = new Contact(LastName = 'n', AccountId = '001000000000001AAA');
ids.add(c.AccountId);
c.AccountId = null;
k++;
System.debug('end ' + Limits.getHeapSize());`,
	"fresh-id-field-write-shared-rebind": `List<Id> ids = new List<Id>();
Integer k = 0;
Contact c = new Contact(LastName = 'n');
c.AccountId = '001000000000001AAA';
ids.add(c.AccountId);
c = new Contact(LastName = 'q');
k++;
System.debug('end ' + Limits.getHeapSize());`,
	"fresh-id-into-map-then-drop": `Map<String, Id> m = new Map<String, Id>();
Integer k = 0;
for (Integer i = 0; i < 5; i++) { Contact c = new Contact(LastName = 'n', AccountId = '001000000000001AAA'); m.put('k' + i, c.AccountId); k++; }
k++;
System.debug('end ' + Limits.getHeapSize());`,
	"fresh-id-into-new-record-then-drop": `List<Contact> rows = new List<Contact>();
Integer k = 0;
for (Integer i = 0; i < 5; i++) { Contact c = new Contact(LastName = 'n', AccountId = '001000000000001AAA'); rows.add(new Contact(LastName = 'r', AccountId = c.AccountId)); k++; }
k++;
System.debug('end ' + Limits.getHeapSize());`,
	"map-id-15-vs-18": `Map<Id, Integer> m = new Map<Id, Integer>();
Integer k = 0;
m.put('001000000000001', 1);
m.put('001000000000001AAA', 2);
m.put('001000000000002AAA', 3);
k++;
System.debug('end ' + Limits.getHeapSize() + ' ' + m.size());`,
	"map-put-aliased": `Map<String, Integer> m = new Map<String, Integer>();
Map<String, Integer> alias = m;
List<Map<String, Integer>> holder = new List<Map<String, Integer>>{ m };
Integer k = 0;
for (Integer i = 0; i < 20; i++) { m.put('key' + i, i); alias.put('al' + i, i); k++; }
m.put('key1', 99);
k++;
System.debug('end ' + Limits.getHeapSize() + ' ' + holder[0].size());`,
	"nested-same-list-break": strictHeapProbeSeed + `for (Account a : accs) { for (Account b : accs) { hits++; if (hits > 5) { break; } } hits++; }
System.debug('end ' + Limits.getHeapSize());`,
	"loopvar-captured-then-modified": strictHeapProbeSeed + `List<Account> other = new List<Account>();
for (Account a : accs) { other.add(a); a.Description = 'desc-' + hits; hits++; }
accs = null;
hits++;
System.debug('end ' + Limits.getHeapSize());`,
	"loopvar-exception-exit": strictHeapProbeSeed + `try { for (Account a : accs) { hits++; if (hits == 3) { Integer z = hits / 0; } } } catch (Exception e) { hits++; }
hits++;
System.debug('end ' + Limits.getHeapSize());`,
	"iterable-reassigned-in-loop": strictHeapProbeSeed + `List<Account> other = new List<Account>{ new Account(Name = 'o') };
for (Account a : accs) { hits++; accs = other; hits++; }
hits++;
System.debug('end ' + Limits.getHeapSize());`,
	"loopvar-continue-return-style": strictHeapProbeSeed + `for (Account a : accs) { hits++; if (hits > 10) { continue; } hits++; }
for (Account a : accs) { hits++; if (hits > 50) { break; } }
hits++;
System.debug('end ' + Limits.getHeapSize());`,
	"loopvar-set-iterable": `Set<Account> s = new Set<Account>();
for (Integer i = 0; i < 10; i++) { s.add(new Account(Name = 's' + i)); }
Integer hits = 0;
List<Account> keep = new List<Account>();
for (Account a : s) { hits++; keep.add(a); a = new Account(Name = 'pad-pad-pad-' + hits); hits++; }
System.debug('end ' + Limits.getHeapSize());`,
}

func TestStrictHeapReviewProbesMatchFullWalk(t *testing.T) {
	fixture := newStrictShapeFixture(t)
	sync, _ := vm.LimitCapsForProfile("default")
	for name, source := range strictHeapReviewProbes {
		t.Run(name, func(t *testing.T) {
			run := func(c vm.LimitCaps, off bool) (out strictShapeOutcome, priced, walked int, panicked string) {
				defer func() {
					if r := recover(); r != nil {
						panicked = fmt.Sprint(r)
					}
				}()
				out, priced, walked = fixture.run(t, source, c, off)
				return
			}
			base, _, _, p := run(sync, true)
			if p != "" {
				t.Fatalf("base panic: %s", p)
			}
			t.Logf("base err=%q heap=%d debug=%v", base.err, base.limits.HeapSize, base.debug)
			end := base.limits.HeapSize
			for _, cap := range []int{0, end / 2, end * 3 / 4, end - 1} {
				c := sync
				if cap != 0 {
					c.HeapSize = cap
				}
				want, p1 := base, ""
				if cap != 0 {
					want, _, _, p1 = run(c, true)
				}
				got, priced, walked, p2 := run(c, false)
				if p1 != "" || p2 != "" {
					t.Errorf("cap %d: panic off=%q on=%q (priced %d walked %d)", cap, p1, p2, priced, walked)
					continue
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("cap %d: differs\n got  err=%q heap=%d debug=%v\n want err=%q heap=%d debug=%v", cap, got.err, got.limits.HeapSize, got.debug, want.err, want.limits.HeapSize, want.debug)
				}
				if cap == 0 {
					t.Logf("priced %d walked %d", priced, walked)
				}
			}
		})
	}
}

func TestStrictHeapLoopVarReboundNoFalseLimit(t *testing.T) {
	sync, _ := vm.LimitCapsForProfile("default")
	source := strictHeapReviewProbes["loopvar-reassigned-then-rebound"]
	for _, cap := range []int{0, 6490, 6500, 6600, 6700, 6800, 7000, 8000, 12000} {
		c := sync
		c.HeapSize = cap
		want, _, _ := runStrictShape(t, source, c, true)
		got, _, _ := runStrictShape(t, source, c, false)
		t.Logf("cap %d: got err=%q want err=%q", cap, got.err, want.err)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("cap %d: got err=%q heap=%d debug=%v; want err=%q heap=%d debug=%v", cap, got.err, got.limits.HeapSize, got.debug, want.err, want.limits.HeapSize, want.debug)
		}
	}
}
