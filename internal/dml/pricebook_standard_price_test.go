package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestPricebookStandardPriceDirectDML(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"Product2", "Pricebook2", "PricebookEntry"} {
		storage.EnsureStandardObject(&org, name)
	}
	e := NewEngine(&org)
	p := scopeInsert(t, &e, "Product2", map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "IsActive": storage.BooleanValue(true)})
	b := scopeInsert(t, &e, "Pricebook2", map[string]storage.Value{"Name": storage.StringValue("Custom"), "IsActive": storage.BooleanValue(true)})
	record := storage.Record{Object: "PricebookEntry", Fields: map[string]storage.Value{"Product2Id": storage.IDValue(p), "Pricebook2Id": storage.IDValue(b), "UnitPrice": storage.DecimalValue("5"), "IsActive": storage.BooleanValue(true)}}
	result := e.Insert([]storage.Record{record})[0]
	if result.StatusCode != "STANDARD_PRICE_NOT_DEFINED" {
		t.Fatalf("unexpected result: %+v; book fields=%+v product fields=%+v", result, org.Objects["Pricebook2"].Records[b].Fields, org.Objects["Product2"].Records[p].Fields)
	}
}

func TestPricebookStandardPriceIndexInvalidation(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"Product2", "Pricebook2", "PricebookEntry"} {
		storage.EnsureStandardObject(&org, name)
	}
	e := NewEngine(&org)
	standard := storage.ID("01s000000000001")
	books := org.Objects["Pricebook2"]
	books.PutRecord(standard, storage.Record{ID: standard, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(true)}})
	org.Objects["Pricebook2"] = books
	product := scopeInsert(t, &e, "Product2", map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "IsActive": storage.BooleanValue(true)})
	price := func(book storage.ID) Result {
		return e.Insert([]storage.Record{{Object: "PricebookEntry", Fields: map[string]storage.Value{"Product2Id": storage.IDValue(product), "Pricebook2Id": storage.IDValue(book), "UnitPrice": storage.DecimalValue("5"), "IsActive": storage.BooleanValue(true)}}})[0]
	}
	custom := func() storage.ID {
		return scopeInsert(t, &e, "Pricebook2", map[string]storage.Value{"Name": storage.StringValue("Custom"), "IsActive": storage.BooleanValue(true)})
	}
	want := func(step string, ok bool) Result {
		t.Helper()
		r := price(custom())
		if r.Success != ok || (!ok && r.StatusCode != "STANDARD_PRICE_NOT_DEFINED") {
			t.Fatalf("%s: success=%v want %v: %+v", step, r.Success, ok, r)
		}
		return r
	}
	want("no standard price", false)
	want("again, cached miss", false)
	std := price(standard)
	if !std.Success {
		t.Fatalf("standard insert: %+v", std)
	}
	want("after entry insert", true)
	want("cached hit", true)
	if r := e.Delete([]storage.Record{{Object: "PricebookEntry", ID: std.ID}})[0]; !r.Success {
		t.Fatalf("delete: %+v", r)
	}
	want("after entry delete", false)
	// A replaced entry row moves the price to a custom book.
	entries := org.Objects["PricebookEntry"]
	entries.PutRecord("01u000000000009", storage.Record{ID: "01u000000000009", Object: "PricebookEntry", Fields: map[string]storage.Value{"Product2Id": storage.IDValue(product), "Pricebook2Id": storage.IDValue(standard), "IsActive": storage.BooleanValue(true)}})
	org.Objects["PricebookEntry"] = entries
	want("after entry put", true)
	// The book stops being standard.
	books = org.Objects["Pricebook2"]
	books.PutRecord(standard, storage.Record{ID: standard, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(false)}})
	org.Objects["Pricebook2"] = books
	want("after book write", false)
	// A rollback replaces the org with a snapshot taken before the entry.
	snapshot := org.Clone()
	books = org.Objects["Pricebook2"]
	books.PutRecord(standard, storage.Record{ID: standard, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(true)}})
	org.Objects["Pricebook2"] = books
	want("before rollback", true)
	e.Org = &snapshot
	want("after rollback", false)
}

func TestPricebookStandardPriceIndexBulkIncremental(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"Product2", "Pricebook2", "PricebookEntry"} {
		storage.EnsureStandardObject(&org, name)
	}
	e := NewEngine(&org)
	standard := storage.ID("01s000000000001")
	books := org.Objects["Pricebook2"]
	books.PutRecord(standard, storage.Record{ID: standard, Object: "Pricebook2", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true), "IsStandard": storage.BooleanValue(true)}})
	org.Objects["Pricebook2"] = books
	fields := func(product, book storage.ID) map[string]storage.Value {
		return map[string]storage.Value{"Product2Id": storage.IDValue(product), "Pricebook2Id": storage.IDValue(book), "UnitPrice": storage.DecimalValue("5"), "IsActive": storage.BooleanValue(true)}
	}
	var products []storage.ID
	for i := 0; i < 4; i++ {
		products = append(products, scopeInsert(t, &e, "Product2", map[string]storage.Value{"Name": storage.StringValue("P"), "IsActive": storage.BooleanValue(true)}))
	}
	custom := scopeInsert(t, &e, "Pricebook2", map[string]storage.Value{"Name": storage.StringValue("Custom"), "IsActive": storage.BooleanValue(true)})
	other := scopeInsert(t, &e, "Pricebook2", map[string]storage.Value{"Name": storage.StringValue("Other"), "IsActive": storage.BooleanValue(true)})
	scopeInsert(t, &e, "PricebookEntry", fields(products[0], standard))
	steps := []struct {
		product, book storage.ID
		ok            bool
	}{
		{products[0], custom, true}, {products[1], custom, false}, {products[0], other, true},
		{products[1], other, false}, {products[1], standard, true}, {products[1], custom, true},
		{products[2], custom, false}, {products[3], standard, true}, {products[3], other, true}, {products[2], other, false},
	}
	for i, step := range steps {
		r := e.Insert([]storage.Record{{Object: "PricebookEntry", Fields: fields(step.product, step.book)}})[0]
		if r.Success != step.ok {
			t.Fatalf("step %d: success=%v want %v: %+v", i, r.Success, step.ok, r)
		}
	}
}
