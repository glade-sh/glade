# Custom price without a standard price

Captured 2026-11-23 in a fresh 30-day Developer scratch org. The org was deleted and deletion verified. Apex API versions 62.0 and 67.0 ran anonymous Apex and an isolated test. The anonymous case inserts a custom price book; the test creates a custom price book named Standard Price Book. Both insert a price for a product without a standard price. Each returned STANDARD_PRICE_NOT_DEFINED. The observations and capture facts are in `internal/apextest/testdata/conformance/pricebook_entry_standard_price/`.

The cases do not establish behavior for currencies, inactive standard prices, bulk inserts, or update and upsert operations.
