Fresh Developer scratch org, API 62.0 and 67.0, anonymous Apex.
The object and field set metadata in this directory were deployed before capture.
Each .apex file was executed separately at each API version. org.tsv records
compile and execution outcomes; successful rows assert the field name or field
set size. The scratch org was deleted and a subsequent lookup failed.
Capture date: 2026-11-09.
A second fresh 30-day Developer scratch org received the object and two field
sets. typing.apex and typing_bad.apex were executed separately as anonymous
Apex at API 62.0 and 67.0; the two versioned ShorthandProbe test classes were
deployed and passed. The second org was deleted and lookup returned not found.
A third fresh Developer scratch org received Probe__c and both field sets.
The inventory.apex source filtered global-describe keys to the `__c` suffix;
it did not enumerate all objects. It also captured Probe__c's field-set map
keys. The exact JSON is retained in inventory62.json and inventory67.json.
The source compiled and executed at both API versions. The scratch org was
deleted and a subsequent lookup returned not found.
