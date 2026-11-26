package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// validateCustomPriceInsert handles the captured custom-price inserts: the
// same product must already have a price in a standard book.
func (e *Engine) validateCustomPriceInsert(objectName string, record storage.Record) error {
	if !strings.EqualFold(objectName, "PricebookEntry") || e.Org == nil {
		return nil
	}
	active, ok := record.GetField("IsActive")
	if !ok || active.Kind != storage.ValueBoolean || (!active.Boolean && !(e.recordTriggeredFlow > 0 && e.flowFaultInTest())) {
		return nil
	}
	if e.Org.HasFeature("MultiCurrency") {
		entry, ok := record.GetField("CurrencyIsoCode")
		def := e.Org.Objects["PricebookEntry"].Definition
		field, found := def.Fields["CurrencyIsoCode"]
		defaultCurrency, valid := storage.CurrencyDefaultForField(e.Org, e.systemUserID(), field)
		if !ok || !found || !valid || entry.Kind != storage.ValueString || !strings.EqualFold(entry.String, defaultCurrency.String) {
			return nil
		}
	}
	bookValue, bok := record.GetField("Pricebook2Id")
	productValue, pok := record.GetField("Product2Id")
	if !bok || !pok {
		return nil
	}
	bookID, productID := idFromStorageValue(bookValue), idFromStorageValue(productValue)
	books, bok := e.Org.Objects["Pricebook2"]
	entries, eok := e.Org.Objects["PricebookEntry"]
	products, pok := e.Org.Objects["Product2"]
	if !bok || !eok || !pok {
		return nil
	}
	_, book, ok := storage.LookupRecordByID(books.Records, bookID)
	if !ok {
		return nil
	}
	standard, ok := book.GetField("IsStandard")
	if !ok || standard.Kind != storage.ValueBoolean || standard.Boolean {
		return nil
	}
	bookActive, ok := book.GetField("IsActive")
	if !ok || bookActive.Kind != storage.ValueBoolean || !bookActive.Boolean {
		return nil
	}
	_, product, ok := storage.LookupRecordByID(products.Records, productID)
	if !ok {
		return nil
	}
	productActive, ok := product.GetField("IsActive")
	if !ok || productActive.Kind != storage.ValueBoolean || !productActive.Boolean {
		return nil
	}
	if e.productHasStandardPrice(books, entries, productID) {
		return nil
	}
	return dmlErrorf("STANDARD_PRICE_NOT_DEFINED", nil, "Before creating a custom price, create a standard price.")
}

// standardPriceIndex lists the products that have an entry in a standard book.
// It is valid for exactly the contents named by the two record tokens, so any
// write to either object, a rollback or a replaced org map misses the cache.
type standardPriceIndex struct {
	books, entries storage.RecordsToken
	products       map[string]struct{}
}

func standardPriceKey(id storage.ID) string {
	if len(id) >= 15 {
		return string(id[:15])
	}
	return string(id)
}

func (e *Engine) productHasStandardPrice(books, entries storage.ObjectState, productID storage.ID) bool {
	bookToken, bok := books.RecordsToken()
	entryToken, eok := entries.RecordsToken()
	cached := bok && eok && e.RecordCreations != nil
	if cached && e.RecordCreations.standardPrices != nil && e.RecordCreations.standardPrices.books == bookToken && e.RecordCreations.standardPrices.entries == entryToken {
		_, ok := e.RecordCreations.standardPrices.products[standardPriceKey(productID)]
		return ok
	}
	if !cached || e.RecordCreations.standardPriceMisses == 0 {
		// The first check on an engine scans with an early exit. A bulk load
		// pays for the index from the second check on.
		e.RecordCreations.standardPriceMisses++
		return scanStandardPrice(books, entries, productID)
	}
	products := make(map[string]struct{})
	standardBooks := make(map[string]bool)
	for _, entry := range entries.Records {
		if entry.System.IsDeleted {
			continue
		}
		entryProduct, ok := entry.GetField("Product2Id")
		if !ok {
			continue
		}
		entryBook, ok := entry.GetField("Pricebook2Id")
		if !ok {
			continue
		}
		bookID := idFromStorageValue(entryBook)
		key := standardPriceKey(bookID)
		standard, known := standardBooks[key]
		if !known {
			if _, pricedBook, ok := storage.LookupRecordByID(books.Records, bookID); ok {
				flag, ok := pricedBook.GetField("IsStandard")
				standard = ok && flag.Kind == storage.ValueBoolean && flag.Boolean
			}
			standardBooks[key] = standard
		}
		if standard {
			products[standardPriceKey(idFromStorageValue(entryProduct))] = struct{}{}
		}
	}
	if cached {
		e.RecordCreations.standardPrices = &standardPriceIndex{books: bookToken, entries: entryToken, products: products}
	}
	_, ok := products[standardPriceKey(productID)]
	return ok
}

