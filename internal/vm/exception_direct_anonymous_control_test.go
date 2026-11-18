package vm

import "testing"

// NATIVE direct anonymous divide controls at source API 62.0 and 67.0:
// the caught fault is at AnonymousBlock line 4 with no extra callers.
func TestDirectAnonymousExceptionFrameControl(t *testing.T) {
	source := `String observation;
try {
    Integer zero = 0;
    Integer ignored = 5 / zero;
} catch (Exception e) {
    observation = e.getStackTraceString();
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if _, err := machine.Execute(p); err != nil {
				t.Fatal(err)
			}
			if got, want := machine.Globals["observation"].Text, "AnonymousBlock: line 4, column 1"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
