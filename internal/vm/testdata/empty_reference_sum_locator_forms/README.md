# Additional empty reference SUM construction observations

Five syntactic forms remain pending at API 62, 67 and 68, in anonymous and
`@IsTest` contexts: 30 cells. All expectations in `probe-cases.json` are null.
Only four forms request additional observations (24 cells). Picklist equality
reuses the exact six pending `valid-noneligible` cells from the precedence
packet; its old row name records the prior candidate's boundary. Do not rename
or silently discard that observation because its admission status changed.

All constructors use the default non-cursor locator overload, `SUM(Amount)`
grouped by reference `Opportunity.AccountId`, and `AccountId IN :keys` where
`keys` is an empty declared lexical `Set<Id>`. Capture these five criteria:

1. `CloseDate < TODAY`
2. `(CloseDate < TODAY OR Amount > 100)`
3. `StageName != 'Prospecting'`
4. `Amount < 200`
5. `StageName = 'Prospecting'` (reuse the precedence packet row)

The runtime candidate uses schema field types, not these field names or values.
It recognizes only Date `< TODAY`, parenthesized Date `< TODAY` OR the SUM's
Decimal `> literal`, the SUM's Decimal `< literal`, and Picklist `=`/`!= string`.
Other date literals, reversed OR order, other operators, arbitrary string fields
and additional Boolean nesting remain excluded. Redundant parentheses normalize
to the same parsed condition shape; this gate does not add source-depth rules
to the existing validator. The original query validator runs on
each admitted construction. This packet does not establish native acceptance.

New named observers pass the bound set as a method parameter. Change class
metadata and project source API for the chosen API. Anonymous payloads log
`CRITERIA_OBSERVATION`; named payloads use an intentional final assertion as
transport. Capture phase, bound size, created flag and full exception type and
message. Exceptions are observations, never skipped rows.

Reuse the original empty-reference-SUM packet for nonempty, cursor, COUNT,
malformed inputs and the fenced constructor/iterator controls. Reuse the
precedence packet for missing-column/error-order rows. No new iterator support
is asserted for these forms; the local candidate's iterator remains rejected.
Native results that disagree require narrowing or dropping the candidate.

`LocatorCriteriaFormsCandidateTest` is a local before/after proof with five
constructor regressions and five unchanged rejection controls. Its admitted
constructors also assert the existing iterator fence. These assertions are
local diagnostics, not a Salesforce oracle.
