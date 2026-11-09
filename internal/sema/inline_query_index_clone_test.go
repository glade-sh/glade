package sema

import "testing"

// Native API 67 compiles a method call on an indexed inline query result; the
// element type is the queried SObject (shapemisc_capture FmProbe.mk).
func TestAnalyzeIndexedInlineSOQLMethodCall(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		wantErrors   bool
	}{
		{"clone_return", `public class QueryProbe {
  public Account read() { return [SELECT Id, Name FROM Account][0].clone(false, true, false, false); }
}`, false},
		{"clone_local", `public class QueryProbe {
  public Account read() { Account a = [SELECT Id FROM Account][0].clone(); return a; }
}`, false},
		{"wrong_type", `public class QueryProbe {
  public Contact read() { return [SELECT Id FROM Account][0].clone(); }
}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QueryProbe.cls": tc.source}, "67.0")
			if result.HasErrors() != tc.wantErrors {
				t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
			}
		})
	}
}

// An indexed aggregate query yields AggregateResult, not the queried SObject.
func TestAnalyzeIndexedAggregateInlineSOQL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		wantErrors   bool
	}{
		{"max_picklist", `public class AggProbe {
  public void run(List<Opportunity> opps) {
    AggregateResult r = [SELECT Max(StageName) maxStage FROM Opportunity WHERE Id = :new Map<Id, Opportunity>(opps).keySet() WITH SYSTEM_MODE][0];
    String s = (String) r.get('maxStage');
  }
}`, false},
		{"min_amount", `public class AggProbe {
  public void run() { AggregateResult r = [SELECT MIN(Amount) m FROM Opportunity][0]; }
}`, false},
		{"group_by", `public class AggProbe {
  public void run() { AggregateResult r = [SELECT COUNT(Id) c, StageName FROM Opportunity GROUP BY StageName][0]; }
}`, false},
		{"plain_wrong_type", `public class AggProbe {
  public void run() { AggregateResult r = [SELECT Id FROM Opportunity][0]; }
}`, true},
		{"agg_to_sobject", `public class AggProbe {
  public void run() { Opportunity o = [SELECT MAX(Amount) m FROM Opportunity][0]; }
}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"AggProbe.cls": tc.source}, "67.0")
			if result.HasErrors() != tc.wantErrors {
				t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
			}
		})
	}
}
