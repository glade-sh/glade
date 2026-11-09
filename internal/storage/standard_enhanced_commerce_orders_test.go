package storage

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const enhancedCommerceOrdersDir = "../apextest/testdata/conformance/standard_enhanced_commerce_orders/"

// The isolated captures show one gate. The OrderSaveBehaviorBoth feature alone
// adds no Order or OrderItem field. The enhanced commerce orders setting adds
// every field below at API 62 and 67. Order.RelatedOrderType appears only at
// API 67, so the policy does not gate it.
func TestEnhancedCommerceOrdersNativeMembership(t *testing.T) {
	file, err := os.Open(enhancedCommerceOrdersDir + "named-fields.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ api, variant, object, field string }
	native := map[key]bool{}
	for _, row := range table[1:] {
		native[key{row[0], row[1], row[2], row[3]}] = true
	}
	variants := map[string][]string{
		"feature-only":         {"OrderSaveBehaviorBoth"},
		"setting-only":         {"EnhancedCommerceOrders"},
		"feature-and-setting":  {"OrderSaveBehaviorBoth", "EnhancedCommerceOrders"},
		"capitalized-settings": {"EnhancedCommerceOrders"},
	}
	gated := 0
	for _, api := range []string{"62.0", "67.0"} {
		for variant, features := range variants {
			t.Run(api+"/"+variant, func(t *testing.T) {
				for _, object := range []string{"Order", "OrderItem"} {
					definition := ObjectDefinition{APIName: object}
					EnsureStandardObjectFieldsForFeatures(&definition, features)
					for k := range native {
						if k.api != api || k.object != object || k.variant != "setting-only" {
							continue
						}
						if native[key{api, "feature-only", object, k.field}] {
							continue
						}
						// RelatedOrderType is the one API 67 only addition.
						if k.field == "RelatedOrderType" {
							continue
						}
						want := native[key{api, variant, object, k.field}]
						if _, got := ResolveFieldName(definition, "", k.field); got != want {
							t.Errorf("%s.%s membership=%t, native=%t", object, k.field, got, want)
						}
						gated++
					}
				}
			})
		}
	}
	if gated == 0 {
		t.Fatal("no gated fields were compared")
	}
	for _, field := range []struct{ object, name, typ, label string }{
		{"Order", "OrderedDate", "datetime", "Ordered Date"},
		{"OrderItem", "TotalLineAmount", "currency", "Product Subtotal"},
	} {
		definition := ObjectDefinition{APIName: field.object}
		EnsureStandardObjectFieldsForFeatures(&definition, []string{"EnhancedCommerceOrders"})
		name, ok := ResolveFieldName(definition, "", field.name)
		if !ok {
			t.Fatalf("%s.%s missing with the enhanced setting", field.object, field.name)
		}
		got := definition.Fields[name]
		if !strings.EqualFold(got.DisplayType, field.typ) || got.Label != field.label {
			t.Errorf("%s.%s facets type=%s label=%q", field.object, field.name, got.DisplayType, got.Label)
		}
	}
}

func TestEnhancedCommerceOrdersGrantedDescribeIsPolicyInput(t *testing.T) {
	file, err := os.Open(enhancedCommerceOrdersDir + "granted-describe.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	stream, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	var described map[string]map[string]map[string][]struct {
		Name         string
		Label        string
		Type         string
		Nillable     bool
		Createable   bool
		Updateable   bool
		Length       int
		Precision    int
		Scale        int
		Permissioned bool `json:"permissionable"`
	}
	if err := json.NewDecoder(stream).Decode(&described); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range described[api]["setting-only"]["OrderItem"] {
			if field.Name != "TotalLineAmount" {
				continue
			}
			if field.Type != "currency" || field.Precision != 18 || field.Scale != 2 || !field.Createable || !field.Updateable {
				t.Fatalf("native TotalLineAmount facets changed: %+v", field)
			}
			definition := ObjectDefinition{APIName: "OrderItem"}
			EnsureStandardObjectFieldsForFeatures(&definition, []string{"EnhancedCommerceOrders"})
			got := definition.Fields["TotalLineAmount"]
			if got.Precision != field.Precision || got.Scale != field.Scale || got.Createable == nil || !*got.Createable {
				t.Errorf("TotalLineAmount facets precision=%d scale=%d createable=%v", got.Precision, got.Scale, got.Createable)
			}
		}
	}
}
