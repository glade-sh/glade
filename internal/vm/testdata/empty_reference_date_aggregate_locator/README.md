# Empty reference Date aggregate locator observations

Owned black-box captures at source and transport APIs 62, 67 and 68, in anonymous
and check-only @IsTest contexts, confirm all three constructor shapes. Each seeds
one otherwise-matching Opportunity and passes an empty or matching Set<Id> into
an active parameter, cloning it before constructing the query. The shapes are
MAX(Date) with that Date < TODAY, MAX(Date) with (that Date < TODAY OR Decimal >
number), and COUNT(Date) with Decimal < number. COUNT(field) is distinct from
COUNT(). Fields are selected by schema type, never by field-name exceptions.

All 18 empty constructor observations create a handle and preserve getQuery().
All 18 matching constructor observations reject with the captured QueryException.
Six matching Account grouped COUNT(Id) controls independently retain that rejection.
`native-observations.json` exports every observed value, source API, transport API,
and execution context. The data-driven conformance test asserts all observed
constructor and empty-iterator cells. No native result is inferred from local or application success.

All 18 empty iterator observations create an iterator with hasNext() == false.
Only these admitted Date aggregate shapes with an empty validated snapshot use
the ordinary empty iterator path. The construction-only restriction remains for
unobserved SUM candidates and any unexpectedly populated aggregate snapshot.
General aggregate locators remain restricted outside this captured scope. No
populated aggregate iterator, cursor or accounting behavior is generalized.

The named project contains the exact compact capture sources. The first attempt
used repeated query strings and exceeded the metadata assertion-message limit;
only complete recaptured frames are used. Named check-only deployment rolls back
both metadata and records. `anonymous-observation.apex` is self-contained and uses
the existing native-capture exception transport to return all four observations and
roll back its transaction. It does not require a persistent helper or runAs.

Only parenthesized criteria, membership-first AND, a reference GROUP BY, an active
lexical empty Set<Id>, the default locator overload, and one aggregate qualify.
Other forms remain local scope controls, not claimed native rejection observations.
The original empty SUM, missing-field precedence and other criteria packets retain
their independent evidence status. These Date captures do not establish those rows.
