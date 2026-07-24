package vm

import (
	"reflect"
	"testing"
)

func TestCloneRuntimeStartsWithFreshAsyncLimitsAndRandomState(t *testing.T) {
	base := New(nil)
	base.localAsyncJobs = []AsyncJob{{
		ID:   "707-dirty-local",
		Kind: "Queueable",
		Args: []Value{String("dirty-local-argument")},
	}}
	base.localAsyncSeq = 41
	base.localAsyncDrain = true
	base.localAsyncChain = true
	base.currentAsyncKind = "Queueable"
	base.currentQueueableDepth = 3
	base.currentQueueableMaxDepth = 9
	base.currentQueueableDelay = 5
	base.testContext = &TestContext{
		Started:        true,
		Stopped:        true,
		AsyncJobs:      []AsyncJob{{ID: "707-dirty-context", Kind: "Future"}},
		HTTPMock:       Object("DirtyHTTPMock"),
		WebServiceMock: Object("DirtyWebServiceMock"),
		ContinuationResponses: map[string]Value{
			"dirty-continuation": String("dirty"),
		},
		ConnectAPIFixtures: map[string]Value{
			"dirty-connect-api": String("dirty"),
		},
		SoqlStubs: map[string]Value{
			"dirty-query": String("dirty"),
		},
	}
	base.limits = Limits{Queries: 7, DMLStatements: 3, AsyncJobs: 2, QueueableJobs: 1}
	base.limitViolations = []LimitViolation{{Name: "queries", Used: 7, Limit: 6}}
	base.queueableDuplicateSignatures["dirty-signature"] = "707-dirty-local"
	base.currentFinalizer = Object("DirtyFinalizer")
	base.SetDeterministicRandomState(0xdeadbeef)

	first := base.CloneRuntime(nil)
	second := base.CloneRuntime(nil)
	assertFreshClonedRequestState(t, "first", first)
	assertFreshClonedRequestState(t, "second", second)

	first.EnableTestContext()
	second.EnableTestContext()
	assertFreshClonedTestContext(t, "first", first.testContext)
	assertFreshClonedTestContext(t, "second", second.testContext)

	first.localAsyncJobs = append(first.localAsyncJobs, AsyncJob{ID: "707-first"})
	first.localAsyncSeq = 1
	first.localAsyncDrain = true
	first.localAsyncChain = true
	first.currentAsyncKind = "Future"
	first.currentQueueableDepth = 1
	first.currentQueueableMaxDepth = 2
	first.currentQueueableDelay = 3
	first.testContext.AsyncJobs = append(first.testContext.AsyncJobs, AsyncJob{ID: "707-first-context"})
	first.testContext.HTTPMock = Object("FirstHTTPMock")
	first.testContext.WebServiceMock = Object("FirstWebServiceMock")
	first.testContext.ContinuationResponses["first"] = String("dirty")
	first.testContext.ConnectAPIFixtures["first"] = String("dirty")
	first.testContext.SoqlStubs["first"] = String("dirty")
	first.limits.Queries = 1
	first.limitViolations = append(first.limitViolations, LimitViolation{Name: "queries", Used: 1, Limit: 0})
	first.queueableDuplicateSignatures["first"] = "707-first"
	first.currentFinalizer = Object("FirstFinalizer")

	if len(second.localAsyncJobs) != 0 || second.localAsyncSeq != 0 || second.localAsyncDrain || second.localAsyncChain {
		t.Fatalf("second clone inherited first local async state: jobs=%#v seq=%d drain=%v chain=%v",
			second.localAsyncJobs, second.localAsyncSeq, second.localAsyncDrain, second.localAsyncChain)
	}
	if second.currentAsyncKind != "" || second.currentQueueableDepth != 0 ||
		second.currentQueueableMaxDepth != 0 || second.currentQueueableDelay != 0 {
		t.Fatalf("second clone inherited first active async state: kind=%q depth=%d maxDepth=%d delay=%d",
			second.currentAsyncKind, second.currentQueueableDepth, second.currentQueueableMaxDepth, second.currentQueueableDelay)
	}
	assertFreshClonedTestContext(t, "second after first mutation", second.testContext)
	if second.limits != (Limits{}) || len(second.limitViolations) != 0 {
		t.Fatalf("second clone inherited first limit state: limits=%#v violations=%#v", second.limits, second.limitViolations)
	}
	if len(second.queueableDuplicateSignatures) != 0 {
		t.Fatalf("second clone inherited first queueable signatures: %#v", second.queueableDuplicateSignatures)
	}
	if !reflect.DeepEqual(second.currentFinalizer, Value{}) {
		t.Fatalf("second clone inherited first finalizer: %#v", second.currentFinalizer)
	}

	oracle := New(nil)
	wantFirstUUID := oracle.nextDeterministicUUID()
	if got := first.nextDeterministicUUID(); got != wantFirstUUID {
		t.Fatalf("first clone initial deterministic UUID = %q, want %q", got, wantFirstUUID)
	}
	if got := second.DeterministicRandomState(); got != 0 {
		t.Fatalf("second clone random state changed when first advanced: %#x", got)
	}
	if got := second.nextDeterministicUUID(); got != wantFirstUUID {
		t.Fatalf("second clone initial deterministic UUID after first advanced = %q, want %q", got, wantFirstUUID)
	}
	if got := base.DeterministicRandomState(); got != 0xdeadbeef {
		t.Fatalf("base random state changed through clone use: %#x", got)
	}
}

