Two fresh 30-day Developer scratch orgs were created for this capture and deleted after use.
One had the MultiCurrency feature enabled; the other used the default single-currency shape.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI project deployment of a synthetic LightningComponentBundle.
Each deployment used the listed API version in both deployment and bundle metadata.
The component source is probe.js and probe.html. Bundle metadata set isExposed false.
The invalid cases replace CurrencyIsoCode with NotARealField__c. Results record
schema import validation during deployment, not runtime field access.
