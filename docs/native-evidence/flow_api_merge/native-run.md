# API merge references in flow text templates

Captured 2026-11-23 in a fresh 30-day Developer scratch org. The org was deleted and deletion verified. At Apex API 62.0 and 67.0, an isolated test inserted a Contact that triggered an after-save Flow deployed at API 67.0. The flow wrote a plain text template to Description; the template included direct partner and enterprise API references and a formula that refers to the partner endpoint. The test reduced each host and org ID to placeholders before recording the result.

The rows establish the SOAP path and version, but not the exact host or org ID. Other endpoint versions, global variables, routes, and formula shapes were not captured.
