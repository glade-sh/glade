package vm

import (
	"reflect"
	"testing"
)

// Each program builds more than eight records so the lists and maps are cache
// eligible, mutates several lists in alternation, and prints what it sees. The
// output must be identical with the miss carry on and off.
func TestAliasMissCarryMatchesUncachedResults(t *testing.T) {
	programs := map[string]string{
		"nested list into map": `
Map<String, List<Account>> g = new Map<String, List<Account>>();
List<Account> all = new List<Account>();
for (Integer i = 0; i < 20; i++) { all.add(new Account(Name = 'A' + i)); }
for (Account a : all) {
	String k = 'k' + Math.mod(a.Name.length(), 2);
	if (!g.containsKey(k)) { g.put(k, new List<Account>()); }
	g.get(k).add(a);
}
List<Account> extra = new List<Account>();
for (Integer i = 0; i < 12; i++) { extra.add(new Account(Name = 'X' + i)); g.get('k1').add(extra[i]); }
System.debug(g.get('k0').size() + ',' + g.get('k1').size() + ',' + extra.size() + ',' + all.size());`,
		"alias mutation": `
List<Account> a = new List<Account>();
for (Integer i = 0; i < 12; i++) { a.add(new Account(Name = 'A' + i)); }
List<Account> b = a;
List<Account> c = new List<Account>();
List<List<Account>> holder = new List<List<Account>>{ a };
for (Integer i = 0; i < 12; i++) { c.add(new Account(Name = 'C' + i)); b.add(new Account(Name = 'B' + i)); }
holder.add(c);
c.add(new Account(Name = 'last'));
System.debug(a.size() + ',' + b.size() + ',' + c.size() + ',' + holder[0].size() + ',' + holder[1].size());`,
		"addAll": `
List<Contact> x = new List<Contact>();
List<Contact> y = new List<Contact>();
for (Integer i = 0; i < 12; i++) { x.add(new Contact(LastName = 'x' + i)); }
Map<String, List<Contact>> m = new Map<String, List<Contact>>{ 'x' => x };
for (Integer i = 0; i < 12; i++) { y.add(new Contact(LastName = 'y' + i)); }
y.addAll(x);
m.put('y', y);
x.add(new Contact(LastName = 'tail'));
y.add(new Contact(LastName = 'tail2'));
System.debug(m.get('x').size() + ',' + m.get('y').size() + ',' + x.size() + ',' + y.size());`,
		"clone": `
List<Account> a = new List<Account>();
for (Integer i = 0; i < 12; i++) { a.add(new Account(Name = 'A' + i)); }
List<Account> copy = a.clone();
List<List<Account>> wrap = new List<List<Account>>{ a };
for (Integer i = 0; i < 10; i++) { copy.add(new Account(Name = 'C' + i)); a.add(new Account(Name = 'D' + i)); }
wrap.add(copy);
copy.add(new Account(Name = 'z'));
System.debug(a.size() + ',' + copy.size() + ',' + wrap[0].size() + ',' + wrap[1].size());`,
	}
	for name, src := range programs {
		t.Run(name, func(t *testing.T) {
			program, err := CompileAnonymous(src)
			if err != nil {
				t.Fatal(err)
			}
			run := func(disable bool) []string {
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				machine.disableAliasMissCarry = disable
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatalf("disable=%v: %v", disable, err)
				}
				return result.Debug
			}
			off, on := run(true), run(false)
			if len(on) == 0 {
				t.Fatal("program produced no debug output")
			}
			if !reflect.DeepEqual(off, on) {
				t.Fatalf("carry on %v differs from carry off %v", on, off)
			}
		})
	}
}
