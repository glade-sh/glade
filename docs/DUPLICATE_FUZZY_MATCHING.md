# Duplicate fuzzy matching: public spec, native rows and the model

Three fixtures hold the native rows. The Apex save and search routes are
covered by `duplicate_save_behavior/`, including default and custom rule
actions, error members, Person Accounts, upsert, and controlled surname panels. `duplicate_standard_rules/` (second fixture,
described under "Standard rules" below) covers the standard Account, Contact
and Lead rules, the cross-object rules, custom rules with OR groups and three
items, a custom Person Account rule and the Syllable Alignment pairs; its test
is `TestStandardRulesNativeRows` (`internal/dml/standard_rules_conformance_test.go`).

Fixture: `internal/apextest/testdata/conformance/duplicate_fuzzy_matching/`
holds `org.tsv` (native verdicts with match confidences), `cases.json` (the
agreement bounds the conformance test enforces) and `RUN-FACTS.txt`. The tests
are `TestFuzzyMatchingNativeRows` (`internal/dml/fuzzy_conformance_test.go`),
`TestFuzzyScoreNativeConfidences` (`fuzzy_confidence_test.go`) and
`TestFuzzyNameTablesMatchNativeRows` (`fuzzy_tables_test.go`). The scorers are
in `internal/dml/fuzzy_match.go`, the phonetic key and Jaro-Winkler in
`fuzzy_metaphone.go`, the row-proven dictionaries in `fuzzy_tables.go`.

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
  are never compared, so the algorithms never see them. First names are
  replaced by an alias where one applies; last names are reduced to a double
  metaphone code. The notation gives the words and characters per word a key
  keeps: the custom-rule example is `First_Name (1,1) Last_Name Company (2,5)`.
  No match key is generated for Title.
- "Standard Contact Matching Rule and Standard Lead Matching Rule"
  (`sales.matching_rules_standard_contact_rule.htm`). Match key examples:
  Last Name as its double metaphone code ("Doe" to "t", "Benioff" to "pnf"),
  Company as the first two words of five characters ("salesforce.com" to
  "sales"), Phone as the number without the leading 1 and the last four
  digits. The equations and per-field criteria of the standard rules.

Double Metaphone and Jaro-Winkler are public algorithms; Glade implements both
from their published descriptions. Metaphone 3 is the Go port
`github.com/dlclark/metaphone3` (BSD 3-Clause, from Lawrence Philips's
published reference code; the release bundle carries its LICENSE and ATTRIB
notices). Syllable Alignment is named by the help pages but Glade does not
carry it (see Gaps).

## Capture

- Six fresh 30-day Developer Edition scratch orgs, each deleted afterwards.
  Custom matching rules with one fuzzy item each (plus an exact Last Name item
  for Title, which the platform requires), a duplicate rule per matching rule,
  and one rule with two fuzzy items (Company Name and City). An object takes
  four active custom matching rules.
- Verdicts come from `Datacloud.FindDuplicates.findDuplicates` in anonymous
  Apex: up to 50 records inserted in one transaction, then every value run as
  an unsaved candidate against them (up to 2,500 pairs per row). A pair is a
  native match when the existing record comes back in the rule's
  `MatchResult`; `getMatchConfidence()` is the third element of each match.
  Records are tagged by index; no record, org or user id is stored. A value
  whose record never matched itself was not indexed in that run and is not
  used as an existing record.
- Row ids: `object.field.method.blankBehavior` for the first run,
  `Holdout.*`, `Probe.*`, `Holdout2.*`, `Probe3.*` and `Holdout3.*` for the
  second and third runs. The fourth run adds `P4.*` (designed probes: match
  key, Jaro-Winkler variant, dictionary names against typos, name formats,
  keyboard costs per key pair, trailing letters, ampersands and acronyms, the
  two-item rule), `N4.FN.Typo.*` (about 2,700 common first names, each with
  two typos that keep its match key), `N4.FN.Family.*` (about 590 nickname
  families), `N4.LN.Var.*` (about 1,400 common surnames, each with up to two
  respellings that keep its match key), and the held-out rows `Holdout4.*`
  and `Holdout5.*`, written without sight of the model and scored once.
