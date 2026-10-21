package apextest

import "testing"

func TestCatalogProductNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "catalog_product_native", "CatalogProductTest", []string{"62.0", "67.0"}, 3)
}
