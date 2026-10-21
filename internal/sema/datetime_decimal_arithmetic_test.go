package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestDatetimeArithmeticAllowsFractionalDecimalDays(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
	}{
		{"literal addition", "base + 0.5"},
		{"literal subtraction", "base - 0.5"},
		{"decimal addition", "base + delta"},
		{"decimal subtraction", "base - delta"},
	} {
		for _, argument := range []bool{false, true} {
			name := tc.name + "/assignment"
			statement := "Datetime observed=" + tc.expression + ";"
			if argument {
				name = tc.name + "/method argument"
				statement = "Id recordId; Test.setCreatedDate(recordId," + tc.expression + ");"
			}
			t.Run(name, func(t *testing.T) {
				result := analyzeDeclarationProject(t, map[string]string{
					"DayArithmeticObservation.cls": `public class DayArithmeticObservation {
  public static void observe() {
    Datetime base=Datetime.newInstanceGmt(2024,1,2,12,0,0);
    Decimal delta=0.5;
    ` + statement + `
  }
}`,
				})
				if result.HasErrors() {
					t.Fatalf("observed Datetime fractional day arithmetic rejected: %v", result.Diagnostics)
				}
			})
		}
	}
}

func TestDatetimeDecimalArithmeticPreservesExistingDayOperands(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"date integer addition", "Date observed=Date.newInstance(2024,1,2)+1;"},
		{"date integer subtraction", "Date observed=Date.newInstance(2024,1,2)-1;"},
		{"datetime integer addition", "Datetime observed=base+1;"},
		{"datetime integer subtraction", "Datetime observed=base-1;"},
		{"datetime double addition", "Double delta=0.5; Datetime observed=base+delta;"},
		{"datetime double subtraction", "Double delta=0.5; Datetime observed=base-delta;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"DayArithmeticObservation.cls": `public class DayArithmeticObservation {
  public static void observe() {
    Datetime base=Datetime.newInstanceGmt(2024,1,2,12,0,0);
    ` + tc.body + `
  }
}`,
			})
			if result.HasErrors() {
				t.Fatalf("existing day operand rejected: %v", result.Diagnostics)
			}
		})
	}
}

// These local controls retain the existing boundary for unobserved operands.
func TestDatetimeDecimalArithmeticKeepsOtherOperandsRejected(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"date decimal", "Date base=Date.newInstance(2024,1,2); Decimal delta=0.5; Object observed=base-delta;"},
		{"date double", "Date base=Date.newInstance(2024,1,2); Double delta=0.5; Object observed=base-delta;"},
		{"datetime long", "Datetime base=Datetime.newInstanceGmt(2024,1,2,12,0,0); Long delta=1L; Object observed=base-delta;"},
		{"datetime multiplication", "Datetime base=Datetime.newInstanceGmt(2024,1,2,12,0,0); Decimal delta=0.5; Object observed=base*delta;"},
		{"reversed decimal subtraction", "Datetime base=Datetime.newInstanceGmt(2024,1,2,12,0,0); Decimal delta=0.5; Object observed=delta-base;"},
		{"reversed literal subtraction", "Datetime base=Datetime.newInstanceGmt(2024,1,2,12,0,0); Object observed=0.5-base;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"DayArithmeticObservation.cls": "public class DayArithmeticObservation { public static void observe() { " + tc.body + " } }",
			})
			var errors []diagnostic.Diagnostic
			for _, item := range result.Diagnostics {
				if item.Severity == diagnostic.Error {
					errors = append(errors, item)
				}
			}
			if len(errors) != 1 || errors[0].Code != "GLADESEMA019" || errors[0].Message != `method "observe" has invalid expression: arithmetic operator requires numeric operands` {
				t.Fatalf("changed arithmetic operand rejection: %v", result.Diagnostics)
			}
		})
	}
}