- API 67.0 for every row; API 62.0 confirmation rows for the first run, six
  probe rows of the second run, eight of the third and the fourth run's probe
  rows listed in `RUN-FACTS.txt`. Every 62.0 row equals its 67.0 row pair for
  pair, confidence for confidence.
- The match engine reported for custom rules with a fuzzy item is
  `FuzzyMatchEngine`.

## What the native rows show (recovered model)

`getMatchConfidence()` is a whole number. A rule's confidence is the mean of
its items' confidences (NATIVE `P4.MULTI.NameCity`: Company 91 with City 94
gives 92, Company 100 with City 94 gives 97, Company 91 with City 100 gives
95). An exact or blank match is an item confidence of 100. A "Maximum" method
(First Name, Last Name, Company, City, Title) has item confidence
`(100 + algorithm score) / 2`; a weighted method (Phone, Street, ZIP) reports
its weighted score.

### First Name

NATIVE `P4.FN.*`, `N4.FN.*`: about 50,000 native matches among the pairs of
those rows, every verdict and confidence reproduced by the four rules below
with the tables.

1. Name Variant. If the existing name is in the candidate's dictionary entry
   the score is 100. The dictionary is directional (`Rosa` finds `Rose`,
   `Rose` does not find `Rosa`; `Andre` finds `Andrea` at 100, `Andrea` finds
   `Andre` at 99 through Jaro-Winkler) and is a graph, not a set of groups
   (`Jon` finds `John` and `Jonathan`, which do not find each other). It is
   large and holds spelling variants as well as nicknames (`Katee`/`Katie`,
   `Michal`/`Micheal`, `Andres`/`Andrés`); it is not published. Glade carries
   the ordered pairs the rows prove (`nameVariantTable`).
2. Match key. Otherwise the two names are compared only when the first two
   characters of their Double Metaphone codes agree. Invented names show the
   key without the dictionary: `Vorkel` finds `Vorpel` (FRKL, FRPL) and
   `Vorkelina` finds `Forkelina`, but not `Volkel` (FL); `Zathor` does not
   find `Zator` (S0, ST); `Zeo` does not find `Zeon` (S, SN); `Zovlan` does
   not find `Zoblan`. The code is the common Java rendering of Double
   Metaphone: `Jose` alone codes HS, so it finds neither `Josef` nor `Josee`;
   `Michael` (MK) does not find `Micheal` (MX), `Rachael` (RK) not `Rachel`
   (RX), `Matt` (MT) not `Matthew` (M0), `Nicholas` (NX) not `Nicolas` (NK).
3. Alias keys. A dictionary name is keyed by its alias, not by its own
   spelling: `Kate` (keyed like `Katherine`) finds neither `Katie` nor the
   typos `Katte` and `Kaate`, although those find each other; `Leo` finds
   `Leona` under the key of `Leon`; `Louie` finds `Louise` under the key of
   `Louis`. `Roberta`, `Jane`, `Tess`, `Tony`, `Becky`, `Nick`, `Rick`,
   `Bill`, `Beth` and others find nothing under their own key. Glade lists the names
   the rows show (`nameAliasKeys`, and `nameKeptApart` for pairs such as
   `Helen`/`Helena` where the rows do not say which name is alias-keyed); every other name, including every name
   outside the captured lists, is keyed by its own spelling.
4. Jaro-Winkler. With equal keys the pair matches when Jaro-Winkler is at
   least 0.89 (`Daniel`/`Diane` 0.89 matches at 95, `Ravi`/`Ravindraa` 0.8889
   does not), confidence `(100 + 100 * jw) / 2`. The variant: Jaro with the
   transposition count halved by integer division, the whole common prefix
   counted, a prefix scale of the smaller of 0.1 and one over the longer
   length, applied from a Jaro value of 0.7 (`Bartholomewsonxy` against
   `Bartholoxewsonxy` is 97, against `Bartholomewsonxz` 100). Accents, spaces,
   hyphens and dots are ordinary characters (`Renee`/`Renée` 95,
   `Jean-Paul`/`Jean Paul` 98, `Mary Ann`/`Mary` 95, `Jane.`/`Jane` 98);
   salutations are not removed (`Dr. Jane` finds nothing).

