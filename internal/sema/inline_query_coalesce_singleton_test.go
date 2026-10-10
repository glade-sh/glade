package sema

import "testing"

func TestAnalyzeAPI67NamedClassInlineSOQLCoalesceSingletonContexts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		source         string
		wantCompatible bool
	}{
		{
			name: "empty_query_record_fallback",
			source: `public class Probe {
  public void run() {
    Account selected = [SELECT Id, Name FROM Account WHERE Name = 'no matching account' LIMIT 1] ?? new Account(Name = 'fallback');
  }
}`,
			wantCompatible: true,
		},
		{
			name: "one_row_query_assignment_fallback",
			source: `public class Probe {
  public void run() {
    Account seed = new Account(Name = 'winner');
    insert seed;
    Account fallbackMarker;
    Account selected = [SELECT Id, Name FROM Account WHERE Id = :seed.Id LIMIT 1] ?? (fallbackMarker = new Account(Name = 'fallback'));
  }
}`,
			wantCompatible: true,
		},
		{
			name: "ordinary_list_coalesces_with_list",
			source: `public class Probe {
  public void run(List<Account> left, List<Account> right) {
    List<Account> selected = left ?? right;
  }
}`,
			wantCompatible: true,
		},
		{
			name: "query_coalesces_with_list",
			source: `public class Probe {
  public void run() {
    List<Account> selected = [SELECT Id, Name FROM Account LIMIT 1] ?? new List<Account>();
  }
}`,
			wantCompatible: true,
		},
		{
			name: "incompatible_scalar_operands_rejected",
			source: `public class Probe {
  public void run() {
    System.debug('value' ?? 1);
  }
}`,
		},
		{
			name: "ordinary_list_to_record_rejected",
			source: `public class Probe {
  public void run(List<Account> rows) {
    System.debug(rows ?? new Account(Name = 'fallback'));
  }
}`,
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": testCase.source}, "67.0")
			if testCase.wantCompatible {
				if result.HasErrors() {
					t.Fatalf("API67 named-class expression should type-check: %#v", result.Diagnostics)
				}
				return
			}
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("incompatible coalesce should retain GLADESEMA019: %#v", result.Diagnostics)
			}
		})
	}
}
