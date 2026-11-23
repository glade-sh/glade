# API origin formula with an appended path

Captured 2026-11-23 in a fresh 30-day Developer scratch org, then deleted and verified gone. An after-save Flow deployed at API 67.0 writes a text template containing a formula that extracts a partner endpoint's origin with LEFT and FIND, substitutes a hostname suffix, and appends a path. Isolated Apex tests at 62.0 and 67.0 observed HTTPS and the replacement suffix followed by the path. The test records booleans rather than the native org hostname. The local fixture uses a synthetic Salesforce-shaped domain to exercise the substitution.