Not modeled: the Initials algorithm (`J`, `J.` and `J R` find each other at
100; `J` finds no longer name in a first-name-only rule), and the punctuation
folding of the dictionary lookup (`Jane.` is keyed like `Jane`).

### Last Name

NATIVE `P4.LN.*`, `N4.LN.Var.*` and the earlier rows.

1. Match key: the Double Metaphone code of the accent-folded name, four
   characters at most. `Frost`/`Frosr` (FRST, FRSR), `Smith`/`Smiht`,
   `Clark`/`Clsrk` and `Kowalski`/`Kovalski` are never compared;
   `Martinez`/`Martinpz` (MRTN) and `Williams`/`Williamson` are. Every native
   match in the rows has equal keys.
2. Metaphone 3. Names with equal Metaphone 3 codes score 100. The code keeps
   B apart from P, D from T and G from K (`Carter`/`Carder` 99 and
   `Reid`/`Right` no match, `Campbell`/`Cambell` no match), ignores vowels
   after the first letter (`Meyer`, `Meier`, `Mayer`, `Maier`, `Myer`), and
   has context rules a Double Metaphone code does not (`Walker` does not find
   `Wallker`, `Cohen` finds `Kohen` only at 96, `Lach` does not find `Leach`,
   `Girard` does not find `Girrard`, `McKnight` does not find `Mknight`).
   Glade encodes both names with the port in exact mode (B, D and G stay
   apart from P, T and K), vowels after the first letter not encoded, and its
   default code length of eight; of the option combinations tried, this one
   has no false match on the rows and the highest recall. The primary codes
   are compared. The code does not see a space or a hyphen, but natively
   `Smith Jones`, `De La Cruz` and `Mc Donald` do not find `Smith-Jones`,
   `Delacruz` and `McDonald`, so two names match only when their separators
   agree. This replaced a hand-written stand-in that rejected anything it
   could not prove and missed dropped vowels, silent letters and name codes.
3. Keyboard Distance, decided at 90, confidence `(100 + score) / 2`. Strings
   are compared position by position; the cost of two different keys is 0.05
   for neighbours and 0.08 for each further step, where the distance is the
   column distance plus two between the top and bottom rows (`P4.LN.KB1`: 25
   replacement letters on two bases); an accent costs 0.16; a trailing extra
   letter about 0.25; the sum is divided by the longer length. The rows do
   not pin the score down for three or more substituted letters, an inserted
   letter or more than two trailing letters (`Martinez` finds
   `Martinezkmptr` at 95 but `Frank` does not find `Franklin`), so Glade
   matches by keyboard only one substitution, two with a score of 92 or
   more, or up to two trailing letters.

The apostrophe-removed form matches (`O'Connor`/`OConnor`), a space or
hyphen does not (`O'Brien`/`O Brien`, `Mc Donald`/`McDonald`).

### Company Name

Lower case, a free-standing `&` is `and`, a hyphen is a space, punctuation is
removed (`Salesforce.com` = `Salesforcecom`), a leading `the`, `of`/`and`/`the`
and one trailing legal designator are dropped (Inc, Incorporated, Corp,
Corporation, Co, Company, Ltd, Limited, LLC, LLP, LP, PLC, GmbH, AG, SA,
Enterprises, NA). NATIVE `P4.CO.Amp`: an `&` inside a word is not `and`
(`AT&T` does not find `AT and T`, `Barnes&Noble` does not find
`Barnes & Noble`), and the Acronym algorithm is a dictionary, not initials
(`ZQW` does not find `Zorblat Quantum Widgets`; `PGE`, `BMW`, `KFC`, `UPS`,
`IBM`, `AMD`, `GE`, `GM`, `HP` and `NASA` find their expansions). Match key =
first five characters of each of the first two words. With the same key:
identical words score 100, as do words that differ only by doubled letters
and the word pairs in `companyWordPairs`; one extra or different third word
scores 91; one extra fourth word after three equal words scores 93; two extra
words do not match. Syllable Alignment itself is not recovered: native
scores some single-word pairs in one direction only (`Nordstrom` finds
`Nordstroms`, not the reverse) and matches word pairs Glade does not
(`Lakeside Hospital`/`Lakeside Hospitality` 100,
`Acme Logistic`/`Acme Logistics` 100) while keeping others apart
(`Partners`/`Partner`, `Marine`/`Marin`).

