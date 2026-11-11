# Duplicate fuzzy matching: public spec and native run

Fixture: `internal/apextest/testdata/conformance/duplicate_fuzzy_matching/` holds `org.tsv` (native verdicts), `cases.json` (the agreement bounds the
conformance test enforces). The test is `TestFuzzyMatchingNativeRows` in
`internal/dml/fuzzy_conformance_test.go`. The scorers are in
`internal/dml/fuzzy_match.go`.

## Public documentation read (own words)

All pages are Salesforce Help articles.

- "Matching Methods Used in Matching Rules"
  (`help.salesforce.com`, id `sf.matching_rules_matching_methods.htm`). Each
  fuzzy method is tied to a field and takes the highest score of its
  algorithms unless it says otherwise. First Name: Exact, Initials,
  Jaro-Winkler, Name Variant, threshold 85. Last Name: Exact, Keyboard
  Distance, Metaphone 3, threshold 90. Company Name: Acronym, Exact, Syllable
  Alignment, threshold 70; legal designators such as Inc and Corp are removed
  first. Phone: weighted average of country code 10, area code 50, next three
  digits 30, last four 10; threshold 80; North American numbers. City: Edit
  Distance and Exact, threshold 85. Street: weighted average of street name 50
  (edit distance), number 20, suffix 15, suite 15; threshold 80. ZIP: first
  five digits 90, next four 10; threshold 80. Title: Acronym, Exact,
  Kullback-Leibler distance, threshold 50. Exact is recommended for
  international data and works on any field, including custom fields.
- "Matching Algorithms Used with Matching Methods"
  (`sf.matching_rules_matching_algorithms.htm`). Comparison ignores letter
  case. Exact scores 100 or 0. Acronym and Name Variant use hard-coded
  dictionaries (Bob matches Robert; Bob does not match Bill). Edit Distance
  counts insertions, deletions and substitutions. Initials compares initials
  of personal names. Jaro-Winkler suits short strings. Keyboard Distance
  weights edits by key distance. Kullback-Leibler scores shared words.
  Metaphone 3 compares sounds. Syllable Alignment compares syllable
  sequences with edit distance.
- "Standard Contact Matching Rule and Standard Lead Matching Rule"
  (`sales.matching_rules_standard_contact_rule.htm`). Match keys narrow
  candidates, then an equation of field comparisons scores them. Contact
  equation: first, last, title and account name; or first, last and email; or
  first, last, phone and account name; or first, last, mailing street plus city,
  zip or phone; and so on. Lead uses company for account name. Blank values
  do not match, except the phone area code section. Names are also compared
  swapped.
- "Standard Matching Rules" (`sf.matching_rules_standard_rules.htm`) could not
  be read; the page renders client side. The Account rule is not modeled.
- Blank behavior: with Match Blank Fields, two blank values match and one
  blank value does not; without it, blanks never match.

## Capture

- One fresh 30-day Developer Edition scratch org, deleted afterwards.
- Custom matching rules deployed as metadata, one fuzzy item each, plus a
  duplicate rule per matching rule (action Allow, Report operation). Platform
  constraints seen while deploying: at most five active matching rules per
  object, a duplicate rule needs an alert or report operation, and a
  title-only fuzzy rule is rejected ("use at least one other field"), so the
  Title rule is `Title fuzzy AND LastName Exact` with the same last name on
  every probe record.
- Verdicts come from `Datacloud.FindDuplicates.findDuplicates` run in
  anonymous Apex. Each run inserts up to 50 records ("existing") in one
  transaction and, in a second transaction after a pause, runs
  `findDuplicates` on up to 50 unsaved candidates built from the same
  values, so every value is compared with every other value (up to 2,500
  pairs per run). A pair is a native match when the existing record comes
  back in the rule's `MatchResult`; `getMatchConfidence()` is stored as the
  third element. Records are tagged with their index; no record, org or user
  id is stored.
