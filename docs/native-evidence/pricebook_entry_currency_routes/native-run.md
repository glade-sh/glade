# Pricebook entry default currency routes

Captured 2026-11-23 in a fresh 30-day Developer scratch org with MultiCurrency enabled, then deleted and verified gone. At Apex API 62.0 and 67.0, anonymous Apex inserts an active custom price without a standard price, and an isolated test inserts an active product whose after-save Flow creates a price in a test-created custom book. Both receive STANDARD_PRICE_NOT_DEFINED. The Flow was deployed at API 67.0. Both use the default currency. Other currencies and direct inactive prices were not captured.