### Phone

Letters and digits are the characters of the number; separators are dropped.
Ten characters, or eleven with the country digit first, parse into country,
area, exchange and line; other lengths never match, not even themselves.
Sections are compared exactly with weights 10, 50, 30, 10; the confidence is
the weighted score; a match needs 80. A letter typed for a digit therefore
still matches (`312 555 O199` finds `312-555-0199` and `312-555-0190` at 90).
A country digit other than 1 never matches the same number with a different
or no country digit.

### City, Street, ZIP, Title

Unchanged from the third run's model.

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
- NATIVE Zip: only the segment before the first hyphen or space decides;
  confidence 100; a value under three characters matches nothing.

NATIVE blanks: with Match Blank Fields two blank values match (100); with the
default they never do; one blank value never matches.

## Glade fit

Recall is the share of native matching pairs Glade also matches; false matches
are pairs Glade matches that native does not; confidence MAE is the mean
absolute difference between Glade's rounded confidence and the native one over
the pairs both match. API 67.0 rows, every ordered pair of every row. "fit"
is every row the model or its tables were built from (runs one to three, `P4.*`
and `N4.*`). The held-out rows were written without sight of the model and
scored once: `Holdout4.*` for Street, City, ZIP, Phone and Title,
`Holdout4.*` and `Holdout6.*` for First Name, `Holdout6.*` for Last Name and
`Holdout5.*` for Company. Last Name and Company were first scored on
`Holdout4.*`, which showed one false pair each (`Smith-Jones`/`Smith Jones`;
`AT&T`/`AT and T`); one rule each was changed and a fresh held-out set drawn.

| Method | Set | Recall | False matches | Confidence MAE |
|---|---|---|---|---|
| First Name | fit | 49935/49935 (100.0%) | 0 | 0.000 |
| First Name | holdout (unbiased) | 460/478 (96.2%) | 0 | 0.200 |
| Last Name | fit | 17010/17600 (96.6%) | 0 | 0.012 |
| Last Name | holdout seen once, then one rule changed | 171/173 (98.8%) | 0 | 0.000 |
| Last Name | holdout (unbiased) | 199/201 (99.0%) | 0 | 0.010 |
| Company | fit | 1619/1665 (97.2%) | 0 | 0.083 |
| Company | holdout seen once, then one rule changed | 145/149 (97.3%) | 0 | 0.159 |
| Company | holdout (unbiased) | 162/164 (98.8%) | 0 | 0.117 |
| Street | fit | 2011/2179 (92.3%) | 0 | 0.208 |
| Street | holdout (unbiased) | 151/151 (100.0%) | 0 | 0.318 |
| City | fit | 531/531 (100.0%) | 0 | 0.000 |
| City | holdout (unbiased) | 57/57 (100.0%) | 0 | 0.000 |
| ZIP | fit | 225/225 (100.0%) | 0 | 0.000 |
| ZIP | holdout (unbiased) | 62/62 (100.0%) | 0 | 0.000 |
| Phone | fit | 872/872 (100.0%) | 0 | 0.000 |
| Phone | holdout (unbiased) | 81/81 (100.0%) | 0 | 0.000 |
| Title | fit | 1510/1515 (99.7%) | 0 | 0.215 |
| Title | holdout (unbiased) | 467/479 (97.5%) | 0 | 0.173 |

The earlier bounds for the Last Name rows of the first three runs were lower
than before the Metaphone 3 change. An earlier phonetic code reproduced those rows but
matched 670 pairs of the surname rows and 2,854 pairs of the first-name rows
that Salesforce does not match; the rule set above matches none of them.

