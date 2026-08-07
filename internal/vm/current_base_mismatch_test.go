package vm

import (
	"strings"
	"testing"
)

func TestCurrentBaseAbsentAsyncOptionsGetterIsRejected(t *testing.T) {
	program, err := CompileAnonymous("AsyncOptions opts = new AsyncOptions(); opts.getMaximumQueueableStackDepth();")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil || !strings.Contains(err.Error(), `unsupported call "AsyncOptions.getMaximumQueueableStackDepth"`) {
		t.Fatalf("err = %v, want absent Salesforce API rejection", err)
	}
}

func TestCurrentBaseLimitsChildRelationshipsReturnsZero(t *testing.T) {
	program, err := CompileAnonymous("Limits.getChildRelationshipsDescribes();")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Execute(program, nil); err != nil {
		t.Fatalf("err = %v, want supported local getter", err)
	}
}

func TestCurrentBaseEventBusRejectsNonPlatformEvent(t *testing.T) {
	program, err := CompileAnonymous("EventBus.publish(new Account(Name = 'not an event'));")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil || !strings.Contains(err.Error(), "platform event") {
		t.Fatalf("err = %v, want platform-event-only rejection", err)
	}
}

func TestCurrentBaseListDeepCloneRejectsNonSObjectList(t *testing.T) {
	program, err := CompileAnonymous("List<Integer> values = new List<Integer>{1}; values.deepClone();")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil || !strings.Contains(err.Error(), "Operation only applies to SObject list types: List<Integer>") {
		t.Fatalf("err = %v, want non-SObject deepClone rejection", err)
	}
}
