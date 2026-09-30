package sema

import (
	"strings"
	"testing"
)

func TestClassicForConditionUsesInitializerScope(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public String minute { get; set; }
  public Probe() {
    minute = 'preserved';
    for (Integer minute = 0; minute < 60; minute++) {}
  }
}`,
	}, "66.0")
	if result.HasErrors() {
		t.Fatalf("loop condition used String property instead of Integer local: %#v", result.Diagnostics)
	}
}

func TestClassicForConditionKeepsOuterPropertyOutsideLoop(t *testing.T) {
	for name, body := range map[string]string{
		"after loop":     `for (Integer minute = 0; minute < 60; minute++) {} if (minute < 60) {}`,
		"no initializer": `for (; minute < 60;) { break; }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe {
  public String minute { get; set; }
  public Probe() { ` + body + ` }
}`,
			}, "66.0")
			orderingErrors := 0
			for _, d := range result.Diagnostics {
				if d.Code == "GLADESEMA019" && strings.Contains(d.Message, "ordering operator requires numeric operands") {
					orderingErrors++
				}
			}
			if orderingErrors != 1 {
				t.Fatalf("expected exactly one String property ordering rejection, got %#v", result.Diagnostics)
			}
		})
	}
}