Before the Metaphone 3 dependency the Last Name rows were fit 12918/17600
(73.4%, MAE 0.165), the seen-once holdout 161/173 (93.1%, 0.112) and the
unbiased holdout 193/201 (96.0%, 0.041), all with 0 false matches. The
`cases.json` bounds for every Last Name row were tightened to the new values.

## Gaps

- Last Name: Glade does not reproduce the 2 of 201 unbiased-holdout, 2 of 173
  seen-once and about 590 of the fit pairs that native matches and the port
  does not. They are Keyboard Distance matches of three or more edits, long
  names the key cuts differently (`Kowalczyk`/`Kowalsky`), respellings the
  port codes apart (`Robards`/`Robertz`, `Waalker`/`Walkker`) and a
  hyphenated surname against its first part (`Fitzgerald`/`Fitzgerald-Lee`).
- Keyboard Distance: the cost of trailing letters depends on the letters
  (`Williams`/`Williamson` 98, `Stevenson`/`Stevensonnn` 97), which the flat
  0.25 misses by one point; three or more substitutions, insertions and more
  than two trailing letters are not matched.
- First Name: the Name Variant dictionary and its alias keys are unpublished.
  Glade carries the pairs and alias-keyed names proven for about 2,700 common
  first names and 590 nickname families. A nickname pair outside those rows
  is a miss. A dictionary name outside those rows whose alias key differs
  from its own key can match a near spelling by Jaro-Winkler in Glade and not
  natively. Initials (`J`, `J.`, `J R`) are not matched.
- Syllable Alignment is modeled only where the second fixture proves a rule
  (see "Syllable Alignment"). Native confidences of one-word plurals vary with
  the word beyond what the length and last letter explain; vowel variants, an
  `&` inside a word against the joined spelling (`AT&T` finds `ATT` one way)
  and acronyms outside the proven entries are misses.
- Street: pairs with an unknown suffix abbreviation score asymmetrically
  natively (`123 Main St`/`123 Main Str` 60 one way, 78 the other) and Glade
  does not match them; single letters after the number other than `E`.
- Title: the (k, n) table covers up to five words; the one-direction match of
  a single word against a five-word title is not modeled.
- Not captured: MatchBlanks on Contact fields, blank values on the last name
  and company name. The standard rules, the cross-object rules, OR groups and
  three-item rules are in the second fixture; its gaps are listed under
  "Standard rules".
- Unsupported fuzzy rules (unknown method names) are still left unenforced.

## Standard rules

Fixture: `internal/apextest/testdata/conformance/duplicate_standard_rules/`
(`org.tsv`, `cases.json`, `RUN-FACTS.txt`); test `TestStandardRulesNativeRows`;
scorers in `internal/dml/standard_rules.go`, `duplicate_filter_confidence.go`
and `fuzzy_raw.go`. Verdicts come from `Datacloud.FindDuplicates` in
anonymous Apex against three fresh 1-day scratch orgs, at API 67.0 and 62.0;
the two versions agree on every row they share. The public help names the
standard rules but not their equations (the article on the standard Contact and
Lead rules gives only match-key examples), so the structure below is read
from native truth tables over every combination of same, different and blank
values for the fields involved.

Every field comparison has five outcomes: the values match, differ, the
candidate is blank, the existing record is blank, or both are. Only a match
satisfies a field. Two blank values never satisfy a field.

### Account

`Standard_Account_Match_Rule_v1_0` matches when the name and a city or ZIP
code match, or the website, a city or ZIP code, and a street or phone number
match (NATIVE `Standard.Account.Truth`, 729 states of the six fields, and
`Standard.Account.Blanks`, 7,290 states with blank existing values). The
confidence is the product of the items on the path: name times the better of
city and ZIP on the Name path (`Standard.Account.NameConfidence`: name 93 and
city 95 report 89), website times the better of street and phone on the
Website path, the city or ZIP gating that path without scoring it
(`Standard.Account.WebsitePath`). The standard items are the fuzzy methods
with their own scales: ZIP is five digits with an optional hyphenated
extension (`94105` finds `94105-1234` at 100, two different extensions score
95, `94105 1234` with a space is not an extension, non-US codes match only
themselves), the website drops case, the scheme, a leading `www.` and a lone
trailing slash (`acme.com/` does not find `acme.com`; `a-cme.com` finds
`acme.com` at 94), phone numbers differing in the last four digits score 94 for
an Account, 97 for a Contact and 98 for a Lead, and a letter typed for a
digit finds nothing.

