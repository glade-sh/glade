package vm

import (
	"reflect"
	"testing"
)

func TestMethodReturnAliasMutationCollectorKeepsFirstTargetInlineAndOrdered(t *testing.T) {
	firstValue := Object("First")
	secondValue := Object("Second")
	thirdValue := Object("Third")
	first := methodReturnAliasMutation{previous: snapshotAlias(firstValue), original: firstValue, updated: firstValue}
	second := methodReturnAliasMutation{previous: snapshotAlias(secondValue), original: secondValue, updated: secondValue}
	third := methodReturnAliasMutation{previous: snapshotAlias(thirdValue), original: thirdValue, updated: thirdValue}

	collector := methodReturnAliasMutationCollector{}
	collector.append(first, 4)
	if collector.many != nil {
		t.Fatalf("first target allocated batch storage: %#v", collector.many)
	}
	if got, ok := collector.single(); !ok || !reflect.DeepEqual(got, first) {
		t.Fatalf("single target = (%#v, %v), want first target", got, ok)
	}

	collector.append(second, 4)
	collector.append(third, 4)
	got := collector.batch()
	if !reflect.DeepEqual(got, []methodReturnAliasMutation{first, second, third}) {
		t.Fatalf("batch order = %#v, want first, second, third", got)
	}
}
