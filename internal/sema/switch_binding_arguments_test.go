package sema

import (
	"strings"
	"testing"
)

// The Apex Developer Guide's "Example with sObjects" uses the variable bound by
// `when Account a` within that when block. The binding is an implicit cast, so it
// can also be passed to a method expecting Account.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_switch.htm
func TestSwitchTypeBindingMethodArguments(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for name, source := range map[string]string{
				"implicit call": `public class Selection {
  public Boolean run(SObject row) {
    switch on row {
      when Account selected { return accept(selected); }
      when else { return false; }
    }
  }
  private Boolean accept(Account row) { return true; }
}`,
				"qualified call and nested block": `public class Selection {
  public Boolean run(SObject row) {
    switch on row {
      when Account selected { if (selected != null) { return this.accept(selected); } return false; }
      when else { return false; }
    }
  }
  private Boolean accept(Account row) { return true; }
}`,
				"ordinary parameter control": `public class Selection {
  public Boolean run(Account selected) { return accept(selected); }
  private Boolean accept(Account row) { return true; }
}`,
			} {
				t.Run(name, func(t *testing.T) {
					result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Selection.cls": source}, api)
					if result.HasErrors() {
						t.Fatalf("method argument binding rejected: %#v", result.Diagnostics)
					}
				})
			}
			for name, body := range map[string]string{
				"after switch":   `switch on row { when Account selected {} when else {} } accept(selected);`,
				"sibling clause": `switch on row { when Account selected {} when null { accept(selected); } when else {} }`,
				"comment control": `// switch on row { when Account selected { accept(selected); } }
    accept(selected);`,
				"string control": `String text = 'switch on row { when Account selected { accept(selected); } }'; accept(selected);`,
			} {
				t.Run(name, func(t *testing.T) {
					source := "public class Selection { public void run(SObject row) { " + body + " } private void accept(Account row) {} }"
					result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Selection.cls": source}, api)
					for _, diagnostic := range result.Diagnostics {
						if diagnostic.Code == "GLADESEMA013" && strings.Contains(diagnostic.Message, "selected") {
							return
						}
					}
					t.Fatalf("expected unknown binding outside a when clause: %#v", result.Diagnostics)
				})
			}
		})
	}
}