The same structure holds with the state and country equal on both records
and with them blank (`Standard.Account.Truth` kept them equal,
`Standard.Account.NoStateCountry` leaves them out). Streets with a number sign
(`210 Elm St #4`) match only their own spelling, and a seven digit phone
number does not find the full number: the fit rows showed it, the holdout
rows did not.

### Contact and Lead

`Standard_Contact_Match_Rule_v1_1` and `Standard_Lead_Match_Rule_v1_0` share
one structure (NATIVE `Standard.Contact.Truth.*`, `Standard.Lead.Truth.*`,
`Standard.*.Blanks`, `Standard.*.CompanyPath*`, `Standard.*.AccountPath*`):
the last name matches, and either

- the first names match and the email address, the phone number, or the
  company branch matches; or
- the candidate has no first name, or both have none, and the email address
  matches.

An existing record without a first name never matches a candidate that has
one. The company branch needs the same company (a Lead's company text, a
Contact's account) or two email addresses at one domain, and then the same
title, or the same street with the same city or ZIP code. Two addresses at one
domain count whatever the domain (gmail.com included, `Standard.*.EmailDomain`).
Contacts compare accounts by id; a Lead candidate's company text never
equals a Contact's account. The confidence is the product of the last name,
first name and path items.

### Cross-object rules

`Standard_Rule_for_Contacts_with_Duplicate_Leads` and
`Standard_Rule_for_Leads_with_Duplicate_Contacts` apply the other object's
rule to the record's own fields (a Contact's account becomes the Lead's
company). NATIVE `Standard.Cross.ContactToLead` and `Standard.Cross.LeadToContact`:
the structure is the Lead rule in the first direction and the Contact rule in
the second, and a Lead candidate's company never equals a Contact's account.
Glade enforces them when the rule is active.

### Custom rules: OR groups and three items

NATIVE `Custom.*`: a rule's confidence is the mean over its filter. An AND
group and an OR group each report the mean of their children, a leaf is its
item's confidence, and a leaf that did not match still counts with the score
its algorithm gave the pair. `(1 AND 2) OR 3` with the first and last name
matching and a phone number sharing only the exchange reports
`(100 + 40) / 2 = 70`; the phone score is the weighted score (country 10, area
50, exchange 30, line 10, blank countries agreeing). An Exact item that
differs counts 50. Three AND items report the mean of three. Raw scores for
Company Name and Title are not recovered and count 50.

### Person Account

NATIVE `Custom.PersonAccount`: a custom matching rule on `PersonAccount`
compares person accounts with each other and a person account is checked only
against Person Account rules. The standard Person Account matching rule ships
inactive and cannot be activated by metadata deploy, so no row covers it and
Glade leaves it unenforced.

### Syllable Alignment

NATIVE `Company.Syllable.*`: for two names whose first words are the same, the
second words match at 100 in both directions when they share their first six
letters and one is the other with more letters after the prefix, or they
differ in one letter after it (`Hospital`/`Hospitality`, `Baseline`/`Baselinu`);
words of five letters or fewer never match, and `Hillcrest`/`Hillcrset`
(two letters) does not. For a one-word name only the plural is matched, in
one direction (`Nordstrom` finds `Nordstroms`, not the reverse) for a word of
six letters or more, at a confidence that grows with the length (91 at six
letters, 98 for a word ending in a vowel sound). `Partner` matches nothing,
itself included, and `Prairie` does not find `Prairies`. The native scores
vary with the word beyond these features (`Walgreen` finds `Walgreens` at 100,
`Evergreen` finds `Evergreens` at 95), which Glade does not reproduce.

### Glade fit