- API 67.0 and 62.0 (`--api-version`). Every row at both versions was
  captured at least twice. Repeats and the two versions agree exactly,
  except Title at 62.0: committed records had not reached the match index
  when the candidates ran (0 and 330 of 437 matches), so no Title row is
  stored for 62.0. The 67.0 Title row repeated exactly.
- Row id is `object.field.method.blankBehavior`. Value JSON:
  `values` (index order, `null` is a blank value, padding blanks included),
  `matches` (`[candidate index, existing index, confidence]`), `const` (fixed
  fields), `blank`, `method`. Rows with id prefix `Holdout.` use values
  chosen after the main fit; they were scored before any tuning (below).
- Match engine reported for custom rules with a fuzzy item:
  `FuzzyMatchEngine` (same name as the standard rules).

## What the native rows show

- NATIVE Phone: sections compared exactly. 10 digits, or 11 digits with a
  leading country digit, are parsed; extra letters (`x5`, `ext`) or other
  lengths are unparsed and never match, not even an identical value. Country
  10, area 50, exchange 30, line 10; a differing or one-sided country code
  scores 90, a differing line 90, both 80, a differing exchange or area drops
  below 80. Slash, dot, space, parenthesis and hyphen are separators.
  Confidence equals the weighted score. Matches the documentation exactly.
- NATIVE Zip: only the segment before the first hyphen or space decides.
  `94105`, `94105-1234` and `94105-1235` match one another; `941051234`,
  `123456` and `12345` do not; `SW1A 1AA` matches `SW1A 1AB`.
  Confidence 100.
- NATIVE City: matches at roughly 87% or higher similarity on a
  longest-length edit distance, not at the documented 85 (single edits in
  words of seven letters or fewer did not match; most single edits in words
  of eight or more did, but `Honolulu`/`Honalulu` did not). Confidence reads `(100 + similarity) / 2`
  for the pairs seen.
- NATIVE Street: same street number and a near-identical name matches.
  Suffix and suite differences do not stop a match (`St`, `Street`, `Ave`,
  `Suite 5` against nothing all match); a changed number, a name edit
  (`Maine`, `Elmwood`), or an extra directional prefix (`N`) does not.
  `Fl`/`Floor` and `Unit` are not suite designators. `PO Box` and `P.O. Box`
  match.
- NATIVE Company: one trailing legal designator and a leading `the` are
  ignored (`Acme`, `Acme Inc`, `Acme Corp`, `The Acme`, `ACME LLC` all match;
  `Acme Corporation Inc` matches none of them). `&` equals `and`; `of` is
  ignored; a hyphen equals a space (`Wal-Mart` matches `Wal Mart`, not
  `Walmart`). Acronyms match only for dictionary entries: IBM, AMD, GE, GM,
  HP and NASA matched their expansions; AA, HPE, DOE, MSFT, JNJ, BofA and
  initials of `Intl Business Machines` did not.
- NATIVE Title: two titles match when they share a word (stems of six or
  more letters, so `Engineer` meets `Engineering`, `Office` meets `Officer`),
  an abbreviation of three or more letters starts a word (`Dir.`, `Exec`), or
  one is the initials of the other (`CEO`, `AE`, `PM`, `CSM`).
- NATIVE Last name: phonetic grouping (`Smith`, `Smyth`, `Smithe`, `Simth`;
  `Schmidt`, `Schmitt`, `Shmidt`; `Johnson`, `Jonson`, `Johnsen`; `Lee`, `Li`,
  `Leigh`), accents kept distinct with a score of 98 to 99 against the plain
  spelling, spaces kept (`O Brien` matches only itself).
- NATIVE First name: Name Variant dictionary groups (Robert/Bob/Rob/Bobby,
  Jon/John/Johnny, Elizabeth/Liz/Beth, William/Bill/Will, ...), Jaro-Winkler
  style scores of 95 to 98 for near spellings (`Johny`/`John` 98,
  `Wiliam`/`William` 98, `Maira`/`Maria` 97), and `Jo` matching `Joseph` and
  `Josef` but not `Jose`. `Michael` does not match `Micheal`, and `Jane` does
  not match `Jayne`, which a plain Jaro-Winkler score would accept.
