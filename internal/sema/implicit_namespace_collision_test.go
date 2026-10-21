package sema

import "testing"

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
	if result.HasErrors() {
		t.Fatalf("local member plus qualified missing package: %#v", result.Diagnostics)
	}
}

func TestAnalyzeImplicitNamespaceCollisionMissingOnly(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"PackageReference.cls": `public class PackageReference {
  missingns.CartLike.SubmitOptions options;
  public void save() { missingns.CartLike.Instance.save(); }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("qualified missing-package twin: %#v", result.Diagnostics)
	}
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
