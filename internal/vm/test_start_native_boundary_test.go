package vm

import (
	"errors"
	"testing"
)

// The anonymous finalizer_deferred_work rows call startTest outside an Apex test.
func TestStartTestOutsideTestingNativeAnonymousRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions("Test.startTest();", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			var failure *RuntimeError
			if !errors.As(err, &failure) || failure.Type != "System.FinalException" || failure.Message != "Method only allowed during testing" {
				t.Fatalf("native System.FinalException: Method only allowed during testing; Glade=%v", err)
			}
		})
	}
}
