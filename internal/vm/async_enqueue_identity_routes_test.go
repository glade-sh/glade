package vm

import "testing"

func TestAsyncEnqueueOverloadReturnAndContextIdentityRoutesNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"anonymous", "isTest"} {
			for _, route := range []struct{ name, argument string }{{"default", ""}, {"Integer", ", 0"}, {"AsyncOptions", ", new AsyncOptions()"}} {
				t.Run(api+"/"+mode+"/"+route.name, func(t *testing.T) {
					machine := New(nil)
					if mode == "isTest" {
						machine.EnableTestContext()
					}
					callback, err := CompileAnonymousWithOptions(`String assigned=context.getJobId(); System.assertEquals(18,assigned.length()); System.assertEquals(18,(''+context.getJobId()).length()); System.debug(assigned);`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "OverloadWorker", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": {Name: "OverloadWorker.execute", ClassName: "OverloadWorker", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: callback}}}); err != nil {
						t.Fatal(err)
					}
					program, err := CompileAnonymousWithOptions(`String returned=System.enqueueJob(new OverloadWorker()`+route.argument+`); System.assertEquals(18,returned.length()); System.debug(returned);`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 2 || result.Debug[0] != result.Debug[1] {
						t.Fatalf("enqueue return/context IDs differ: %v", result.Debug)
					}
				})
			}
		}
	}
}
