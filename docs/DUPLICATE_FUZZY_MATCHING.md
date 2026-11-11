# Duplicate fuzzy matching: public spec, native rows and the fitted model

Fixture: `internal/apextest/testdata/conformance/duplicate_fuzzy_matching/`
holds `org.tsv` (native verdicts with match confidences) and `cases.json` (the
agreement bounds the conformance test enforces). The test is
`TestFuzzyMatchingNativeRows` in `internal/dml/fuzzy_conformance_test.go`; the
direct formula checks are in `internal/dml/fuzzy_confidence_test.go`. The
scorers are in `internal/dml/fuzzy_match.go`, the row-proven dictionaries in
`internal/dml/fuzzy_tables.go`.

## Public documentation read (own words)

All pages are Salesforce Help articles.

- "Matching Methods Used in Matching Rules"
  (`sales.matching_rules_matching_methods.htm`). Each fuzzy method is tied to
  a field and takes the highest score of its algorithms ("Maximum") unless it
  is a weighted average. First Name: Exact, Initials, Jaro-Winkler, Name
  Variant, threshold 85. Last Name: Exact, Keyboard Distance, Metaphone 3,
  threshold 90. Company Name: Acronym, Exact, Syllable Alignment, threshold 70;
  words such as Inc and Corp are removed first, company names are normalized
  ("IBM" to "International Business Machines"), and for names of more than two
  words only the first two words take part in the match key. Phone: weighted
  average of country code 10, area code 50, next three digits 30, last four 10;
  threshold 80. City: Edit Distance and Exact, threshold 85. Street: weighted
  average of street name 50 (edit distance), number 20, suffix 15, suite 15;
  threshold 80. ZIP: first five digits 90, next four 10; threshold 80. Title:
  Acronym, Exact, Kullback-Leibler distance, threshold 50.
- "Matching Algorithms Used with Matching Methods"
  (`sales.matching_rules_matching_algorithms.htm`). Comparison ignores letter
  case. Worked examples with scores: Edit Distance "VP Sales" against "VP of
  Sales" scores 73; Jaro-Winkler "Johnny" against "Johny" scores 97;
  Kullback-Leibler "Director of Engineering" against "Engineering Director"
  scores 65; Syllable Alignment "Department of Energy" against "Department of
  Labor" scores 59 where Edit Distance gives 74; Initials "Jonathan" against
  "J" scores 100; Name Variant Bob/Robert 100, Bob/Bill 0.
- "How Match Keys Are Generated and Used in Matching Rules"
  (`sales.matching_rules_match_key_process.htm`). Field values are
  normalized, then combined into match keys. Records that share no match key
  are never compared, so the algorithms never see them. The notation gives the
  words and characters per word a key keeps: the custom-rule example is
  `First_Name (1,1) Last_Name Company (2,5)`. No match key is generated for
  Title.
- "Standard Contact Matching Rule and Standard Lead Matching Rule"
  (`sales.matching_rules_standard_contact_rule.htm`). Match key examples:
  First Name reduced to its initial, Last Name to its double metaphone code
  ("Doe" to "t", "Benioff" to "pnf"), Company to the first two words of five
  characters ("salesforce.com" to "sales"), Phone to the number without the
  leading 1 and the last four digits. The equations and per-field criteria of
  the standard rules; the Contact criteria list First Name with Metaphone 3 as
  well, and Account Name with Edit Distance instead of Syllable Alignment.

## Capture

- Two fresh 30-day Developer Edition scratch orgs, each deleted afterwards.
  Custom matching rules with one fuzzy item each (plus an exact Last Name item
  for Title, which the platform requires), a duplicate rule per matching rule.
  At most five active matching rules per object, so the Account Phone rule of
  the first run was not repeated.
- Verdicts come from `Datacloud.FindDuplicates.findDuplicates` in anonymous
  Apex: up to 50 records inserted in one transaction, then every value run as
  an unsaved candidate against them (up to 2,500 pairs per row). A pair is a
  native match when the existing record comes back in the rule's
  `MatchResult`; `getMatchConfidence()` is the third element of each match.
  Records are tagged by index; no record, org or user id is stored.
- Row ids: `object.field.method.blankBehavior` for the first run (fit rows),
  `Holdout.*` for its later rows, `Probe.*.<family>` for the second run's
  designed probes (position of an edit, suffix tables, suite designators,
  legal designators, same-key company spellings, Jaro-Winkler bands, name
  variants, phonetic and keyboard families, title word structure) and
  `Holdout2.*` for the second run's held-out rows, chosen before the fit and
  never tuned on.
- API 67.0 for every row; API 62.0 confirmation rows for the first run's rows
  and for six probe rows of the second run. Every 62.0 row equals its 67.0 row
  pair for pair, confidence for confidence.
- The match engine reported for custom rules with a fuzzy item is
  `FuzzyMatchEngine`.

