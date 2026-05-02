package vm

import "testing"

func BenchmarkExecTriggerDML(b *testing.B) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	a.Name = a.Name + '!';
}
`)
	if err != nil {
		b.Fatal(err)
	}
	program, err := CompileAnonymous(`
for (Integer i = 0; i < 25; i++) {
	insert new Account(Name = 'Acme');
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
		if err := machine.RegisterTrigger(Trigger{
			Name:      "AccountBeforeInsert",
			Object:    "Account",
			Timing:    triggerTimingBefore,
			Operation: "insert",
			Program:   triggerProgram,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}
