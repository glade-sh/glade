package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Twins of the named deploy rows in cart_extension_platform_types/org.tsv:8-19.
func TestCartPlatformAnonymousRoutesNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
		for _, row := range []struct {
			name, source string
			accepted     bool
		}{
			{"list-constructor", `new CartExtension.CartItemList();`, false},
			{"item-return-assignability", `CartExtension.CartItem item; CartExtension.CommerceObject value=item;`, true},
			{"group-return-assignability", `CartExtension.CartDeliveryGroup groupValue; CartExtension.CommerceObject value=groupValue;`, true},
			{"list-add-remove-item", `CartExtension.CartItemList items; CartExtension.CartItem item; items.add(item); items.remove(item);`, true},
			{"list-add-remove-group", `CartExtension.CartItemList items; CartExtension.CartDeliveryGroup groupValue; items.add(groupValue); items.remove(groupValue);`, true},
			{"list-add-remove-string", `CartExtension.CartItemList items; items.add('invalid'); items.remove('invalid');`, false},
			{"list-enhanced-for", `CartExtension.CartItemList items; for(CartExtension.CartItem item:items) {}`, false},
		} {
			t.Run(api+"/anonymous/"+row.name, func(t *testing.T) {
				result := AnalyzeAnonymous(index, row.source, api)
				if result.HasErrors() == row.accepted {
					t.Fatalf("native accepted=%t; Glade: %+v", row.accepted, result.Diagnostics)
				}
			})
		}
	}
}