## What the native rows show (recovered model)

The confidence `getMatchConfidence()` returns is a whole number. For a rule
with one fuzzy item of a Maximum method it is `(100 + score) / 2` where score
is the algorithm score below; for Phone and Street it is the weighted score
itself; for the Title rows (Title plus exact Last Name) it is the mean of the
two item scores. Glade therefore reports the mean of the item scores for a
custom rule, with an exact or blank match scoring 100.

- NATIVE City: match key = first six letters or digits of the accent-folded
  value with spaces and punctuation removed; a different key never matches
  (`Honolulu`/`Honalulu`, `Seattle`/`Seatle`, `Portxand`/`Portland`). With the
  same key, Edit Distance score `100 * (1 - d / longer length)` on the raw
  lower-cased strings (accents and dots count as edits: `Düsseldorf`/
  `Dusseldorf` 95, `St. Louis`/`St Louis` 94) decides at 85 (`Salt Lake City`/
  `Salt Lake Ciyt` 85.7 matches, `San Francisco`/`San Francsico` 84.6 does
  not). Confidence `(100 + score) / 2`.
- NATIVE Street: addresses are lower-cased, dots and number signs removed,
  commas spaced, ordinal words spelled as numbers (`Fifth` = `5th`). Sections:
  the number is a leading token starting with a digit (`221B` included); a
  single letter after it is dropped when it is `E` and otherwise stays with
  the number (`100 E Main St` matches `100 Main St`, `100 N Main St` matches
  nothing). The suffix is the first known suffix word after the name: St,
  Street, Ave, Avenue, Rd, Road, Blvd, Boulevard, Dr, Drive, Ln, Lane, Cir,
  Hwy (abbreviation and full word are one value); Way, Loop, Court, Ct,
  Place, Pl, Terrace, Circle, Highway, Square, Parkway, Trail and their
  abbreviations are not suffixes and stay in the name. `Rue` is a leading
  suffix word. After a known suffix a directional is dropped and the rest is
  the suite, less a designator word (Suite, Ste, Apt, Unit, Rm, Bldg, Fl and
  any other word); `#2`, `# 2` and `2` are the same suite. Without a known
  suffix a trailing number is the suite and a trailing directional is
  dropped; designator words then stay in the name, which is why
  `42 Wallaby Way` matches `42 Wallaby Way Ste 2` (name similarity 73) but not
  `42 Wallaby Way Unit 2` (69). Match key = number plus the first five
  characters of the name (`Washngton`/`Washington` never compared,
  `Lake Shore`/`Lakeshore` never compared). Weighted score S = number 20
  (exact), name 50 (edit similarity), suffix 15 (exact), suite 15 (exact),
  averaged over the sections present on both sides; a section present on one
  side only is left out (`123 Main St`/`123 Main St Suite 5` scores 100); a
  number blank on both sides counts 0 under NullNotAllowed and 20 under
  MatchBlanks. Match when S >= 80 (`Pkwy`/`Parkway` 80.5 matches, `Pl`/`Place`
  76 does not). Confidence `(S + edit similarity of the cleaned strings) / 2`:
  `15 Elm St`/`15 Elm Drive` = (82.35 + 58.3) / 2 = 70, `123 Main St`/
  `123 Main Street` = 87, `Oak St 100` against itself = 90 (number blank,
  trailing number as suite).
- NATIVE Company: lower case, `&` = `and`, hyphen = space, punctuation
  removed (`Salesforce.com` = `Salesforcecom`, `Margie's` = `Margies`), a
  leading `the`, `of`/`and`/`the` and one trailing legal designator dropped:
  Inc, Incorporated, Corp, Corporation, Co, Company, Ltd, Limited, LLC, LLP,
  LP, PLC, GmbH, AG, SA, Enterprises, NA. Not designators: Group, Holdings,
  International, Technologies, Systems, Software, Partners, Associates, Trust,
  Bank, USA, America, Pty Ltd, Bros; a second designator stays
  (`Acme Corporation Inc` is not `Acme`). Match key = first five characters of
  each of the first two words (`Adobe`/`Adobe Systems`, `Pfizer`/`Pfiser`,
  `Burger King`/`Burger Kings`, `Oracle`/`Orcle` are never compared). With the
  same key: identical words score 100, and so do words that differ only by
  doubled letters, a plural s on long words, a trailing e, or a trailing y/i
  (`Goldmann Sachs`, `Northwinds Traders`, `Hewlett Packards`, `Verizone`,
  `Industries`/`Industrie`/`Industri`); one extra or different word after the
  first two scores 91 (`Hewlett Packard Enterprise`, `International Business
  Systems`, `Blue Yonder Airlines`/`Blue Yonder`); two extra words do not
  match (`Blue Yonder Air Lines`/`Blue Yonder`). A vowel change with the same
  consonants scores 96 to 98 for a/o, o/u and a/e (`Nordstram`,
  `Caterpiller`, `Prosewear`) and nothing for e/i, e/o and i/y (`Verizen`,
  `Veryzon`, `Stanlie`, `Advisers`); a consonant change or a missing vowel
  does not match (`Microsft`, `Verison`, `Starbux`, `Morgan Stanly`). Acronym
  dictionary entries proven: IBM, AMD, GE, GM, HP, NASA; AA, HPE, DOE, MSFT,
  JNJ, J&J, P&G, BofA, CMO-style initials do not match their expansions.
