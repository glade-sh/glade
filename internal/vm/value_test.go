package vm

import "testing"

func TestValueStringGenericObjectIncludesApexTypeDelimiter(t *testing.T) {
	value := Object("TriggerHandler")

	if got := value.String(); got != "TriggerHandler:{}" {
		t.Fatalf("String() = %q", got)
	}
}
