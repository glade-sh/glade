package vm

import "testing"

// Preserve the local inherited-depth error for a test context that has not
// stopped. That route is a compatibility control with no native row. Standalone
// (anonymous) dispatch is covered by the async_stack_depth native rows, which
// observe a catchable System.AsyncException.
func TestQueueableInheritedDepthOutsideStopTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"standalone", "testContext"} {
			for _, overload := range []string{"plain", "delay", "options"} {
				t.Run(api+"/"+route+"/"+overload, func(t *testing.T) {
					machine := New(nil)
					machine.currentMethod.APIVersion = api
					if route == "testContext" {
						machine.EnableTestContext()
						machine.testContext.Draining = true
					} else {
						machine.localAsyncDrain = true
					}
					machine.currentAsyncKind = "Queueable"
					machine.currentQueueableDepth = 1
					machine.currentQueueableMaxDepth = 1
					args := []Value{Object("ChildWorker")}
					if overload == "delay" {
						args = append(args, Int(0))
					} else if overload == "options" {
						args = append(args, Object("AsyncOptions"))
					}
					_, err := machine.enqueueJob(args, &Result{})
					want := "MaximumQueueableStackDepth exceeded"
					if route == "standalone" {
						want = "Maximum stack depth has been reached."
					}
					if err == nil || err.Error() != want {
						t.Fatalf("outside stopTest inherited-depth error = %v; want existing local error", err)
					}
					if len(machine.localAsyncJobs) != 0 || (machine.testContext != nil && len(machine.testContext.AsyncJobs) != 0) {
						t.Fatal("rejected child was queued")
					}
				})
			}
		}
	}
}
