package vm

import "testing"

// BenchmarkAliasRollupShape reproduces a rollup-shaped loop: build a
// List<SObject>, group it into Map<String,List<SObject>>, then cast, copy and
// put() fields. Every List.add and Map.put runs alias propagation.
func BenchmarkAliasRollupShape(b *testing.B) {
	program, err := CompileAnonymous(`
List<SObject> all = new List<SObject>();
for (Integer i = 0; i < 400; i++) {
	if (Math.mod(i, 2) == 0) {
		all.add(new Contact(LastName = 'L' + i, FirstName = 'F' + i));
	} else {
		all.add(new Account(Name = 'A' + i));
	}
}
Map<String, List<SObject>> grouped = new Map<String, List<SObject>>();
for (SObject s : all) {
	String key = String.valueOf(s.getSObjectType()) + Math.mod(all.size(), 7);
	if (!grouped.containsKey(key)) {
		grouped.put(key, new List<SObject>());
	}
	grouped.get(key).add(s);
}
Integer total = 0;
for (String key : grouped.keySet()) {
	List<SObject> rows = grouped.get(key);
	List<SObject> copies = new List<SObject>();
	for (SObject s : rows) {
		SObject c = s.clone(false, true, false, false);
		c.put('Description', 'x' + total);
		if (c instanceof Contact) {
			Contact ct = (Contact) c;
			ct.Title = 't';
		}
		copies.add(c);
		total++;
	}
	List<SObject> lit = new List<SObject>{ copies[0], copies[copies.size() - 1] };
	total += lit.size();
}
`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		machine := New(nil)
		org := testDataOrg()
		machine.SetOrg(&org)
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}
