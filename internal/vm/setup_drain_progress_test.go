package vm

import "testing"

// A @TestSetup drain stops after a pass that runs no job: an aborted schedule
// or a nested Schedulable left queued must not spin the pass loop.
func TestSetupDrainStopsWithoutProgress(t *testing.T) {
	for name, source := range map[string]string{
		"aborted": `System.abortJob(System.schedule('spin-abort', '0 0 0 1 1 ? 2099', new SpinSched()));`,
		"nested":  `System.schedule('spin-nested', '0 0 0 1 1 ? 2099', new SpinSched.Nested());`,
	} {
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			body, err := CompileAnonymous(`System.debug('ran');`)
			if err != nil {
				t.Fatal(err)
			}
			for _, class := range []string{"SpinSched", "SpinSched.Nested"} {
				if err := machine.RegisterClass(Class{Name: class, Interfaces: []string{"Schedulable"}, Methods: map[string]Method{"execute": {Name: class + ".execute", ClassName: class, ReturnType: "void", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, Program: body}}}); err != nil {
					t.Fatal(err)
				}
			}
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.DrainTestSetupAsync(); err != nil {
				t.Fatal(err)
			}
			if machine.setupDrainPasses > 2 {
				t.Fatalf("setup drain ran %d passes", machine.setupDrainPasses)
			}
			if name == "aborted" && len(result.Debug) != 0 {
				t.Fatalf("aborted schedule ran: %v", result.Debug)
			}
		})
	}
}
