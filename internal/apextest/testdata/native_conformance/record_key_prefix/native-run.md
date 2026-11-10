Long-lived personal Developer Edition org (not a fresh scratch org), capture
date 2026-11-10. PrefixProbe__c and the three PrefixCapture classes were
validated together with a check-only deployment at each class's declared API
version, running the PrefixCapture tests; all nine methods passed and nothing
was saved. anonymous.apex ran through executeAnonymous at API 68.0, the org's
API version; its Account insert was rolled back. No org, user or record
identifiers are recorded.
