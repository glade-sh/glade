package typesys

import "testing"

// Native NULL_UPCAST/STANDARD/CUSTOM assertions at APIs 62/67 are retained in
// ../apextest/testdata/conformance/address_location/org.tsv.
func TestAddressLocationPlatformHierarchy(t *testing.T) {
	for _, symbols := range [][]TypeSymbol{StandardPlatformSymbols(), StandardPlatformSymbolView()} {
		address := requireStandardSymbol(t, symbols, "Address")
		if address.SuperClass != "System.Location" {
			t.Fatalf("Address extends %q, want System.Location", address.SuperClass)
		}
	}
}
