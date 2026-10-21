package sema

import "testing"

// Native API62/64/67 accepts these contextual query/null shapes. See the owned
// inline_query_coalesce_null conformance fixture and native-capture.
func TestAnalyzeInlineSOQLCoalesceNullReturn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		wantErrors   bool
	}{
		{"record", `public class QueryProbe {
  public Account read() { return [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, false},
		{"list", `public class QueryProbe {
  public List<Account> read() { return [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, false},
		{"ordinary_list", `public class QueryProbe {
  public Account read(List<Account> rows) { return rows ?? null; }
}`, true},
		{"different_record", `public class QueryProbe {
  public Contact read() { return [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QueryProbe.cls": tc.source}, "64.0")
			if result.HasErrors() != tc.wantErrors {
				t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
			}
		})
	}
}

func TestAnalyzeInlineSOQLCoalesceNullAssignments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		wantErrors   bool
	}{
		{"initialize_record", `public class QueryProbe {
  public void read() { Account value = [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, false},
		{"assign_record", `public class QueryProbe {
  public void read() { Account value; value = [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, false},
		{"initialize_list", `public class QueryProbe {
  public void read() { List<Account> value = [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, false},
		{"initialize_ordinary_list", `public class QueryProbe {
  public void read(List<Account> rows) { Account value = rows ?? null; }
}`, true},
		{"initialize_different_record", `public class QueryProbe {
  public void read() { Contact value = [SELECT Id FROM Account LIMIT 1] ?? null; }
}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QueryProbe.cls": tc.source}, "64.0")
			if result.HasErrors() != tc.wantErrors {
				t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
			}
		})
	}
}