func scanStandardPrice(books, entries storage.ObjectState, productID storage.ID) bool {
	for _, entry := range entries.Records {
		if entry.System.IsDeleted {
			continue
		}
		entryProduct, ok := entry.GetField("Product2Id")
		if !ok || !storage.IDsEqual(idFromStorageValue(entryProduct), productID) {
			continue
		}
		entryBook, ok := entry.GetField("Pricebook2Id")
		if !ok {
			continue
		}
		if bookIsStandard(books, idFromStorageValue(entryBook)) {
			return true
		}
	}
	return false
}

func bookIsStandard(books storage.ObjectState, bookID storage.ID) bool {
	_, book, ok := storage.LookupRecordByID(books.Records, bookID)
	if !ok {
		return false
	}
	flag, ok := book.GetField("IsStandard")
	return ok && flag.Kind == storage.ValueBoolean && flag.Boolean
}

// noteEntryInsert keeps a valid index current across the engine's own insert
// of a price entry. The index survives only when the insert was the single
// write since the index was built; any other write already moved the token.
func (e *Engine) noteEntryInsert(objectName string, entries storage.ObjectState, stored storage.Record) {
	if e.RecordCreations == nil {
		return
	}
	idx := e.RecordCreations.standardPrices
	if idx == nil || !strings.EqualFold(objectName, "PricebookEntry") {
		return
	}
	books := e.Org.Objects["Pricebook2"]
	after, ok := entries.RecordsToken()
	bookToken, bok := books.RecordsToken()
	if !ok || !bok || bookToken != idx.books || !after.FollowsOne(idx.entries) {
		e.RecordCreations.standardPrices = nil
		return
	}
	if !stored.System.IsDeleted {
		product, pok := stored.GetField("Product2Id")
		book, bkok := stored.GetField("Pricebook2Id")
		if pok && bkok && bookIsStandard(books, idFromStorageValue(book)) {
			idx.products[standardPriceKey(idFromStorageValue(product))] = struct{}{}
		}
	}
	idx.entries = after
}

// validateInsertReferences runs the reference checks of an insert and then the
// price entry check. It keeps the price work out of insertOne.
//
//go:noinline
func (e *Engine) validateInsertReferences(definition storage.ObjectDefinition, record storage.Record, ordinaryInsert bool) error {
	operation := referenceValidationInternal
	if ordinaryInsert {
		operation = referenceValidationInsert
	}
	if err := e.validateReferencesWithLoadedView(definition, record, operation); err != nil {
		return err
	}
	if len(record.Object) == len("PricebookEntry") {
		return e.validateCustomPriceInsert(record.Object, record)
	}
	return nil
}

// indexInsertedRecord updates the insert-time indexes after a record is
// stored, including the standard price index for a price entry.
//
//go:noinline
func (e *Engine) indexInsertedRecord(objectName string, definition storage.ObjectDefinition, record storage.Record) {
	e.addUniqueIndexRecord(objectName, definition, record)
	if len(objectName) == len("PricebookEntry") {
		entries := e.Org.Objects[objectName]
		e.noteEntryInsert(objectName, entries, entries.Records[record.ID])
	}
}
