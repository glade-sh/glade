package sema

import "testing"

func TestTypeContractAllowsMetadataLayoutCastFromRetrieve(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public void run() {
    List<Metadata.Metadata> layouts = Metadata.Operations.retrieve(Metadata.MetadataType.Layout, new List<String>{'Account-Layout'});
    Metadata.Layout layout = (Metadata.Layout) layouts.get(0);
  }
}`,
	}, "48.0")
	if result.HasErrors() {
		t.Fatalf("Metadata.Layout cast from retrieve was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractRejectsMetadataSiblingCast(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public void run(Metadata.CustomMetadata value) {
    Metadata.Layout layout = (Metadata.Layout) value;
  }
}`,
	}, "48.0")
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
		t.Fatalf("Metadata sibling cast was accepted: %#v", result.Diagnostics)
	}
}
