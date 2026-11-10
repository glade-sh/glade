package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// assertOnlyUndeclaredFieldRejection allows only the native rejection of the
// undeclared missingns field type (ruling 49; namespace_external_positions
// ExtFieldOnlyV035). The local CartLike class adds nothing.
func assertOnlyUndeclaredFieldRejection(t *testing.T, label string, result Result) {
	t.Helper()
	found := false
	for _, d := range result.Diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		if d.NativeMessage != "Invalid type: missingns.CartLike.SubmitOptions" {
			t.Fatalf("%s: %#v", label, result.Diagnostics)
		}
		found = true
	}
	if !found {
		t.Fatalf("%s: undeclared field type accepted", label)
	}
}

func TestAnalyzeImplicitNamespaceCollisionLocalMember(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"CartLike.cls": `public class CartLike {
  public Id a() { return b(); }
  private Id b() { return null; }
}`,
		"PackageReference.cls": `public class PackageReference {
  missingns.CartLike.SubmitOptions options;
  public void save() { missingns.CartLike.Instance.save(); }
}`,
		"CartLikeTest.cls": `@IsTest private class CartLikeTest {
  @IsTest static void localCall() { System.assertEquals(null, new CartLike().a()); }
}`,
	})
	assertOnlyUndeclaredFieldRejection(t, "local member plus qualified missing package", result)
}

func TestAnalyzeImplicitNamespaceCollisionMissingOnly(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"PackageReference.cls": `public class PackageReference {
  missingns.CartLike.SubmitOptions options;
  public void save() { missingns.CartLike.Instance.save(); }
}`,
	})
	assertOnlyUndeclaredFieldRejection(t, "qualified missing-package twin", result)
}

func TestAnalyzeImplicitNamespaceCollisionRejectsUnknownLocalMember(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"CartLike.cls": `public class CartLike { public Id a() { return b(); } }`,
		"PackageReference.cls": `public class PackageReference {
  public void save() { missingns.CartLike.Instance.save(); }
}`,
	})
	if !result.HasErrors() {
		t.Fatal("missing-package tolerance must not accept an unknown local b()")
	}
}
