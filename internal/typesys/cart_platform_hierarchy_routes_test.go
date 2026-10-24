package typesys

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// Native rows: ../sema/testdata/cart_extension_platform_types/org.tsv:8-15,
// source APIs 62/67: no list constructor; item and delivery group are CommerceObject.
func TestCartPlatformHierarchyCatalogRoute(t *testing.T) {
	for _, route := range []string{"cold-catalog", "cached-catalog"} {
		t.Run(route, func(t *testing.T) {
			symbols := StandardPlatformSymbols()
			if route == "cached-catalog" {
				symbols = StandardPlatformSymbolView()
			}
			for _, name := range []string{"CartExtension.CartItem", "CartExtension.CartDeliveryGroup"} {
				if symbol := requireStandardSymbol(t, symbols, name); symbol.SuperClass != "CartExtension.CommerceObject" {
					t.Errorf("%s extends %s; native CommerceObject", name, symbol.SuperClass)
				}
			}
			list := requireStandardSymbol(t, symbols, "CartExtension.CartItemList")
			if !list.ConstructorsAuthoritative {
				t.Error("native list constructor restriction is missing")
			}
			for _, member := range list.Members {
				if member.Kind == apexast.DeclarationConstructor {
					t.Errorf("native rejects list constructor: %+v", member)
				}
			}
		})
	}
}
