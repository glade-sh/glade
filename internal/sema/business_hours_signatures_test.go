package sema

import "testing"

func TestBusinessHoursDiffAndIsWithinAcceptStringAndIdVariables(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"BusinessHoursSignatures.cls": `
public class BusinessHoursSignatures {
  public void run(String stringId, Id id, Datetime startDate, Datetime endDate) {
    Long diffFromString = BusinessHours.diff(stringId, startDate, endDate);
    Long diffFromId = BusinessHours.diff(id, startDate, endDate);
    Boolean withinFromString = BusinessHours.isWithin(stringId, startDate);
    Boolean withinFromId = BusinessHours.isWithin(id, startDate);
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("BusinessHours String/Id variable calls were rejected: %#v", result.Diagnostics)
	}
}