API 67.0 rows, every ordered pair of every row. "fit" is every row the model
was read from; "holdout" rows were written without sight of the model. Recall
is the share of native matching pairs Glade also matches, extra the pairs Glade
matches and native does not (the bar is 0 on every row), MAE the mean absolute
difference between Glade's rounded confidence and the native one over the pairs
both match. The 62.0 rows (about half of the fit rows and a rerun of the key
rows) agree with their 67.0 rows; `cases.json` holds a bound per row.

| Method | Set | Recall | False matches | Confidence MAE |
|---|---|---|---|---|
| Company Syllable Alignment | fit | 3137/3395 (92.4%) | 0 | 0.013 |
| Company Syllable Alignment | holdout | 1325/1488 (89.0%) | 0 | 0.020 |
| Cross-object rules | fit | 52/52 (100.0%) | 0 | 0.000 |
| Custom OR / 3-item rules | fit | 57/57 (100.0%) | 0 | 0.158 |
| Custom OR / 3-item rules | holdout | 50/50 (100.0%) | 0 | 0.220 |
| Person Account custom rule | fit | 4/4 (100.0%) | 0 | 0.000 |
| Person Account custom rule | holdout | 4/4 (100.0%) | 0 | 0.000 |
| Standard Account City | fit | 68/68 (100.0%) | 0 | 0.000 |
| Standard Account City | holdout | 76/76 (100.0%) | 0 | 0.000 |
| Standard Account Name | fit | 161/174 (92.5%) | 0 | 0.106 |
| Standard Account Name | holdout | 99/103 (96.1%) | 0 | 0.061 |
| Standard Account Phone | fit | 615/2275 (27.0%) | 0 | 0.244 |
| Standard Account Street | fit | 338/488 (69.3%) | 0 | 0.296 |
| Standard Account Website | fit | 335/351 (95.4%) | 0 | 0.000 |
| Standard Account Zip | fit | 79/79 (100.0%) | 0 | 0.000 |
| Standard Account Zip | holdout | 87/97 (89.7%) | 0 | 0.000 |
| Standard Contact/Lead Company | fit | 2500/2500 (100.0%) | 0 | 0.005 |
| Standard Contact/Lead Company | holdout | 99/117 (84.6%) | 0 | 1.253 |
| Standard Contact/Lead Email | fit | 83/83 (100.0%) | 0 | 0.000 |
| Standard Contact/Lead First | fit | 490/622 (78.8%) | 0 | 0.114 |
| Standard Contact/Lead First | holdout | 432/524 (82.4%) | 0 | 0.222 |
| Standard Contact/Lead Last | fit | 340/400 (85.0%) | 0 | 0.012 |
| Standard Contact/Lead Last | holdout | 256/304 (84.2%) | 0 | 0.047 |
| Standard Contact/Lead Phone | fit | 1230/1418 (86.7%) | 0 | 0.000 |
| Standard rule structure (Account) | fit | 2118/2118 (100.0%) | 0 | 0.000 |
| Standard rule structure (Account) | holdout | 44/44 (100.0%) | 0 | 0.000 |
| Standard rule structure (Contact) | holdout | 8/8 (100.0%) | 0 | 0.000 |
| Standard rule structure (Contact/Lead) | fit | 314/314 (100.0%) | 0 | 0.000 |
| Standard rule structure (Lead) | holdout | 10/10 (100.0%) | 0 | 0.000 |
| Standard.Account.NameConfidence | fit | 42/42 (100.0%) | 0 | 1.000 |
| Standard.Account.NoStateCountry | fit | 85/85 (100.0%) | 0 | 0.000 |
| Standard.Contact.EmailDomain | fit | 63/63 (100.0%) | 0 | 0.000 |
| Standard.Contact.FreeMailDomains | fit | 27/33 (81.8%) | 0 | 0.000 |
| Standard.Contact.NameConfidence | fit | 20/208 (9.6%) | 0 | 0.000 |
| Standard.Lead.EmailDomain | fit | 63/63 (100.0%) | 0 | 0.000 |
| Standard.Lead.FreeMailDomains | fit | 27/33 (81.8%) | 0 | 0.000 |
| Standard.Lead.NameConfidence | fit | 20/208 (9.6%) | 0 | 0.000 |
| Standard.Shapes.Account.Family | fit | 1600/1600 (100.0%) | 0 | 4.013 |
| Standard.Shapes.Account.Name | fit | 67/73 (91.8%) | 0 | 2.269 |
| Standard.Shapes.Account.NameZip | fit | 55/59 (93.2%) | 0 | 2.764 |
| Standard.Shapes.Contact.First | fit | 34/36 (94.4%) | 0 | 0.824 |
| Standard.Shapes.Contact.Last | fit | 88/94 (93.6%) | 0 | 0.023 |
| Standard.Shapes.Lead.Last | fit | 88/94 (93.6%) | 0 | 0.023 |

