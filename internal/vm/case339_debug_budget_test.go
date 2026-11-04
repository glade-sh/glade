package vm

import (
	"fmt"
	"testing"
)

func TestCase339DebugBudgetCountsMessageOverhead(t *testing.T) {
	for _, message := range []string{"", "x", "abc"} {
		t.Run(fmt.Sprintf("bytes-%d", len(message)), func(t *testing.T) {
			program, err := CompileAnonymous(fmt.Sprintf("for (Integer i = 0; i < 200; i++) { System.debug('%s'); }", message))
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetDebugOutputLimit(64)
			result, err := machine.Execute(program)
			if err == nil {
				t.Fatalf("%d messages ran without hitting output limit", len(result.Debug))
			}
			if len(result.Debug) > 64/(len(message)+3) {
				t.Fatalf("collected %d messages beyond budget", len(result.Debug))
			}
		})
	}
}
