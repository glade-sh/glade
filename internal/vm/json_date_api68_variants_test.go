package vm

import (
	"fmt"
	"testing"
)

// Native adjacent controls cover both entry points and both null spellings.
func TestJSONRootDateNullAPI68Variants(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ method, document string }{
			{"deserialize", `"null"`},
			{"deserializeStrict", `null`},
			{"deserializeStrict", `"null"`},
		} {
			t.Run(api+"/"+tc.method+"/"+tc.document, func(t *testing.T) {
				source := fmt.Sprintf(`
String observed = 'returned';
try { JSON.%s('%s', Date.class); }
catch (Exception problem) { observed = problem.getTypeName() + '|' + problem.getMessage(); }
System.assertEquals('System.JSONException|Invalid format: "null"', observed);
`, tc.method, tc.document)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