- NATIVE First Name: Name Variant pairs score 100 (261 pairs proven, listed
  in `fuzzy_tables.go`; the dictionary is a graph, not transitive groups: Jon
  matches Joan and Jan, Joan matches Jenny, Jon does not match Jenny). Other
  pairs score Jaro-Winkler with a 0.1 prefix scale and no cap on the prefix
  length (`Chris`/`Christoper` 96 needs a five-letter prefix), confidence
  `(100 + score) / 2`, matching from about 0.87 (`Jon`/`Johny` 0.893 and
  `Brynle`/`Brynn` 0.893 match, `Kaia`/`Kaya` 0.867 does not). Pairs the
  dictionary keeps apart do not match even at high Jaro-Winkler
  (`Michael`/`Micheal`, `Jane`/`Jayne`, `Nicholas`/`Nicolas`, `Kate`/`Katie`,
  `Matthew`/`Matt`, `Tamsin`/`Tasmin`); Glade blocks the 59 proven pairs and
  any two names that both appear in a proven variant pair. `J` matches no
  longer name in a first-name-only rule.
- NATIVE Last Name: match key = four-character phonetic code of the
  accent-folded name (`Williams`/`Williamson` compared, `Smith`/`Smiht` never
  compared); equal full codes score 100 (`Taylor`/`Tayler`/`Tailor`/`Taylr`,
  `Clark`/`Klark`/`Clarck`, `Meyer`/`Meier`/`Mayer`, `Reid`/`Reed`/`Read`,
  `Wright`/`Right`/`Rite`, `Thompson`/`Thomson`, `Müller`/`Miller`). Codes
  keep D distinct from T (`Carter`/`Carder` 99, `Peterson`/`Pederson` 99) and B
  from P (`Campbell`/`Cambell` no match), treat accented letters as neither
  vowel nor consonant for the context rules (`García` against `Garcia` 99,
  against `Gracia` no match), keep H between vowels (`Cohen` against `Coen` no
  match) and keep a doubled L before a consonant (`Walker`/`Wallker` no match).
  Otherwise Keyboard Distance decides at 90: the strings are compared position
  by position, a substitution costs 0.08 per key step (the larger of the row
  and column distance on a QWERTY layout: `Martinez`/`Martinqz` 99,
  `Martinez`/`Martinpz` 97), an accent 0.16 (`Björk`/`Bjork` 98), an extra
  character 0.25 (`Williams`/`Williamson` 98), normalized by the longer length;
  confidence `(100 + score) / 2`.
- NATIVE Title: words are lower-cased, dots and commas trimmed, `&` = `and`.
  Exact when the content words (without of/and/the) are the same sequence or
  all words sort equal (`Sales Director`/`Director Sales` 100,
  `VP Sales`/`VP of Sales` 100, `Sales`/`Sales Sales` 100). A word equals any
  word it starts, two letters or more (`Dir`/`Director`, `Eng`/`Engineering`,
  `II`/`III`), but two single-word titles need an exact match (`Director`/
  `Directors` no match). Initials match (`CMO`/`Chief Marketing Officer`,
  `BDM`/`Business Development Manager`, `Bus Dev Manager`/`Business
  Development Manager` 100). Otherwise the confidence depends on the number of
  shared words k and the larger content-word count n: (1,2) 93, (1,3) 89,
  (1,4) 88, (2,3) 95, (2,4) 93, (2,5) 91, (3,4) 97, (3,5) 94, (4,5) 97; the
  same words in a different order with a stop word present 93 or 94
  (`Head of Product`/`Product Head`); shared words out of order lose about 3.
  One shared word against a five-word title matches only from the one-word
  side (`Global` finds `Senior Global Regional Operations Analyst`, not the
  reverse); Glade matches neither.
- NATIVE Phone: sections compared exactly; 10 digits, or 11 with a leading
  country digit, parse; letters or other lengths never match. Country 10,
  area 50, exchange 30, line 10; confidence equals the score (90, 80).
- NATIVE Zip: only the segment before the first hyphen or space decides;
  confidence 100.
- NATIVE blanks: with Match Blank Fields two blank values match (100); with
  the default they never do; one blank value never matches.
- NATIVE verdicts and confidences were identical at API 62.0 and 67.0 for
  every row captured at both versions.