Rows listed as fit that began as holdout (Account Street, Account Phone, Contact and Lead Phone, Account Website) were scored first as holdout: the first scoring of `Holdout.Field.Account.Street` and
`Holdout.Field.Account.Phone` showed false matches (a street with a number sign,
a seven digit phone number against a full number, a letter typed for a digit); the rules were changed
to leave those unmatched, so those rows count as fit after the change. `Holdout.Field.Account.Website`
was read once before the hyphen rule was written. The first Account structure
holdout looked wrong only because the capture org had reached its daily limit
of Apex log headers and returned no rows; those captures were repeated in
fresh orgs with a sentinel pair that must match before a row is accepted.

### Digits, non-Latin names and the capture's blank names

NATIVE `Standard.Shapes.*`: Salesforce matches names that differ only in a
digit (`Test1` finds `Test2` at 100, `42` finds `43` at 99, `Contact 1` finds
`Contact 27`, `Test Account 1` finds every `Test Account N`, at 94 to 100
depending on where the digits sit) and names in another script (`王` finds `李`,
`Иванов` finds `Петров`, at 100), because its phonetic key ignores digits and is
empty for those scripts. Glade matches the same pairs; it keeps one native
exception, that a name of digits only never finds a name in letters (`42` and
`王` do not match). Apex test data with numbered names therefore reports
duplicates under the standard rules as it does in an org. A record with a
blank required name was captured with the placeholder `Blankrow` (the same text
on both sides), so no row covers blank Account names or Contact last names.

### Standard rule gaps

- The standard rules still miss some phonetic first names and surname variants.
  The `duplicate_save_behavior` fixture exercises single-letter initials,
  approximate surnames, explicit Person Account record types, edit actions,
  scalar upserts and two overfull Account searches through Apex at both APIs.
  The table above records the earlier scorer fit. The new standard-name checks
  use a matching surname plus email or phone for an initial, and an exact first
  name and email for additional surname variants. Custom field-method rows
  have separate match-key behavior.
  The short-name and six-letter terminal transposition panels separate native
  saves from rejections even when the edited letters agree. The measured
  correction is limited to an accepted keyboard comparison corroborated by
  first name and email. For eligible six-letter names, exchanging letters four
  columns apart on the same or neighboring QWERTY rows matched at 97 when
  the sum of `abs(2*column-11) + 2*row` over the name was at least 44.
  Row and column start at zero. Phonetic hits scored 100. The 80 unseen inputs
  in the boundary fixture agreed on match identities, saves, and recorded
  integer confidences at both APIs on the anonymous and test routes. The
  underlying Salesforce distance function and wider input range remain open.
- Street: native scores the street sections multiplicatively (a different
  suffix word 91, a different street number 95); Glade matches the pairs the
  custom Street method matches and approximates the confidence.
- Phone: on the Account phone panels, the website, ZIP and street also supply
  matching paths. Missing records on those panels cannot all be attributed to
  the phone comparison. Identical international and extension strings in the
  Contact save fixture return no match with only the names as corroboration.
- Standard Person Account matching rule: not activatable by metadata deploy,
  so not captured; unenforced in Glade.
- Raw (below threshold) item scores for Company Name and Title count 50 in an
  OR rule's confidence, and the Keyboard Distance score of two unlike last names
  is approximate (OR rules report 0.2 to 0.4 points of confidence error).
- Syllable Alignment: only the two-word second-word rule and the one-word plural
  are modeled; first-word variants, three-word names, vowel substitutions and
  the varying plural confidences are misses.
