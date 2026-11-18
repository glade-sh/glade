package vm

import "testing"

// BenchmarkDatetimeLoop is a Datetime-heavy loop: construction, arithmetic,
// comparison and hashing, 2000 iterations per run.
func BenchmarkDatetimeLoop(b *testing.B) {
	program, err := CompileAnonymous(`
Datetime d = Datetime.newInstance(0L);
Integer h = 0;
for (Integer i = 0; i < 2000; i++) {
	Datetime e = d.addDays(i).addHours(i);
	if (e > d) { h += e.hashCode(); }
	Datetime f = Datetime.newInstance(Long.valueOf(i));
	h += f.getTime() > 0 ? 1 : 0;
}
`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		machine := New(nil)
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}
