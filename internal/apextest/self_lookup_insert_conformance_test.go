package apextest

import "testing"

func TestSelfLookupInsertNativeConformance(t *testing.T) {
	scalarSchemaNativeConformance(t, "self_lookup_insert.json", "SelfLinkProbe", "LINK_ROWS|", "|LINK_END", 2)
}
