package typesys

import (
	"github.com/glade-sh/glade/internal/apexast"
	"slices"
	"testing"
)

// commerce_extension_shapes/org.tsv captures accepted calculate overrides and
// empty subclasses, with rejected abstract construction, at APIs 62 and 67.
func TestCommerceExtensionNativeCatalogColdAndWarm(t *testing.T) {
	for _, route := range []string{"cold", "warm"} {
		t.Run(route, func(t *testing.T) {
			symbols := StandardPlatformSymbols()
			if route == "warm" {
				symbols = StandardPlatformSymbolView()
			}
			for _, name := range []string{"InventoryCartCalculator", "PricingCartCalculator", "ShippingCartCalculator"} {
				symbol := requireStandardSymbol(t, symbols, "CartExtension."+name)
				if !slices.Contains(symbol.Modifiers, "abstract") {
					t.Errorf("%s must be abstract", name)
				}
				found := false
				for _, member := range symbol.Members {
					if member.Kind == apexast.DeclarationMethod && member.Name == "calculate" {
						found = true
						if !slices.Contains(member.Modifiers, "virtual") {
							t.Errorf("%s.calculate must be virtual", name)
						}
					}
				}
				if !found {
					t.Errorf("%s.calculate missing", name)
				}
			}
		})
	}
}
