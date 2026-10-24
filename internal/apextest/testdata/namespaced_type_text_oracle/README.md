# Namespaced type and field-set observations

Synthetic project observations from Salesforce at source APIs 62.0 and 67.0.
`org.tsv` is the sole expected-value source for `TestNamespacedTypeTextOracleNativeRoutes`.
The 24 R rows fill `cases[i].expectedRuntime[api]` in the consumer fixture
`internal/apextest/testdata/conformance/namespaced_type_text.json`.
Numbers below identify `org.tsv` lines and consumer JSON expectation lines.

| Case | Question | API 62 row -> JSON line | API 67 row -> JSON line |
| --- | --- | --- | --- |
| R001 | Stub rendering and generated stub type name | 4 -> 27 | 5 -> 28 |
| R002 | Stub provider type and provider fields | 6 -> 37 | 7 -> 38 |
| R003 | Generic method parameter Type tokens seen by StubProvider | 8 -> 47 | 9 -> 48 |
| R004 | Generic Type tokens from class literals and Type.forName | 10 -> 57 | 11 -> 58 |
| R005 | Failed cast to an owned class | 12 -> 67 | 13 -> 68 |
| R006 | Failed cast to List of an owned class | 14 -> 77 | 15 -> 78 |
| R007 | Namespaced custom metadata type text | 16 -> 87 | 17 -> 88 |
| R008 | Global describe keys matching an owned unprefixed name | 18 -> 97 | 19 -> 98 |
| R009 | Global describe containsKey with an unprefixed name | 20 -> 107 | 21 -> 108 |
| R010 | Global describe get with an unprefixed name | 22 -> 117 | 23 -> 118 |
| R011 | Global describe containsKey with a lower-case unprefixed name | 24 -> 127 | 25 -> 128 |
| R012 | Field set member paths for an owned custom field | 26 -> 137 | 27 -> 138 |

F001 named rows 2-3 and anonymous rows 28-29 pin direct field-set paths.
They supply the observation used by the consumer field-set test; custom paths
are `nsx__Size__c` and `nsx__Owner__r.Name`, while `Name` is unchanged.
T001 rows 30-31 cover anonymous provider, scalar/list cast rejection, global
describe, token-map and describe-map field-set routes. T002 rows 32-33 cover
qualified lookup, foreign-namespace rejection and non-event share boundaries.
T003 rows 34-35 cover qualified/unqualified global-map keys, get and containsKey
for an event-enabled object. R004 retains builtin type-token boundaries.

The named F001 and anonymous helper sources preserve the captured terminal
assertions; the Go runner substitutes native assertions without live requests.
Named helpers use the row API; anonymous helpers retain source API 67.0.
No product runtime changes are included.

The unchanged integration runtime compiles this test and passes 10 of 34 cells;
24 fail on field paths, provider text, cast text and global-map behavior.
Example: F001 expects `nsx__Size__c` but receives `Size__c` for the field path.
