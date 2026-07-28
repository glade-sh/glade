package sema

import (
	"strings"
	"testing"
)

func TestSalesforceOracleAcceptsObjectHashCode(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ObjectHashCode.cls": `public class ObjectHashCode {
  public Integer value(Object input) {
    return input.hashCode();
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("Salesforce accepts Object.hashCode(): %#v", result.Diagnostics)
	}
}

func TestSalesforceOracleAcceptsCustomExceptionGetMessageOverride(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ServiceFailureException.cls": `public class ServiceFailureException extends Exception {
  public override String getMessage() {
    return 'service failed';
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("Salesforce accepts a custom Exception getMessage override: %#v", result.Diagnostics)
	}
}

func TestSalesforceOracleRejectsAuraEnabledOverloads(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"AuraEnabledOverloads.cls": `public class AuraEnabledOverloads {
  @AuraEnabled public static String find() {
    return 'all';
  }
  @AuraEnabled public static String find(String query) {
    return query;
  }
}`,
	})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA032" || !strings.Contains(result.Diagnostics[0].Message, "AuraEnabled methods cannot be overloaded: find") {
		t.Fatalf("Salesforce rejects only the overloaded AuraEnabled methods contract: %#v", result.Diagnostics)
	}
}