## Glade fit

Recall is the share of native matching pairs Glade also matches; extra is the
count of pairs Glade matches that native did not; conf MAE is the mean
absolute difference between Glade's confidence and the native confidence over
the pairs both match. API 67.0 rows; in-sample is every row the model was
fitted to (both runs), held-out is the second run's `Holdout2` rows, scored
before any tuning on them. Before is the previous scorer set (branch
v038-dup-fuzzy at 240912244, which reported confidence 100 for every match).

| Method | Set | Before: recall, extra, conf MAE | After: recall, extra, conf MAE |
|---|---|---|---|
| City | in-sample | 319/363 (87.9%), 14, 1.28 | 363/363 (100%), 0, 0.00 |
| City | held-out | 78/80 (97.5%), 14, 1.77 | 80/80 (100%), 0, 0.00 |
| Street | in-sample | 972/1754 (55.4%), 78, 10.36 | 1698/1754 (96.8%), 50, 0.20 |
| Street | held-out | 108/164 (65.9%), 0, 7.28 | 158/164 (96.3%), 0, 0.33 |
| Company | in-sample | 640/1196 (53.5%), 0, 0.06 | 1158/1196 (96.8%), 16, 0.08 |
| Company | held-out | 72/118 (61.0%), 0, 0.00 | 114/118 (96.6%), 0, 0.02 |
| First Name | in-sample | 506/950 (53.3%), 10, 0.13 | 936/950 (98.5%), 0, 0.00 |
| First Name | held-out | 78/170 (45.9%), 6, 0.26 | 164/170 (96.5%), 0, 0.00 |
| Last Name | in-sample | 620/752 (82.4%), 58, 0.12 | 704/752 (93.6%), 8, 0.05 |
| Last Name | held-out | 154/170 (90.6%), 12, 0.08 | 166/170 (97.6%), 0, 0.13 |
| Title | in-sample | 942/942 (100%), 63, 7.01 | 939/942 (99.7%), 0, 0.25 |
| Title | held-out | 286/286 (100%), 0, 6.57 | 286/286 (100%), 0, 0.04 |
| Phone | in-sample | 649/649 (100%), 0, 7.58 | 649/649 (100%), 0, 0.00 |
| ZIP | in-sample | 95/95 (100%), 0, 0.00 | 95/95 (100%), 0, 0.00 |

The held-out First Name row scored 136/170 with 20 extra before its own
variant and blocked pairs were added to the tables (the extras were
dictionary pairs such as `Nicholas`/`Nicolas` and `Matthew`/`Matt`); the
figures above are the shipped tables, which include every proven pair. The
in-sample extras that remain are synthetic probe values the approximations
cannot separate (Street: `Oak Ct`/`Oak Court`-style unknown suffix pairs and
`Unit 3`/`Suite B` suite letters; Company: one-direction native matches such as
`Nordstrom`/`Nordstroms` and `Initech`/`Initechs`, `Goldman Sachss`; Last Name:
`Campbrll`).

## Gaps

- Syllable Alignment, Metaphone 3 and the Kullback-Leibler scoring are
  approximated by the row-fitted rules above. Native scores some same-key
  company pairs in one direction only (`Initec`/`Initech` 93, `Salesforce`/
  `Salesforcecom` 90, `Nordstrom`/`Nordstroms` 100) and Glade is symmetric.
  `Starbucks`/`Starbuck` (93), `Starbux` (95), `Lockheed`/`Lockhead` (100),
  `Hughes`/`Huse` (100) and `Nguyen`/`Nugyen` (97) are misses.
- The Name Variant and Acronym dictionaries are unpublished; Glade carries
  the proven pairs only. Two unseen dictionary names that Jaro-Winkler brings
  within 0.87 may match in Glade and not natively.
- Street: four pairs with an unknown suffix abbreviation score asymmetrically
  natively (`123 Main St`/`123 Main Str` 60 one way, 78 the other) and Glade
  does not match them; `8 Oak`/`8 Oak Crt` (78) and `200 Park Ave`/
  `200 Park Av` (81) are misses; `Friedrichstrasse 43`/`Friedrichstraße 43`
  (79, no street number) is a miss. Suite letters (`Unit 3` against
  `Suite B`) match in Glade and not natively.
- Title: the (k, n) table covers up to five words; the one-direction match of
  a single word against a five-word title is not modeled.
- Not captured: standard Account, Contact and Lead rules (their fuzzy
  equations and match keys), cross-object Lead and Contact rules, Person
  Account rules, rules with several fuzzy items under OR (Glade averages the
  matched items' scores), MatchBlanks on Contact fields, Title at API 62.0,
  blank values on the last name and company name. Glade keeps its earlier
  behavior for those rules.
- Unsupported fuzzy rules (unknown method names) are still left unenforced.
