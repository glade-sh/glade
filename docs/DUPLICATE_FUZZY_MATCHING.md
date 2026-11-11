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

- Three fresh 30-day Developer Edition scratch orgs, each deleted afterwards.
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
  `Holdout2.*` for the second run's held-out rows, chosen before the fit. The
  third run holds `Probe3.*` (hypothesis probes for the street abbreviation
  table, company word variants, phone country digits, title prefix words,
  first-name Jaro-Winkler fall-through and last-name keyboard families) and
  `Holdout3.*`, CRM-shaped pairs that no scorer was tuned on. Holdout2 was
  tuned on after it was first scored, so only Holdout3 is an unbiased
  estimate.
- API 67.0 for every row; API 62.0 confirmation rows for the first run's rows,
  six probe rows of the second run and one probe row per method of the third
  (eight rows). Every 62.0 row equals its 67.0 row
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
  76 does not). This is an approximation: native also matches pairs with an
  unknown abbreviation at confidences of 50 to 80 that S cannot produce, and
  the third run shows no name-similarity guard: `Pennsylvania`/`Pennsylxxxia`
  (name similarity 75, S 85) matches. `Crt` behaves as a suffix word of its
  own (`8 Pine Crt` matches `8 Pine` at 80 and does not match `8 Pine Court`);
  `Ct`, `Court`, `Ter`, `Pl` and `Sq` stay in the name. Glade rejects a pair
  whose last name word and name stem both differ (`Harbor Lp`/`Harbour Loop`
  does not match natively) and, with no street number on either side, a name
  similarity under 80 (`Calle 8 No 5`/`Calle 8 Numero 5`). A single letter
  after the number is not only `E`: natively `100 A`, `I`, `O` and `Y Main St`
  match `100 Main St` at 83 and `De`, `La`, `Et`, `And`, `The` do not; Glade
  drops `E` only. Confidence `(S + edit similarity of the cleaned strings) / 2`:
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
  doubled letters (`Goldmann Sachs`) and the word pairs listed in
  `companyWordPairs` (`Northwinds Traders`, `Hewlett Packards`, `Verizone`,
  `Industries`/`Industrie`/`Industri`, `Kinetik`/`Kinetick`). The third run
  shows these are not rules: `Guitars`, `Markets` and `Studios` match their
  singular but `Partners`/`Partner`, `Robots`, `Towers`, `Lasers` and `Motors`
  do not; `Verizon`/`Verizone` matches but `Marine`/`Marin` and `Grande`/
  `Grand` do not; `Horizon` matches none of `Horizan`, `Horizun` and
  `Horizen` although `Momentum` matches `Momentem` and `Momentom`. Glade
  therefore carries only the pairs the rows prove; one extra or different word after the
  first two scores 91 (`Hewlett Packard Enterprise`, `International Business
  Systems`, `Blue Yonder Airlines`/`Blue Yonder`); two extra words do not
  match (`Blue Yonder Air Lines`/`Blue Yonder`). Vowel changes score 96 to 98 for
  some words (`Nordstram`, `Caterpiller`, `Prosewear`) and nothing for others
  (`Verizen`, `Veryzon`, `Stanlie`, `Advisers`, `Horizan`); a consonant change
  or a missing vowel mostly does not match (`Microsft`, `Verison`, `Morgan
  Stanly`; `Starbux` 94 and `Initeck` 95 do). A hyphenated name matches the
  same words spelled with a space and also the spaced name plus a designator
  (`Coca-Cola`/`Coca Cola Company` match, 100 or 97 by direction). Acronym
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
  `Matthew`/`Matt`, `Tamsin`/`Tasmin`); Glade blocks the proven pairs and any
  two names that both appear in a proven variant pair unless the rows show
  that pair matching (`nameJaroPairs`: `Alexander`/`Alexandra`, `Tamsin`/
  `Tamsinel`). The third run confirms the Jaro-Winkler fall-through for names
  the dictionary does not pair (`Paul`/`Paula`, `Eric`/`Erica`, `Carl`/`Carla`,
  `Mario`/`Maria`, `Dan`/`Dana` all match natively at their Jaro-Winkler
  confidence); the rows prove no other form of the "both known" rule. `J` matches no
  longer name in a first-name-only rule.
