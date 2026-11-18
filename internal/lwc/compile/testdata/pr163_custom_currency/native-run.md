Two fresh 30-day Developer scratch orgs were created and deleted after capture.
One had MultiCurrency enabled; the other had the default single-currency shape.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI project deployment of synthetic object and LWC metadata.
The object definition is Shape__c.object-meta.xml, deployed before probe.js and
probe.html. Each bundle used the listed API version and isExposed false. Invalid
cases substitute NotARealField__c for CurrencyIsoCode. Results record deployment
schema validation, not runtime field access.
