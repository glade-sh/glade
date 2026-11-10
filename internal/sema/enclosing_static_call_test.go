package sema

import (
	"strings"
	"testing"
)

func TestStaticCallThroughInnerClassResolvesEnclosingStaticMethod(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Hub.cls": `
public class Hub {
  public abstract class Adapter {}
  public static Boolean newResult(Boolean kept) { return kept; }
  public Boolean instanceOnly() { return true; }
}
`,
		"HubUser.cls": `
public class HubUser {
  public static Boolean ok() { return Hub.Adapter.newResult(false); }
  public static Boolean missing() { return Hub.Adapter.notThere(false); }
  public static Boolean notStatic() { return Hub.Adapter.instanceOnly(); }
}
`,
	})
	var unknown []string
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" {
			unknown = append(unknown, diag.Message)
		}
	}
	joined := strings.Join(unknown, "\n")
	if strings.Contains(joined, "Hub.Adapter.newResult") {
		t.Fatalf("enclosing static method was not resolved through the inner class: %s", joined)
	}
	if !strings.Contains(joined, "Hub.Adapter.notThere") {
		t.Fatalf("missing method was accepted: %#v", result.Diagnostics)
	}
	if !strings.Contains(joined, "Hub.Adapter.instanceOnly") {
		t.Fatalf("enclosing instance method was accepted as static: %#v", result.Diagnostics)
	}
}
