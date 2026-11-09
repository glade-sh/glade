package sema

import "testing"

func TestCapturedComparisonInitializer(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": `
public class Probe {
  public static Boolean run() {
    Integer a=1,b=2,c=3,d=2;
    Boolean x = a < b && c > d;
    return x;
  }
}`}, api)
			if result.HasErrors() {
				t.Fatalf("captured expression rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestComparisonInitializerDoesNotCreateLocal(t *testing.T) {
	body := "Boolean x = a < b && c > d; List<String> later = new List<String>();"
	locals := map[string]string{}
	for _, m := range findSemaLocalDeclMatches(body) {
		locals[body[m[4]:m[5]]] = body[m[2]:m[3]]
	}
	if _, ok := locals["d"]; ok {
		t.Errorf("comparison operand treated as a declaration: %v", locals)
	}
	if locals["later"] != "List<String>" {
		t.Errorf("genuine generic declaration not found: %v", locals)
	}
}
