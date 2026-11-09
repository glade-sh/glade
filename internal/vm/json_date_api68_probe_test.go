package vm

import (
	"testing"
)

// api68plumb_date_probe records the anonymous 62/67 rows and 68 = 67.
func TestJSONRootDateNullAPI68Anonymous(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Date valid = (Date) JSON.deserialize('"2026-11-03"', Date.class);
System.assertEquals('2026-11-03', String.valueOf(valid));
String observed = 'returned';
try {
    JSON.deserialize('null', Date.class);
} catch (Exception problem) {
    observed = problem.getTypeName() + '|' + problem.getMessage();
}
System.assertEquals('System.JSONException|Invalid format: "null"', observed);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
