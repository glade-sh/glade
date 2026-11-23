# API references in an after-update Flow email

Captured 2026-11-23 in a fresh 30-day Developer scratch org, then deleted and verified gone. At Apex API 62.0 and 67.0 an isolated test runs an after-update Flow deployed at API 67.0. The Flow sends a plain text template with the partner endpoint and a formula extracting its host, and writes the same text to a new record. The test replaces host and org ID with placeholders before recording the result. The rows do not establish email delivery, exact host, org ID, or other API reference versions.
