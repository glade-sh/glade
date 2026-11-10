package sema

import (
	"maps"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// assertOnlyUndeclaredFieldRejection allows only the native rejections of the
// undeclared missingns references, once each: the field type (ruling 49;
// namespace_external_positions ExtFieldOnlyV035) and the static member call
// (receiver_member_opaque_package C01, C02 at 62.0 and 67.0, which report
// both, with or without a same-spelled local class). The local CartLike
// class adds nothing.
func assertOnlyUndeclaredFieldRejection(t *testing.T, label string, result Result) {
	t.Helper()
	want := map[string]int{
		"GLADESEMA002|Invalid type: missingns.CartLike.SubmitOptions":       1,
		"GLADESEMA008|Variable does not exist: missingns.CartLike.Instance": 1,
	}
	got := map[string]int{}
	for _, d := range result.Diagnostics {
		if d.Severity == diagnostic.Error {
			got[d.Code+"|"+d.NativeMessage]++
		}
	}
	if !maps.Equal(got, want) {
		t.Fatalf("%s: errors %v, want %v: %#v", label, got, want, result.Diagnostics)
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
