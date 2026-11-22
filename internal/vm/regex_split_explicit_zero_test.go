package vm

import "testing"

// The explicit-zero overload has no native row; retain origin/main behavior.
func TestRegexSplitExplicitZeroOriginMain(t *testing.T) {
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions("Integer result = '😀'.split('', 0).size();", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			if got := machine.Globals["result"].Int; got != 1 {
				t.Fatalf("explicit-zero split size = %d, want 1", got)
			}
		})
	}
}
