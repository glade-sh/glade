# Locale text ordering capture

A fresh one-day Developer Edition scratch org was seeded with 35 synthetic Account records using the fixture's `seed.apex`. Each record's AccountNumber is its three-digit position in the seed list; BillingCity is the test value. The seed includes one null, ASCII punctuation and digits, uppercase and lowercase, accented Latin letters, ligatures, Japanese, Han, Greek, and Cyrillic.

For each of de_DE, sv_SE, ja_JP, fr_FR, es_ES, and en_US, the running user's LocaleSidKey was updated in a separate transaction. LanguageLocaleKey remained en_US. The fixture's `query.apex` was executed anonymously at API 62.0 and 67.0. It returns labels from `ORDER BY BillingCity ASC NULLS FIRST, AccountNumber ASC`. The tie breaker fixes the result when collation treats strings equally. `org.tsv` contains the resulting labels and observed running locale. The two API versions agreed for each locale.

The scratch org was deleted after capture. A subsequent org display reported the org was not found.
