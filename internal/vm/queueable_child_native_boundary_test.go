package vm

import (
	"errors"
	"testing"
)

// finalizer_deferred_work pins the uncapped/capped body and finalizer origins at stopTest.
func TestQueueableChildNativeTestStopBodyAndFinalizerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct {
			name            string
			depth, maxDepth int
			rejected        bool
		}{
			{"bodyUncapped", 1, 0, true},
			{"bodyDepthTwo", 1, 2, false},
			{"bodyDepthOne", 1, 1, true},
			{"finalizerUncapped", 0, 0, false},
		} {
			for _, overload := range []string{"plain", "delay", "options"} {
				if route.maxDepth > 0 && overload != "plain" {
					continue
				}
				t.Run(api+"/"+route.name+"/"+overload, func(t *testing.T) {
					machine := New(nil)
					machine.currentMethod.APIVersion = api
					machine.EnableTestContext()
					machine.testContext.Started = true
					machine.testContext.Stopped = true
					machine.testContext.Draining = true
					machine.currentAsyncKind = "Queueable"
					machine.currentQueueableDepth = route.depth
					machine.currentQueueableMaxDepth = route.maxDepth
					args := []Value{Object("ChildWorker")}
					if overload == "delay" {
						args = append(args, Int(0))
					}
					if overload == "options" {
						args = append(args, Object("AsyncOptions"))
					}
					_, err := machine.enqueueJob(args, &Result{})
					if !route.rejected {
						if err != nil {
							t.Fatal(err)
						}
						if len(machine.testContext.AsyncJobs) != 1 || machine.testContext.AsyncJobs[0].Deferred {
							t.Fatal("native drainable child not queued for execution")
						}
						return
					}
					var failure *apexThrowError
					if !errors.As(err, &failure) || failure.value.Type != "System.AsyncException" || failure.value.Fields["message"].Text != "Maximum stack depth has been reached." {
						t.Fatalf("native System.AsyncException: Maximum stack depth has been reached.; Glade=%v", err)
					}
					if len(machine.testContext.AsyncJobs) != 0 {
						t.Fatal("rejected child was queued")
					}
				})
			}
		}
	}
}

// Preserve the existing first-explicit-cap path; its native result is outside this capture.
func TestQueueableChildExplicitDepthRetainsExistingAdmission(t *testing.T) {
	machine := New(nil)
	machine.EnableTestContext()
	machine.testContext.Started = true
	machine.testContext.Stopped = true
	machine.testContext.Draining = true
	machine.currentAsyncKind = "Queueable"
	machine.currentQueueableDepth = 1
	options := Object("AsyncOptions")
	options.Fields["MaximumQueueableStackDepth"] = Int(2)
	_, err := machine.enqueueJob([]Value{Object("ChildWorker"), options}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if len(machine.testContext.AsyncJobs) != 1 || machine.testContext.AsyncJobs[0].Deferred || machine.testContext.AsyncJobs[0].QueueableMaxDepth != 2 {
		t.Fatal("explicit child cap admission changed")
	}
}
