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
				if !standardSuperChainContains(t, symbols, name, "CartExtension.CommerceObject") {
					t.Errorf("%s does not reach CartExtension.CommerceObject; native accepts the assignment", name)
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

// standardSuperChainContains follows SuperClass links through symbols. Native
// compile rows establish assignability, so any ancestor depth satisfies them.
func standardSuperChainContains(t *testing.T, symbols []TypeSymbol, name, ancestor string) bool {
	t.Helper()
	for depth := 0; depth < 8; depth++ {
		symbol := requireStandardSymbol(t, symbols, name)
		if symbol.SuperClass == "" || symbol.SuperClass == "Object" {
			return false
		}
		if symbol.SuperClass == ancestor {
			return true
		}
		name = symbol.SuperClass
	}
	return false
}
