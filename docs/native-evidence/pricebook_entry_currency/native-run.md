# Pricebook entry with the default currency

Captured 2026-11-23 in a fresh 30-day Developer scratch org with MultiCurrency enabled, then deleted and verified gone. At Apex API 62.0 and 67.0, an isolated test inserted active product and custom book records using the default currency. The custom price failed until a standard price for the same product was inserted. Other currencies and inactive price records were not captured.
