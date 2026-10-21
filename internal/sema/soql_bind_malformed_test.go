package sema

import "testing"

func TestSOQLBindCapturedMalformedIndex(t *testing.T) {
	body := "Object r; List<Account> a=[SELECT Id,Name FROM Account]; Integer i=0; r=[SELECT Id FROM Account WHERE Id=:a[0.Id].size();"
	const want = "Unrecognized symbol 'i', which is not a valid Apex identifier."
	diagnostics := soqlBindMalformedDiagnostics("Malformed.cls", body, true)
	if len(diagnostics) != 1 || diagnostics[0].Message != want {
		t.Fatalf("%v", diagnostics)
	}
	for _, control := range []string{
		"Object r; List<Account> a=[SELECT Id FROM Account]; r=[SELECT Id FROM Account WHERE Id=:a[0].Id].size();",
		"Object r; List<Account> a=[SELECT Id FROM Account]; r=[SELECT Id FROM Account WHERE Id=:a['x'].Id].size();",
		"String text='[SELECT Id FROM Account WHERE Id=:a[0.Id].size()';",
	} {
		if diagnostics := soqlBindMalformedDiagnostics("Control.cls", control, true); len(diagnostics) != 0 {
			t.Fatalf("control: %v", diagnostics)
		}
	}
}
