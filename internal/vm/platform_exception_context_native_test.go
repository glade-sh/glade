package vm

import (
	"errors"
	"strings"
	"testing"
)

// Native controls: system_exception_hierarchy/native/winter27_context/org.tsv.
func TestPlatformExceptionConstructionContextNative(t *testing.T) {
	cases := []struct {
		name, expression string
		restricted       bool
	}{
		{"AuraHandledException", "new System.AuraHandledException('probe')", true},
		{"InvalidParameterValueException", "new System.InvalidParameterValueException('value','probe')", true},
		{"NoAccessException", "new System.NoAccessException()", true},
		{"NoDataFoundException", "new System.NoDataFoundException()", true},
		{"TouchHandledException", "new System.TouchHandledException('probe')", true},
		{"TypeException", "new System.TypeException()", false},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, tc := range cases {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					machine := New(nil)
					if route == "isTest" {
						machine.EnableTestContext()
					}
					program, err := CompileAnonymousWithOptions("try { System.Exception e="+tc.expression+"; } catch(Exception e) { System.assert(false,'caught'); }", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					if tc.restricted && route == "anonymous" {
						var fault *RuntimeError
						if !errors.As(err, &fault) || fault.Type != "System.LimitException" || !strings.Contains(fault.Message, "Can only throw this exception type from VisualForce or Aura context") {
							t.Fatalf("native uncaught limit exception; local %v", err)
						}
					} else if err != nil {
						t.Fatalf("native constructor succeeded; local %v", err)
					}
				})
			}
		}
	}
}
