# Empty reference SUM locator error precedence

This is a pending observation packet, not a native oracle. Run the twelve rows
in `probe-cases.json` at API 62, 67 and 68, in anonymous and `@IsTest` contexts:
72 cells. All native expectations remain null. Change the named class metadata
and project source API to the selected version before deployment.

The decisive row is `missing-string-criterion`: a non-cursor default-overload
grouped `SUM(Amount)` by reference field `Opportunity.AccountId`, with an active
empty `Set<Id>` method/local bind and one literal string equality on the absent
synthetic field `MissingCriterion`. Capture constructor exception type and full
message. It must distinguish missing-column from aggregate/queryMore rejection.
Do not infer a native result from the isolated local candidate.

`valid-noneligible` distinguished validation from admission in the initial
precedence candidate. Its exact picklist-equality payload is now reused by the
additional criteria-forms packet, where admission is a pending candidate. Keep
the row identity and null native expectation; do not infer a native result from
this change. `eligible-empty-create` and
`eligible-empty-iterate` retain separate stages for the previously pending empty
SUM construction and its fenced iterator. These observations overlap the
existing empty-reference-SUM packet; reuse matching native cells when available.
The nonempty, list, numeric missing field, malformed literal/syntax and cursor
rows bound the candidate; do not generalize precedence to those shapes.

Owned `query_locators.json` N015 establishes the API62/67 matching grouped COUNT
negative control. S003/S009 establish ordinary locator missing-column text.
They do not establish aggregate-plus-missing-column precedence or any new API68
answer. The public API documents query errors but provide no oracle here for
the ordering of these two simultaneous faults.

The anonymous files log `PRECEDENCE_OBSERVATION`; named observer classes end
with an intentional assertion carrying the same payload. That assertion is
transport only. Exceptions are observations, never skipped rows. Capture
`phase`, `created`, `boundSize`, exception type and full message; distinguish
construction and iteration instead of collapsing failures.

`LocatorCriteriaCandidateTest` is a separate local proof class. Its assertions
describe the isolated candidate and retained controls, not native acceptance.
