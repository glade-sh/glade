# Lookup followups case table

| Item | Native probes at API 62 and 67 | Boundary and rejection |
|---|---|---|
| Record type comma label | RecordTypeId and RecordType.Name, bare and quoted comma label; developer-name control | Other comma label; plain label; null lookup |
| Numeric/date commas | Bare and quoted 1,234; 12.5,67.25; MM/DD/YYYY date lists; rejected ISO/written date forms | Both list members, joined-number value, outside values; metadata rejection captured |
| Explicit unflagged person type | Explicit Named_Person and Named_Individual person record types | Generic default differs; missing LastName; business control |
| Person flag order | PersonAccount visibility over an existing record type, profile and permission set sources | Both source orders; namespace/name deduplication; Account-only boundary |
