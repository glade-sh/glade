package typesys

import "testing"

func TestStandardPlatformSymbolsModelMetadataLayoutRetrieveHierarchy(t *testing.T) {
	symbols := StandardPlatformSymbols()
	layout := requireStandardSymbol(t, symbols, "Metadata.Layout")
	if layout.SuperClass != "Metadata.Metadata" {
		t.Fatalf("Metadata.Layout superclass = %q, want Metadata.Metadata", layout.SuperClass)
	}
	operations := requireStandardSymbol(t, symbols, "Metadata.Operations")
	requireStandardMethodReturn(t, operations, "retrieve", []string{"Metadata.MetadataType", "List<String>"}, "List<Metadata.Metadata>", true)
}