- NATIVE blanks: with Match Blank Fields two blank values match (confidence
  100); with the default, blanks never match. One blank value never matches.
  Recorded for Account Name, Street and City, and implied for the others by
  the shared engine.
- NATIVE verdicts were identical at API 62.0 and 67.0 for every row captured
  at both versions.

## Glade fit (internal/dml/fuzzy_match.go)

The scorers are conservative: a false match blocks a save that Salesforce
allows, while a missed match only leaves the earlier unenforced behavior. Every
captured row, in-sample and held-out, has zero extra matches. Near-threshold
pairs the approximations cannot separate resolve to no match, which costs
recall.

Recall is the share of native matching pairs Glade also matches; extra is the
count of pairs Glade matches that native did not. API 67.0, all pairs of probe
values including the blank-row pairs of MatchBlanks rows; API 62.0 rows give the
same figures.

In-sample (values the scorers were fitted to):

| Method | Native matches | Glade recall | Extra |
|---|---|---|---|
| First name | 195 | 185 (94.9%) | 0 |
| Last name | 178 | 172 (96.6%) | 0 |
| Phone (Contact) | 189 | 189 (100%) | 0 |
| Phone (Account) | 189 | 189 (100%) | 0 |
| Title | 437 | 437 (100%) | 0 |
| Company name | 163 | 160 (98.2%) | 0 |
| Street | 207 | 207 (100%) | 0 |
| City | 68 | 64 (94.1%) | 0 |
| ZIP | 95 | 95 (100%) | 0 |

Held-out (values chosen after the main fit):

| Method | Native matches | Before tuning: recall / extra | After tuning: recall / extra |
|---|---|---|---|
| Company name | 81 | 70 (86.4%) / 6 | 77 (95.1%) / 0 |
| Street | 147 | 100 (68.0%) / 6 | 135 (91.8%) / 0 |
| City | 103 | 78 (75.7%) / 2 | 97 (94.2%) / 0 |
| Phone | 271 | 244 (90.0%) / 0 | 271 (100%) / 0 |

Only the "before tuning" columns estimate agreement on unseen values: there
Glade produced extra matches (6, 6 and 2) that the conservative changes then
removed after seeing them, so the "after" columns are no longer held out. Dictionary
and threshold choices are fitted to the rows; expect lower recall on other
values, and extra matches cannot be ruled out for unseen pairs.

## Documented gaps

- Name Variant, Acronym and the other dictionaries are not published.
  Glade carries only the groups and acronym pairs the rows prove; every other
  nickname or acronym falls back to Jaro-Winkler or exact and will differ.
- Metaphone 3, Syllable Alignment, Kullback-Leibler and Keyboard Distance are
  approximated (reduced phonetic key; exact or near-equal tokens; shared
  stems; key-distance edit distance). Native rows reject some pairs these
  accept (`Micheal`, `Lakeshore`, `Garcia` with an accent) and accept some
  that need a real syllable model (`Initec`/`Initech`, `Goldman`/`Goldmann`,
  `Industry`/`Industries` beyond the plural rule, `Maria`/`Maira`).
- City threshold: native behaves like about 87 to 88, not the documented 85,
  on the similarity used. Glade uses 88 so the 87.5 pair that native rejects
  stays a no match; single edits in eight-letter words lose recall.
- `matchConfidence` is not modeled below 100 (Glade still reports 100). The
  rows record native confidences (Phone and ZIP follow the documented weights;
  City reads `(100 + similarity) / 2`; Title and Street do not follow a
  formula that was found).
- Not captured: standard Account, Contact and Lead rules (their fuzzy
  equations and match keys), cross-object Lead and Contact rules, Person
  Account rules, rules with several fuzzy items under OR, MatchBlanks on
  Contact fields (the Contact matching rules would not finish activating in
  the run), Title at API 62.0, and blank values on the last name and company
  name (the object requires them). Glade keeps its earlier behavior for those
  rules.
- Unsupported fuzzy rules (unknown method names) are still left unenforced.