- NATIVE Last Name: match key = four-character phonetic code of the
  accent-folded name (`Williams`/`Williamson` compared, `Smith`/`Smiht` never
  compared); equal full codes score 100 (`Taylor`/`Tayler`/`Tailor`/`Taylr`,
  `Clark`/`Klark`/`Clarck`, `Meyer`/`Meier`/`Mayer`, `Reid`/`Reed`/`Read`,
  `Wright`/`Right`/`Rite`, `Müller`/`Miller`; `Thompson` and `Thomson` do not match each other,
  though each matches a neighbour: `Thompson`/`Thompsen`, `Thomson`/`Tomson`). Codes
  keep D distinct from T (`Carter`/`Carder` 99, `Peterson`/`Pederson` 99) and B
  from P (`Campbell`/`Cambell` no match), treat accented letters as neither
  vowel nor consonant for the context rules (`García` against `Garcia` 99,
  against `Gracia` no match), keep H between vowels (`Cohen` against `Coen` no
  match) and keep a doubled L before a consonant (`Walker`/`Wallker` no match). The
  apostrophe-removed form matches (`O'Connor`/`OConnor`) but a space does not
  (`O'Brien`/`O Brien`). A keyboard substitution of `r` for the vowel before
  a double L never matches (`Campbrll`, `Kimbrll`, `Russrll`, `Mitchrll`,
  `Powrll`) while `w` does (`Kimbwll`, `Russwll`, `Mitchwll`); Glade's
  two-vowel-group guard produces that split for these names.
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
  `II`/`III`, `Pr`/`Product` and `Principal`), except `IT`, which does not
  equal `Italy` (`IT Manager`/`Italy Manager` is 93, one shared word), but two single-word titles need an exact match (`Director`/
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
- NATIVE Phone: sections compared exactly; a country digit other than 1 never
  matches the same number with a different or no country digit (`2 415 555
  0100` matches only itself); 10 digits, or 11 with a leading
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
the pairs both match. API 67.0 rows, every ordered pair of every row. Before
is the scorer set at 02bab08ab; After is this change. Fit rows are the first
two runs. Holdout2 was scored once and then tuned on, so its figures are
tuned-on, not an estimate. Probe3 is the third run's probe set, which the
change was fitted to. Holdout3 is the only unbiased set: nothing was tuned on
it.

| Method | Set | Before: recall, extra, conf MAE | After: recall, extra, conf MAE |
|---|---|---|---|
| City | Fit rows, runs 1-2 | 334/334 (100.0%), 0, 0.000 | 334/334 (100.0%), 0, 0.000 |
| City | Holdout2 (tuned on) | 80/80 (100.0%), 0, 0.000 | 80/80 (100.0%), 0, 0.000 |
| City | Probe3 (fitted to) | 38/38 (100.0%), 0, 0.000 | 38/38 (100.0%), 0, 0.000 |
| City | Holdout3 (unbiased) | 79/79 (100.0%), 0, 0.000 | 79/79 (100.0%), 0, 0.000 |
| Street | Fit rows, runs 1-2 | 1648/1728 (95.4%), 0, 0.148 | 1676/1728 (97.0%), 0, 0.112 |
| Street | Holdout2 (tuned on) | 148/164 (90.2%), 0, 0.770 | 148/164 (90.2%), 0, 0.770 |
| Street | Probe3 (fitted to) | 68/162 (42.0%), 2, 0.676 | 80/162 (49.4%), 0, 0.550 |
| Street | Holdout3 (unbiased) | 105/125 (84.0%), 0, 0.686 | 107/125 (85.6%), 0, 0.673 |
| Company | Fit rows, runs 1-2 | 1135/1193 (95.1%), 0, 0.032 | 1177/1193 (98.7%), 0, 0.006 |
| Company | Holdout2 (tuned on) | 114/118 (96.6%), 0, 0.018 | 114/118 (96.6%), 0, 0.000 |
| Company | Probe3 (fitted to) | 91/112 (81.2%), 12, 0.374 | 101/112 (90.2%), 0, 0.238 |
| Company | Holdout3 (unbiased) | 95/105 (90.5%), 0, 0.242 | 87/105 (82.9%), 0, 0.678 |
| First Name | Fit rows, runs 1-2 | 936/950 (98.5%), 0, 0.000 | 950/950 (100.0%), 0, 0.000 |
| First Name | Holdout2 (tuned on) | 164/170 (96.5%), 0, 0.000 | 170/170 (100.0%), 0, 0.000 |
| First Name | Probe3 (fitted to) | 134/154 (87.0%), 8, 0.836 | 154/154 (100.0%), 0, 0.013 |
| First Name | Holdout3 (unbiased) | 110/116 (94.8%), 5, 0.600 | 110/116 (94.8%), 5, 0.600 |
| Last Name | Fit rows, runs 1-2 | 696/748 (93.0%), 0, 0.046 | 696/748 (93.0%), 0, 0.046 |
| Last Name | Holdout2 (tuned on) | 166/170 (97.6%), 0, 0.133 | 166/170 (97.6%), 0, 0.133 |
| Last Name | Probe3 (fitted to) | 78/80 (97.5%), 0, 0.000 | 78/80 (97.5%), 0, 0.000 |
| Last Name | Holdout3 (unbiased) | 99/101 (98.0%), 0, 0.000 | 99/101 (98.0%), 0, 0.000 |
| Title | Fit rows, runs 1-2 | 939/942 (99.7%), 0, 0.246 | 939/942 (99.7%), 0, 0.246 |
| Title | Holdout2 (tuned on) | 286/286 (100.0%), 0, 0.045 | 286/286 (100.0%), 0, 0.045 |
| Title | Probe3 (fitted to) | 111/113 (98.2%), 0, 0.649 | 111/113 (98.2%), 0, 0.523 |
| Title | Holdout3 (unbiased) | 174/174 (100.0%), 0, 0.121 | 174/174 (100.0%), 0, 0.132 |
| Phone | Fit rows, runs 1-2 | 649/649 (100.0%), 0, 0.000 | 649/649 (100.0%), 0, 0.000 |
| Phone | Probe3 (fitted to) | 23/23 (100.0%), 8, 0.000 | 23/23 (100.0%), 0, 0.000 |
| Phone | Holdout3 (unbiased) | 164/200 (82.0%), 16, 0.000 | 164/200 (82.0%), 0, 0.000 |
| ZIP | Fit rows, runs 1-2 | 95/95 (100.0%), 0, 0.000 | 95/95 (100.0%), 0, 0.000 |
| ZIP | Probe3 (fitted to) | 17/17 (100.0%), 0, 0.000 | 17/17 (100.0%), 0, 0.000 |
| ZIP | Holdout3 (unbiased) | 113/113 (100.0%), 6, 0.000 | 113/113 (100.0%), 0, 0.000 |

Holdout3 was measured once after the scorers were fixed on Probe3 and the
earlier rows alone: 933/1,013 native pairs matched and 11 extra (First Name 5,
ZIP 6). The ZIP extras were values under three characters (`0`, `L1`) that
native does not even match to themselves; ZIP now rejects any value shorter
than three characters. That rule was chosen after seeing those rows, so the
ZIP After figure and the total of 5 extra are tuned-on; the Holdout3 numbers
before that rule are 933/1,013 with 11 extra. The five First Name extras
(`Jose`/`José`, `Rachael`/`Rachel`, `Rose`/`Rosa` one way) are dictionary names
Glade does not know; no rule separates them from `Paul`/`Paula`, which
matches natively, so they remain and are not tuned on.

The conservative company tables cost recall on unseen words: Company Holdout3
recall is 82.9% where the generalizing rules had 90.5%, with no extra pair in
either, and the probes show the generalizations (`Partners`, `Marine`,
`Horizan`) did produce false matches. Where the approximation could not
separate a pair that native accepts in one direction only (`Nordstrom`/
`Nordstroms`, `Initech`/`Initechs`, `Starbuck`/`Starbucks`) or rejects without
a visible cause (`Campbell`/`Campbrll`, `Oak Court`/`Oak Crt`, `Hewlett-Packard`
against its acronym), Glade does not match the pair, so those are misses and
not extras. Street suites parse a trailing letter or a number with letters as
a suite, a designator word outside the proven list (`Fl`, `Floor`) stays in
the name, and a separated number letter (`100 B`) does not equal an attached
one (`100B`).

## Gaps

- Syllable Alignment, Metaphone 3 and the Kullback-Leibler scoring are
  approximated by the row-fitted rules above. Native scores some same-key
  company pairs in one direction only (`Initec`/`Initech` 93, `Salesforce`/
  `Salesforcecom` 90, `Nordstrom`/`Nordstroms` 100) and Glade is symmetric.
  `Starbucks`/`Starbuck` (93), `Nordstrom`/`Nordstroms` and `Initech`/`Initechs` (one direction), `Starbux` (95), `Lockheed`/`Lockhead` (100),
  `Hughes`/`Huse` (100) and `Nguyen`/`Nugyen` (97) are misses.
- The Name Variant and Acronym dictionaries are unpublished; Glade carries
  the proven pairs only. Two unseen dictionary names that Jaro-Winkler brings
  within 0.87 may match in Glade and not natively.
- Street: four pairs with an unknown suffix abbreviation score asymmetrically
  natively (`123 Main St`/`123 Main Str` 60 one way, 78 the other) and Glade
  does not match them; `8 Oak`/`8 Oak Crt` (78) and `200 Park Ave`/
  `200 Park Av` (81) are misses; `Friedrichstrasse 43`/`Friedrichstraße 43`
  (79, no street number) is a miss.
- Phone: native ignores a letter inside the digits (`312 555 O199` matches
  `312-555-0199` at 90, 36 Holdout3 misses); Glade rejects any letter.
- Company: Glade does not match an acronym of a multi-word name that native
  does (`BMW`, `KFC`, `UPS` against their expansions).
- Title: the (k, n) table covers up to five words; the one-direction match of
  a single word against a five-word title is not modeled.
- Not captured: standard Account, Contact and Lead rules (their fuzzy
  equations and match keys), cross-object Lead and Contact rules, Person
  Account rules, rules with several fuzzy items under OR (Glade averages the
  matched items' scores), MatchBlanks on Contact fields, Title at API 62.0,
  blank values on the last name and company name. Glade keeps its earlier
  behavior for those rules.
- Unsupported fuzzy rules (unknown method names) are still left unenforced.
