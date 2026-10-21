package sema

import "testing"

// Database.query returns record lists. DML accepts that result directly without
// requiring an intermediate list variable; query indexing still selects one row.
func TestDynamicQueryDMLAcceptsRecordResults(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"literal", `public class QueryDeleteProbe {
  public void run() { delete Database.query('SELECT Id FROM Account'); }
}`},
		{"query variable", `public class QueryDeleteProbe {
  public void run() {
    String query = 'SELECT Id FROM Account';
    delete Database.query(query);
  }
}`},
		{"query parameter", `public class QueryDeleteProbe {
  public void run(String query) { delete Database.query(query); }
}`},
		{"bound query", `public class QueryDeleteProbe {
  public void run(Set<Id> ids) {
    delete Database.query('SELECT Id FROM Account WHERE Id IN :ids');
  }
}`},
		{"inline query control", `public class QueryDeleteProbe {
  public void run() { delete [SELECT Id FROM Account]; }
}`},
		{"assigned list control", `public class QueryDeleteProbe {
  public void run(String query) {
    List<SObject> rows = Database.query(query);
    delete rows;
  }
}`},
		{"indexed row control", `public class QueryDeleteProbe {
  public void run(String query) {
    SObject row = Database.query(query)[0];
    delete row;
    delete Database.query(query)[0];
  }
}`},
	}
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, test := range cases {
				t.Run(test.name, func(t *testing.T) {
					result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QueryDeleteProbe.cls": test.source}, api)
					if result.HasErrors() {
						t.Fatalf("record DML target rejected: %#v", result.Diagnostics)
					}
				})
			}
		})
	}
}

func TestDynamicQueryDMLRejectsNonRecordAndMergeListTargets(t *testing.T) {
	cases := []struct {
		name, source, database string
	}{
		{"integer", `public class QueryDeleteProbe {
  public void run(Integer value) { delete value; }
}`, ""},
		{"string", `public class QueryDeleteProbe {
  public void run(String value) { delete value; }
}`, ""},
		{"string list", `public class QueryDeleteProbe {
  public void run(List<String> values) { delete values; }
}`, ""},
		{"project query returning string", `public class QueryDeleteProbe {
  public void run(String query) { delete Database.query(query); }
}`, `public class Database {
  public static String query(String query) { return 'ordinary text'; }
}`},
		{"project query result object", `public class QueryDeleteProbe {
  public void run() {
    Database.QueryResult value = new Database.QueryResult();
    delete value;
  }
}`, `public class Database { public class QueryResult {} }`},
		{"merge list master control", `public class QueryDeleteProbe {
  public void run(List<Account> masters, Account duplicate) { merge masters duplicate; }
}`, ""},
		{"dynamic query merge list master", `public class QueryDeleteProbe {
  public void run(String query, Account duplicate) { merge Database.query(query) duplicate; }
}`, ""},
	}
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, test := range cases {
				t.Run(test.name, func(t *testing.T) {
					files := map[string]string{"QueryDeleteProbe.cls": test.source}
					if test.database != "" {
						files["Database.cls"] = test.database
					}
					result := analyzeDeclarationProjectWithAPIVersion(t, files, api)
					if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
						t.Fatalf("non-record or merge-list target accepted: %#v", result.Diagnostics)
					}
				})
			}
		})
	}
}

func TestDynamicQueryDMLPreservesScalarIndexType(t *testing.T) {
	// The internal query-result collection's element contract remains one SObject,
	// independent of the DML-only normalization of the whole query result.
	sig, ok := semaCollectionMethodSignature("Database.QueryResult", "get")
	if !ok || sig.returnType != "SObject" {
		t.Fatalf("indexed query result contract = %#v, found %v; want SObject", sig, ok)
	}
}
