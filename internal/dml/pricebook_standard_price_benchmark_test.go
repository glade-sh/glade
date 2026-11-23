package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// BenchmarkCustomPriceInsert prices a product with an existing standard price
// in an org that has many other price entries. Both revisions must succeed.
func BenchmarkCustomPriceInsert(b *testing.B) {
	base := testOrg()
	for _, name := range []string{"Product2", "Pricebook2", "PricebookEntry"} {
		storage.EnsureStandardObject(&base, name)
	}
	productID := storage.ID("01t000000000001")
	standardID := storage.ID("01s000000000001")
	customID := storage.ID("01s000000000002")
	products := base.Objects["Product2"]
	products.PutRecord(productID, storage.Record{ID: productID, Object: "Product2", Fields: map[string]storage.Value{"Name": storage.StringValue("Product"), "IsActive": storage.BooleanValue(true)}})
	base.Objects["Product2"] = products
	books := base.Objects["Pricebook2"]
	books.PutRecord(standardID, storage.Record{ID: standardID, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(true)}})
	books.PutRecord(customID, storage.Record{ID: customID, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(false)}})
	base.Objects["Pricebook2"] = books
	entries := base.Objects["PricebookEntry"]
	ids := storage.NewIDGenerator(map[string]string{"PricebookEntry": entries.Definition.KeyPrefix, "Product2": products.Definition.KeyPrefix})
	for i := 0; i < 200; i++ {
		id, err := ids.Next("PricebookEntry")
		if err != nil {
			b.Fatal(err)
		}
		p, err := ids.Next("Product2")
		if err != nil {
			b.Fatal(err)
		}
		if i == 0 {
			p = productID
		}
		entries.PutRecord(id, storage.Record{ID: id, Object: "PricebookEntry", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(standardID), "Product2Id": storage.IDValue(p), "UnitPrice": storage.DecimalValue("5"), "IsActive": storage.BooleanValue(true)}})
	}
	base.Objects["PricebookEntry"] = entries
	candidate := storage.Record{Object: "PricebookEntry", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(customID), "Product2Id": storage.IDValue(productID), "UnitPrice": storage.DecimalValue("4"), "IsActive": storage.BooleanValue(true)}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		org := base.Clone()
		engine := NewEngine(&org)
		b.StartTimer()
		result := engine.Insert([]storage.Record{candidate})
		if len(result) != 1 || !result[0].Success {
			b.Fatalf("insert: %+v", result)
		}
	}
}

// BenchmarkCustomPriceBulkInsert inserts many custom prices over an org with
// many standard prices. The check must stay linear in the batch.
func BenchmarkCustomPriceBulkInsert(b *testing.B) {
	base := testOrg()
	for _, name := range []string{"Product2", "Pricebook2", "PricebookEntry"} {
		storage.EnsureStandardObject(&base, name)
	}
	standardID := storage.ID("01s000000000001")
	customID := storage.ID("01s000000000002")
	books := base.Objects["Pricebook2"]
	books.PutRecord(standardID, storage.Record{ID: standardID, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(true)}})
	books.PutRecord(customID, storage.Record{ID: customID, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(false)}})
	base.Objects["Pricebook2"] = books
	products := base.Objects["Product2"]
	entries := base.Objects["PricebookEntry"]
	ids := storage.NewIDGenerator(map[string]string{"PricebookEntry": entries.Definition.KeyPrefix, "Product2": products.Definition.KeyPrefix})
	var batch []storage.Record
	for i := 0; i < 300; i++ {
		id, _ := ids.Next("PricebookEntry")
		p, _ := ids.Next("Product2")
		products.PutRecord(p, storage.Record{ID: p, Object: "Product2", Fields: map[string]storage.Value{"Name": storage.StringValue("Product"), "IsActive": storage.BooleanValue(true)}})
		entries.PutRecord(id, storage.Record{ID: id, Object: "PricebookEntry", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(standardID), "Product2Id": storage.IDValue(p), "UnitPrice": storage.DecimalValue("5"), "IsActive": storage.BooleanValue(true)}})
		batch = append(batch, storage.Record{Object: "PricebookEntry", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(customID), "Product2Id": storage.IDValue(p), "UnitPrice": storage.DecimalValue("4"), "IsActive": storage.BooleanValue(true)}})
	}
	base.Objects["Product2"] = products
	base.Objects["PricebookEntry"] = entries
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		org := base.Clone()
		engine := NewEngine(&org)
		b.StartTimer()
		for _, result := range engine.Insert(batch) {
			if !result.Success {
				b.Fatalf("insert: %+v", result)
			}
		}
	}
}
