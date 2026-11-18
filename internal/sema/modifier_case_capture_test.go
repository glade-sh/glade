package sema

import "testing"

func TestCapturedLocalModifierCase(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": `
public class Probe {
  public static Integer run() {
    FINAL Integer x = 7;
    Transient Integer y = 2;
    return x + y;
  }
}`}, api)
			if result.HasErrors() {
				t.Fatalf("captured declarations rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestCapturedLocalModifierScan(t *testing.T) {
	body := "FINAL Integer x = 7; Transient Integer y = 2;"
	got := map[string]string{}
	for _, match := range findSemaLocalDeclMatches(body) {
		got[body[match[4]:match[5]]] = body[match[2]:match[3]]
	}
	for _, name := range []string{"x", "y"} {
		if got[name] != "Integer" {
			t.Errorf("captured local %s: type = %q, want Integer", name, got[name])
		}
	}
}
