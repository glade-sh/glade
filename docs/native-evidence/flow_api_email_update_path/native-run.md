# API formula and path in an after-update Flow email

Captured 2026-11-23 in a fresh 30-day Developer scratch org, then deleted and verified gone. At Apex API 62.0 and 67.0 an isolated test runs an after-update Flow deployed at API 67.0. The Flow sends a plain text template with a partner endpoint and a formula extracting its host, replacing a hostname suffix and appending a path. It writes the same text to a new record. The test replaces host and org ID with placeholders and records whether the replacement occurred. The local fixture uses a synthetic Salesforce-shaped origin.
