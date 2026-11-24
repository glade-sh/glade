# Locale text ordering capture

The fixture under `internal/apextest/testdata/conformance/locale_order_466` contains 35 synthetic Account values: a null, punctuation, digits, case, accented letters, ligatures, Japanese, Han, Greek, and Cyrillic. AccountNumber is the position in `seed.apex`; BillingCity is the text value.

The anonymous rows were captured in a fresh one-day Developer Edition scratch org by updating the running user's LocaleSidKey in a separate transaction and executing `query.apex` at API 62.0 and 67.0. LanguageLocaleKey remained en_US.

The IsTest rows were captured in a second fresh one-day Developer Edition scratch org. The deployed `LocaleCapture62` and `LocaleCapture67` classes each contain six test methods, one for each locale. Each creates a synthetic user and runs the same seed and query inside `System.runAs`. The method deliberately fails an assertion to report the observed locale and label sequence; class metadata specifies API 62.0 or 67.0, and the test-run request used the CLI default API version. All 12 test rows match the corresponding anonymous rows. Both orgs were deleted and subsequent org display returned not found.

The query uses `ORDER BY BillingCity ASC NULLS FIRST, AccountNumber ASC` to fix ties. `org.tsv` records both routes independently; the conformance test compares every native row with both Glade anonymous Apex and named IsTest execution.
