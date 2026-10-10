package sema

import (
	"strings"
	"testing"
)

func TestLanguageRollupSalesforceContracts(t *testing.T) {
	files := map[string]string{
		"Abstract.cls":  `public abstract class AbstractBase { public abstract override String toString(); }`,
		"Exception.cls": `public class ExceptionProbe { public void run() { try { throw new System.SerializationException('x'); } catch (System.SerializationException ex) { System.debug(ex); } } }`,
		"Switch.cls":    `public class SwitchProbe { public String run(Integer value) { String result = 'other'; switch on value { when -1 { result = 'negative'; } when else { } } return result; } }`,
	}
	result := analyzeDeclarationProjectWithAPIVersion(t, files, "67.0")
	if result.HasErrors() {
		t.Fatalf("Salesforce language contracts were rejected: %#v", result.Diagnostics)
	}
}

func TestNegativeSwitchCasesKeepDistinctValues(t *testing.T) {
	for name, source := range map[string]string{
		"distinct":  `public class SwitchProbe { public void run(Integer value) { switch on value { when -1 { } when -2 { } } } }`,
		"duplicate": `public class SwitchProbe { public void run(Integer value) { switch on value { when -1 { } when -1 { } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Switch.cls": source}, "67.0")
			duplicate := false
			for _, diagnostic := range result.Diagnostics {
				duplicate = duplicate || strings.Contains(diagnostic.Message, "duplicate switch branch value")
			}
			if name == "distinct" && duplicate {
				t.Fatalf("distinct negative cases were collapsed: %#v", result.Diagnostics)
			}
			if name == "duplicate" && !duplicate {
				t.Fatalf("duplicate negative case was not diagnosed: %#v", result.Diagnostics)
			}
		})
	}
}
