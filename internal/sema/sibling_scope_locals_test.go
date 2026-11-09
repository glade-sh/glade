package sema

import (
	"strings"
	"testing"
)

func TestLocalsDeclaredLaterInSiblingScopesDoNotLeakBackwards(t *testing.T) {
	files := map[string]string{
		"ScopeLeak.cls": `
public class ScopeLeak {
  public static Integer loops(List<String> items) {
    Integer total = 0;
    for (Integer i = 0; i < items.size(); i++) { total += i; }
    Integer i = 0;
    return total + i;
  }
  public static Integer blocks(List<Map<String, Object>> rows) {
    Map<String, Object> best = new Map<String, Object>();
    for (Map<String, Object> line : rows) { best = line; }
    if (rows.size() > 1) { Integer line = 5; return line; }
    Map<Date, Set<Id>> dates = new Map<Date, Set<Id>>();
    for (Integer k = 0; k < 2; k++) { Map<Date, Set<Id>> temp = new Map<Date, Set<Id>>(); dates = temp; }
    for (Date d : dates.keySet()) { List<Id> temp = new List<Id>(dates.get(d)); }
    return 0;
  }
  public static void stillWrong() {
    Map<String, Object> target = new Map<String, Object>();
    Integer number = 1;
    target = number;
  }
}
`,
	}
	result := analyzeDeclarationProjectWithAPIVersion(t, files, "45.0")
	var messages []string
	for _, diag := range result.Diagnostics {
		messages = append(messages, diag.Code+" "+diag.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, unwanted := range []string{`redeclares local variable "i"`, `variable "best"`, `variable "dates"`} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("later sibling-scope declaration leaked backwards (%s):\n%s", unwanted, joined)
		}
	}
	if !strings.Contains(joined, `assigns Integer to Map<String,Object> variable "target"`) {
		t.Fatalf("a real type mismatch was no longer reported:\n%s", joined)
	}
}
