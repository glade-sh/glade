package typesys

import "testing"

// Native compile rows: ../sema/testdata/cart_extension_platform_types/org.tsv,
// cart_item_list_constructor and commerce_object_assignability, API 62/67.
func TestCartExtensionPlatformSymbolsConstructorAndInheritanceRoutes(t *testing.T) {
	for _, route := range []struct {
		name    string
		symbols func() []TypeSymbol
	}{
		{"owned-symbol-clone", StandardPlatformSymbols},
		{"read-only-symbol-view", StandardPlatformSymbolView},
	} {
		t.Run(route.name, func(t *testing.T) {
			symbols := route.symbols()
			t.Run("direct-list-constructor", func(t *testing.T) {
				list := requireStandardSymbol(t, symbols, "CartExtension.CartItemList")
				if constructors := standardConstructorSignatures(list); !list.ConstructorsAuthoritative || len(constructors) != 0 {
					t.Fatalf("native rejects list construction; authoritative=%v constructors=%v", list.ConstructorsAuthoritative, constructors)
				}
			})
			for _, name := range []string{"CartExtension.CartItem", "CartExtension.CartDeliveryGroup"} {
				t.Run("commerce-assignment-and-list-argument/"+name, func(t *testing.T) {
					if symbol := requireStandardSymbol(t, symbols, name); symbol.SuperClass != "CartExtension.CommerceObject" {
						t.Fatalf("native accepts CommerceObject assignment and list arguments; superclass=%q", symbol.SuperClass)
					}
				})
			}
		})
	}
}
