package sema

import (
	"strings"
	"testing"
)

func TestComplementRejectsNonIntegralOperands(t *testing.T) {
	for _, operand := range []string{"true", "'text'", "1.5"} {
		t.Run(operand, func(t *testing.T) {
			r := analyzeDeclarationProject(t, map[string]string{"Probe.cls": "public class Probe { public Object run() { return ~(" + operand + "); } }"})
			found := false
			for _, d := range r.Diagnostics {
				if strings.Contains(d.Message, "operator ~ requires an Integer or Long operand") {
					found = true
				}
			}
			if !found {
				t.Fatalf("nonintegral complement accepted: %#v", r.Diagnostics)
			}
		})
	}
}