func assertFreshClonedRequestState(t *testing.T, label string, machine *VM) {
	t.Helper()
	if len(machine.localAsyncJobs) != 0 || machine.localAsyncSeq != 0 || machine.localAsyncDrain || machine.localAsyncChain {
		t.Fatalf("%s clone retained local async state: jobs=%#v seq=%d drain=%v chain=%v",
			label, machine.localAsyncJobs, machine.localAsyncSeq, machine.localAsyncDrain, machine.localAsyncChain)
	}
	if machine.currentAsyncKind != "" || machine.currentQueueableDepth != 0 ||
		machine.currentQueueableMaxDepth != 0 || machine.currentQueueableDelay != 0 {
		t.Fatalf("%s clone retained active async state: kind=%q depth=%d maxDepth=%d delay=%d",
			label, machine.currentAsyncKind, machine.currentQueueableDepth, machine.currentQueueableMaxDepth, machine.currentQueueableDelay)
	}
	if machine.testContext != nil {
		t.Fatalf("%s clone retained test context: %#v", label, machine.testContext)
	}
	if machine.limits != (Limits{}) || len(machine.limitViolations) != 0 {
		t.Fatalf("%s clone retained limit state: limits=%#v violations=%#v", label, machine.limits, machine.limitViolations)
	}
	if machine.queueableDuplicateSignatures == nil || len(machine.queueableDuplicateSignatures) != 0 {
		t.Fatalf("%s clone queueable signatures are not fresh: %#v", label, machine.queueableDuplicateSignatures)
	}
	if !reflect.DeepEqual(machine.currentFinalizer, Value{}) {
		t.Fatalf("%s clone retained finalizer: %#v", label, machine.currentFinalizer)
	}
	if got := machine.DeterministicRandomState(); got != 0 {
		t.Fatalf("%s clone retained deterministic random state: %#x", label, got)
	}
}

func assertFreshClonedTestContext(t *testing.T, label string, context *TestContext) {
	t.Helper()
	if context == nil {
		t.Fatalf("%s test context is nil", label)
	}
	if context.Started || context.Stopped || len(context.AsyncJobs) != 0 {
		t.Fatalf("%s test context retained lifecycle or async state: %#v", label, context)
	}
	if !reflect.DeepEqual(context.HTTPMock, Value{}) || !reflect.DeepEqual(context.WebServiceMock, Value{}) {
		t.Fatalf("%s test context retained mocks: http=%#v webService=%#v", label, context.HTTPMock, context.WebServiceMock)
	}
	if context.ContinuationResponses == nil || len(context.ContinuationResponses) != 0 ||
		context.ConnectAPIFixtures == nil || len(context.ConnectAPIFixtures) != 0 ||
		context.SoqlStubs == nil || len(context.SoqlStubs) != 0 {
		t.Fatalf("%s test context mock registries are not fresh: continuation=%#v connect=%#v soql=%#v",
			label, context.ContinuationResponses, context.ConnectAPIFixtures, context.SoqlStubs)
	}
}
