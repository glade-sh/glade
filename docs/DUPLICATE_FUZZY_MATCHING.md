# Duplicate fuzzy matching: public spec, native rows and the model

Four fixtures hold the native rows. The Apex save and search routes are
covered by `duplicate_save_behavior/`, including default and custom rule
actions, error members, Person Accounts, upsert, and controlled surname panels. `duplicate_standard_rules/` (second fixture,
described under "Standard rules" below) covers the standard Account, Contact
and Lead rules, the cross-object rules, custom rules with OR groups and three
items, a custom Person Account rule and the Syllable Alignment pairs; its test
is `TestStandardRulesNativeRows` (`internal/dml/standard_rules_conformance_test.go`).
`duplicate_fuzzy_panels/` (described under "Custom rule panels" below) replays
custom-rule value panels through both Apex routes; its test is
`TestDuplicateFuzzyPanelsNativeRoutes` (`internal/apextest`).

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
2. Match key. Otherwise the two names are compared only when their first
   words, every character other than a letter or digit removed, are equal or
   the first two characters of the Double Metaphone codes of those words
   agree (`fuzzy_name_keys`: `Y R` and `Y R S` share the key of `Y`, `PW` and
   `P W` are compared, `YR` and `Y R` are not; `Billy Bob` finds `Billy` and
   `Jane.` finds `Jane` although both are alias-keyed). Invented names show the
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
   `Louis`. Under a custom rule `Ty` is keyed TR, `Ray` RM and `Sue` SS
   (`fuzzy_first_alias_keys`: `Ty` finds `Tyr` and not `Tye`, `Tyl` or the
   initial `T`; `Ray` finds `Raym` and not `Rae` or `R`; `Sue` finds `Sues`
   and not `Su` or `S`; `nameAliasKeysCustom`). `Roberta`, `Jane`, `Tess`, `Tony`, `Becky`, `Nick`, `Rick`,
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
   salutations are not removed (`Dr. Jane` finds nothing). Inner white space
   is compared as typed: `Mary Ann` finds `Mary  Ann` at 99, not 100, and
   `Mary` does not find `Mary  Ann`.
5. Initials. Before Jaro-Winkler, a pair scores 100 when one name's first
   word, punctuation removed, is a single letter and the other name is one
   word starting with that letter (`fuzzy_name_keys` `in1`, `in2`, `din`):
   `P W G` finds `PW`, `P.W.`, `P-W` and `P` at 100. `P W` finds `P W G` only
   by Jaro-Winkler (95) and `Yancey` finds `Yancey R` only by Jaro-Winkler.
   `Y` does not find `Yancey` and `Y.R.`: the match key comes first.

The standard rules keep the whole-name key and Jaro-Winkler on the folded
text (`standardFirstName`); the rows for rules 2 and 5 are custom-rule rows.

### Last Name

NATIVE `P4.LN.*`, `N4.LN.Var.*` and the earlier rows.

1. Match key: the Double Metaphone code of the last word, accent-folded,
   four characters at most. Characters other than letters, digits and white
   space are removed first (`Al-Rashid` keys as `Alrashid`), and trailing
   `Jr`, `Sr`, `II`, `III` and `IV` are dropped while another word remains
   (`Harlow Jr` keys as `Harlow`; `Harlow PhD` keys as `PhD`). A last word with
   no letter keys as itself (`fuzzy_name_keys` `ln1`, `ln2`: `Test 1` finds
   `Last 1`, not `Test 2`; `1` finds neither `10` nor `Smith 1`).
   `Frost`/`Frosr` (FRST, FRSR), `Smith`/`Smiht`,
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
   are compared whatever separates the words (`Harlow` finds `Harlow III`,
   `Al-Rashid` finds `Alrashid`, `van der Berg` finds `van den Berg` at 99).
   `Smith Jones`, `De La Cruz` and `Mc Donald` do not find `Smith-Jones`,
   `Delacruz` and `McDonald` because their last words key apart. The standard
   rules keep the whole-name key and require equal spaces and hyphens
   (`standardLastName`; their rows, such as `42` finding `43`, are fitted to
   that form). This replaced a hand-written stand-in that rejected anything it
   could not prove and missed dropped vowels, silent letters and name codes.
3. Keyboard Distance, decided at 90, confidence `(100 + score) / 2`. Strings
   are compared position by position over the shorter length, with no
   alignment. In `fuzzy_name_keys` panels ks1 to ks3, suf1 and rsp1,
   `Farnsdelwick` finds
   `Farnsdraeovk` at 96 with five substitutions; `Abernathy` finds `Abernaty`
   at 99; `Frank` does not find `Franklin`). The sum is divided by the longer
   length. The costs are fitted to the 2,528 distinct native pairs that reach
   this step (`keyboardDistance` in `internal/dml/fuzzy_keyboard.go`) for
   unequal lengths or two or more substitutions. Equal-length pairs with one
   substitution retain the earlier `keyboardSimilarity` scorer. Pairs that
   the earlier scorer already matched also retain its confidence; the new
   scorer applies only when that scorer did not match and the pair's joint
   shape is one where a captured pair needs it: the longer name's length in
   runes, the count of positional substitutions and the count of trailing
   runes, taken together, for a fitted pair native matches and the earlier
   scorer does not. The native rows hold 78 such shapes (`keyboardPanelCells`
   in `internal/dml/fuzzy_match.go`), with lengths 4–15. For any other
   shape, including untested combinations inside those ranges, the earlier
   keyboard scorer decides. Inside a shape the new scorer decides only at a score of
   93 or more (`keyboardFallbackMargin`), one margin for every shape. The
   fitted score is not reliable below it: native non-matches score 90.1 to
   90.84 in substitution shapes (`duplicate_fuzzy_panels` k1 to k3:
   `Kellingswokwr` and `Kellinglkokzh`, `Gilderpajt` and `Gildewrarb`) and up
   to 92.26 in extension-only shapes (the blind twin panels tk01 to tk22:
   `Ingleborn` and `Inglebornxbqm`), while native matches also sit there
   (`Farnsdelwick` and `Farnsdraeovk` score 92.07). The margin is 0.74 above
   the highest captured native non-match of any shape in any panel (fitted
   rows, k1 to k3, hk1, hk2, tk01 to tk22, bk, ck). Per-shape floors were tried and
   dropped: a floor existed only where an earlier panel held a
   counterexample, and a fresh panel then showed 108 more false matches. The
   margin leaves the earlier result for the native matches that score below
   it, so recall in these panels is below the fitted scorer's and above the
   earlier scorer's. The shape table comes from the fit panels
   (`fuzzy_name_keys`, `duplicate_fuzzy_matching`) with hk1 in view; k1 to k3
   set the earlier per-shape floors. hk1 and hk2 are not held-out evidence for
   the table. The twin panels (tk01 to tk22), the second blind panel (bk01 to
   bk09) and the third (ck01 to ck09) were captured after the code they
   score. bk and ck fitted nothing; the twin's tk14 sets the highest native
   non-match that the margin clears. `TestLastNameKeyboardFuzzyPanelsNativeRows`
   requires zero extra matches on all three, and
   `TestLastNameKeyboardFallbackMargin` requires the margin above the highest
   captured non-match. Panels kept out
   of the fit agree on every pair Glade matches:
   `fuzzy_keyboard_neighbors` (`Farnsdelwicka` finds `Farnsdraeovka`, native
   97; its score is below the margin, so Glade keeps the earlier result).
   A second, flat score follows the fitted one in every shape
   (`keyboardFlatDistance` in `internal/dml/fuzzy_keyboard_flat.go`). Each key
   step costs 0.0759 whatever the position or key pair, each trailing letter
   costs 0.2437, the sum is divided by the longer length, and the pair
   matches at `keyboardFallbackMargin` (93) with confidence
   `(100 + score) / 2`. It is a clean-room fit to the native rows; it adds the
   matches the fitted table misses and keeps every earlier match and
   confidence. Over every captured LastName pair it adds 582 unordered pairs
   and no false match; the highest captured native non-match scores 92.81
   (`Kesterloy` and `Kestermlinb`, tk07). It recovers 86 of the ck native
   matches the fitted table left, with no false match on the blind ck panel.
   Spaces, punctuation and accents keep the fitted costs.
   The fourth blind panel set (`dkeyboard`: dk01, dk02) was captured after
   this score and its predictions were frozen. It shows no false match, and
   it shows what the score lacks. In dk01 every pair with the same key steps
   and a flat score of 91.65 to 92.41 matches natively at 96, whether the two
   Metaphone codes are one character or four apart, so a graded Metaphone
   component is not the missing part. In dk02 pure extensions match natively
   at 3 and 4 trailing letters on five stems and at 5 or 6 on some (Bellingmoor
   at 6 letters, flat 91.40, matches; Hartlebury at 5, flat 91.88, does not),
   so no function of the flat score alone separates them. The margin stays 93;
   the rows are pinned and the unreported matches stay misses.
   The fitted costs are:
   - a substitution costs by key distance, the column distance plus two
     between the top and bottom rows: 0.06 for neighbours and 0.079 a step
     beyond; the final character of two names of equal length is one step
     cheaper. About 120 key pairs need a cost other than the
     formula and are listed in the code;
   - an accent costs 0.16; a letter against a space or punctuation mark and a
     space against a punctuation mark have their own costs; digits never
     substitute;
   - a trailing character costs by the character, with one table after an
     otherwise equal name (about 0.25) and one after a substitution (top row
     about 0.2, bottom row about 0.35).

   The one-substitution legacy score does not reproduce every native
   confidence: `Harlow II` / `Harlow IV` rounds to 98, versus native 97.
   The standard rules keep the earlier
   costs (`keyboardSimilarity`: one substitution, two with a score of 92 or
   more, or up to two trailing letters), which their rows were fitted with.

The apostrophe-removed form matches (`O'Connor`/`OConnor`, `O’Leary`/`OLeary`);
a space splits the last word (`O'Brien`/`O Brien`, `Mc Donald`/`McDonald`).

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

Custom Company Name rules score some shapes with a fitted directional model
(NATIVE `internal/dml/testdata/company_name_fit`): names with digits
(`Test Account`/`Test Account 1` 97, the reverse 94), a trailing Partner, SARL
or BV, a hyphen or underscore join on one side only, and two different
trailing qualifiers (`/West`, `(West)`, ` - West`). Those pairs use a key of
the first six characters of the first two words and match at a fitted
confidence of 85. Numbered and designator pairs whose first two words differ
past the key (`Qelthorin Freights 1`/`Qelthorin Freighter 1`), numbered pairs
the word scorer keeps apart without their numbers and a ` - ` qualifier
against a different qualifier keep the word scorer. So do a joined
side with more words than the other (`Dunder-Mifflin Paper`/`Dunder Mifflin`),
short ampersand forms (`Q&V`/`QV`) and a plain name against a different ` - `
suffix. The standard Account rule keeps the word scorer for every pair.

The D1 capture (`company_name_fit` rows `D1_*`) settled the shapes the fit had
extrapolated. Same words with different numbers match (`Zorvex Region 1
Sales`/`Zorvex Region 2 Sales`); with a differing word, both names need a
number and the numbers must be equal or one must start the other, and a digit
glued to a word (`Zorvex Region1`) does not match `Zorvex Region 1 Sales`. An
underscore join does not match its spaced name and a hyphen join matches only
when the aligned words are alike (`Vexlett-Packard`/`Vexlet Packard` yes,
`Quen-State Paving`/`Quen State Painting` no). A slash and a parenthesis form
need equal suffix word counts and alike bases. Known gaps, each with a residual
in the test: 13 native matches the word scorer also rejects, such as `Zorvex
Partnership`/`Zorvex` and three `East`/`West` pairs across a dash and a
parenthesis or plain form, and 10 pairs
the word scorer matches and native does not (`Zorvex Partner`/`Zorvex Partner
Inc`).

The R1 and R2 captures (rows `R1_*` and `R2_*`, ten panels of 23 names) vary
those shapes and bound the rules:

- A slash joins the words on either side of it in the match key. `Zorvex
  Freight/West` keeps the key of `Zorvex Freight West`; `Orbel Trade/West`
  does not match `Orbel Trade West`, `Orbel Trade - West` or `Orbel
  Trade/East`, and `Orbel Trade/Midwest` matches `Orbel Trade/Mideast`. No
  captured match with a slash has unequal joined keys.
- A doubled letter matches only past the first six letters of the first word:
  `Vexlett Packard`/`Vexlet Packard` yes, `Vexllet Packard`, `Qell
  Systems`/`Qel Systems` no. Other endings past the key match too; see the R3
  rules below.
- Two different one-word suffixes across a dash and a spaced slash match when
  both start with a consonant and are alike (`West`/`Best`, `Mast`/`Last`). A
  suffix that starts with a vowel finds an alike consonant suffix and is not
  found by it (`East` finds `West`). Two vowel suffixes match when only the
  first letter differs. Against a parenthesis form only suffixes with the same
  consonants match (`West`/`Wast`).
- The number rule above covers names of up to three words. A `#`, a `No`
  before a number, a number with a letter (`3A`) and a Roman numeral are part
  of the number. `Zorvex Region1` finds `Zorvex Region 1 East`, and `Zorvex
  Region 2 East` finds `Zorvex Region 1 West`; the reverse pairs do not match.
- Digit runs glued to the same multiword name score 100 when they have one
  length and differ in one position (`Test Account12`/`Test Account13`), when
  one starts the other (`Test Account1`/`Test Account123`) or when one is the
  other with one digit added (`Test Account1`/`Test Account21`). Other glued
  runs score the syllable alignment of the whole names: `Test
  Account11`/`Test Account22` 97, `Test Account1`/`Test Account20` 95, `Test
  Account456`/`Test Account1` 91 (301 of the 302 directed `R2_glu` pairs).
- A hyphen or underscore digit join scores 91 against a single consonant
  (`Test Account_2`/`Test Account D`); a vowel or `Y` keeps its earlier score.

`TestCompanyShapePanelCounts` pins each panel: 2,240 of 2,351 captured matches
reported, 1,378 exact confidences, 40 matches native does not report.

The R3 capture (rows `R3_*`, four panels of 23 names) isolates the rules the
R1 and R2 rows left open:

- First words that share the six-letter match key match at 100 in both
  directions when one is the other with letters added (`Vexlet`/`Vexletronics`,
  `Qelthorin`/`Qelthoriner`) or when both have one length and differ in one
  letter after the key (`Vexlets`/`Vexleta`, `Zorvexs`/`Zorvexa`); the rest of
  the name keeps the word scorer's rules (`Vexlet Packardd`/`Vexlett Packard`
  100, `Vexlet Packard Group`/`Vexleta Packard` 91). Both names need two or
  more words of letters only. Not matched: `Vexlets`/`Vexleter`,
  `Vexletab`/`Vexletcd`, `Vexle`/`Vexles` and the one-word names. Native
  also matches some longer and tripled endings in one direction
  (`Vexlettt`/`Vexlets` 97, `Vexlets`/`Vexletronics` 91,
  `Qelthorins`/`Qelthoriner` 100); Glade does not yet.
- A spaced slash against a spaced slash or a plain form with the same base
  and a different one-word suffix scores as the dash form, whatever the suffix
  lengths (`Orbel Trade / Art` finds `Orbel Trade / North`). Names with digits
  and a vowel candidate against a parenthesis form keep the earlier decision.
- A plain candidate whose suffix starts with a vowel and has four or more
  letters finds a dash suffix of the same length that differs from it only in
  its first two letters (`East` finds `- West` at 93 and `- Mast` at 94, not
  `- North`), and a vowel dash suffix that differs only in its first letter at
  96. The base needs two words besides a legal designator.

The R4 to R7 captures (rows `R4_*` to `R7_*`, fifteen panels of 25 or 26
names) test those rules on names they were not fitted on. They confirm the
first-word rule, the spaced slash rule and the plain vowel candidate, and they
set these limits:

- A first word followed only by a legal designator is a one-word name:
  `Vexlets Inc` does not find `Vexlet Inc`, `Vexlets Group` finds `Vexlet
  Group`.
- After a legal designator a glued run scores 100 only when it starts the
  other (`Test Account Inc1`/`Test Account Inc12`); one added digit does not
  (`Test Account Inc2`/`Test Account Inc12` 99 and 98).
- A vowel suffix against a consonant suffix across a dash and a parenthesis
  form (`(East)`/`- West`, `(Oast)`/`- Best`, and the first-letter change
  `(East)`/`- Wast`) matches on some bases and not on others. 100 further bases
  were captured. No length or score of the whole names orders them: by the syllable
  alignment of the two names `Mandolar Outlet` rejects at 95.45 and `Orbelvanto
  Bank` matches at 93.00, and `Plovdi Trade` matches where `Zorvex Audio`
  rejects at the same 94.05. Native reports these matches at 90 to 93. Glade
  reports none of them. Capture S1 (26 further bases) found no separator in
  length, word class, first-word length or vowel identity either; the rows are
  pinned as misses in `company_s1_panels_test.go`.
- Two parenthesis forms on one base whose one-word suffixes both start with a
  vowel and differ only in that letter (`(East)`/`(Oast)`/`(Uast)`) match at
  100 in every direction on every captured base (S1, R2_par, R3_long). Glade
  scores them 100 for any vowel pair. The captures cover E, O and U; I and A
  are untested.
- Word endings on the `Vexlet` stem (S1 `s1_d`) that Glade still misses, such
  as `Vexletob`/`Vexleta` at 96 and `Vexlettt`/`Vexletab` at 93, and
  `Qelthorins`/`Qelthoriner` at 100, rest on one base each and are pinned, as
  is `Vexlet Inc` finding `Vexlets Inc` at 93 (Glade 91).
- A ladder on one base (`company_name_fit` T1_first and T2_last) holds the
  native confidence at 91 while the letters change: `Plovdi Trade (East)`
  finds `Plovdi Trade - West` on all 13 first-word variants and on 11 of 12
  last-word variants (not `Plovdi Trad`; `Plovdi Tradex` reports 92). The
  earlier syllable alignment candidate predicted none of the 13 and 7 of
  the 11. Glade still reports none; `TestCompanyVowelLadderPinnedMisses` pins
  the rows.
- A dash candidate with a vowel suffix against a plain name (`- East`/`West`,
  native 91) is the same: it matches on `Orbel Trade`, `Zorvex Freight` and
  `Qa Trade` and not on `Mandolar Oil`, `Orbel Oil Outlet` or `Ab Logistics`.
  Glade reports none.
- Suffixes of one to three letters do not follow the four-letter rules
  (`Zorvex Freight Art` does not find `Zorvex Freight - Bat`).

`TestCompanyIsolationPanelCounts` pins the R3 panels (400 of 458 captured
matches reported, 297 exact confidences, none that native does not report) and
the R4 to R7 panels (1,224 of 1,398, 864 exact, none that native does not
report; the glued-digit threshold and the qualifier and direction ceilings
removed the 33 the word scorer and the earlier rules reported).

### Phone

Letters and digits are the characters of the number; separators are dropped.
Ten characters, or eleven with the country digit first, parse into country,
area, exchange and line; other lengths never match, not even themselves.
Sections are compared exactly with weights 10, 50, 30, 10; the confidence is
the weighted score; a match needs 80. A letter typed for a digit therefore
still matches (`312 555 O199` finds `312-555-0199` and `312-555-0190` at 90).
A country digit other than 1 never matches the same number with a different
or no country digit. The N5 custom phone panels (`phone_city_zip`: line
transpositions and the 80 threshold, extensions written `x12`, `ext. 12`,
`,12`, `#12` and more, international numbers with `+`, `00` and `011`
prefixes, and seven to nine digit numbers) are reproduced pair for pair by
this model: a different line is a candidate at 90, no extension form finds
another or the bare number, and short numbers never match.

### City, Street, ZIP, Title

Unchanged from the third run's model.

- NATIVE City: match key = first six letters or digits of the accent-folded
  value with spaces and punctuation removed; a different key never matches
  (`Honolulu`/`Honalulu`, `Seattle`/`Seatle`, `Portxand`/`Portland`). With the
  same key, Edit Distance score `100 * (1 - d / longer length)` on the raw
  lower-cased strings (accents and dots count as edits: `Düsseldorf`/
  `Dusseldorf` 95, `St. Louis`/`St Louis` 94) decides at 85 (`Salt Lake City`/
  `Salt Lake Ciyt` 85.7 matches, `San Francisco`/`San Francsico` 84.6 does
  not). Confidence `(100 + score) / 2`. Inner spaces count as typed
  (`phone_city_zip` N5.cm1: `Los Angeles`/`Los  Angeles` 96, `LosAngeles`/
  `Los  Angeles` no match); tabs, line breaks and non-ASCII text keep the
  folded comparison, which no row measures. The raw City score an unmatched
  item adds inside AND and OR groups compares the same text. The N5 City panels (one edit at
  each position of `City`, a length series from one to eleven letters, multi-
  word, abbreviation and city-plus-state values) are reproduced pair for pair
  under the custom and the standard Account rule, which match the same pairs.
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
  `Ct`, `Court`, `Ter`, `Pl` and `Sq` stay in the name. The shared scorer (standard rules) rejects a pair
  whose last name word and name stem both differ (`Harbor Lp`/`Harbour Loop`
  does not match natively) and, with no street number on either side, a name
  similarity under 80 (`Calle 8 No 5`/`Calle 8 Numero 5`); the custom method
  needs neither, the match key (below) keeps those pairs apart. A single letter
  after the number is not only `E`: natively `100 A`, `I`, `O` and `Y Main St`
  match `100 Main St` at 83 and `De`, `La`, `Et`, `And`, `The` do not; Glade
  drops `E` only. Confidence `(S + edit similarity of the cleaned strings) / 2`:
  `15 Elm St`/`15 Elm Drive` = (82.35 + 58.3) / 2 = 70, `123 Main St`/
  `123 Main Street` = 87, `Oak St 100` against itself = 90 (number blank,
  trailing number as suite).
- NATIVE Title: words are lower-cased. Dots inside a word are dropped
  (`C.E.O.` is `ceo`, `V.P.` is `vp`); a trailing comma of the whole title is
  dropped, a comma or semicolon inside it stays on its word and counts as a
  character (`Zorp, Quill` does not find `Zorps Quill`, `Director, IT` does
  not find `Director/IT`). `of`, `and`, `the`, `&` and `it` are stop words
  (`Director IT` and `Director` are 100; `&` is a stop word of its own, so
  `Zorp & Quill` and `Quill and Zorp` are 93, not 100; an ampersand at the
  edge of a word is that stop word, `Chief& Officer`). Exact when the content words
  are the same sequence or all words sort equal (`Sales Director`/`Director
  Sales` 100, `VP Sales`/`VP of Sales` 100, `Sales`/`Sales Sales` 100). A word
  equals any word it starts, two letters or more (`Dir`/`Director`,
  `Eng`/`Engineering`, `II`/`III`, `Pr`/`Product` and `Principal`), except
  `IT`. Two words one edit apart are equal when the longer has six characters
  or more (`Coordinator`/`Coordinater`, `Zorps`/`Zorpas`, `Alpha,`/`Alphas`
  and `Alpha,`/`Alpha;`; not `Alpha`/`Alphs`, `Zorp,`/`Zorps` or `CEO`/`CFO`). Two one-word titles need an exact match (`Director`/`Directors` and
  `Dir`/`Director` no match), but a prefix is enough when one title has more
  words (`Dir` finds `Director of IT`). Initials match (`CMO`/`Chief Marketing
  Officer`, `BDM`/`Business Development Manager`, `Bus Dev Manager`/`Business
  Development Manager` 100). Otherwise the confidence depends on the number of
  shared words k and the larger content-word count n: (1,2) 93, (1,3) 89,
  (1,4) 88, (2,3) 95, (2,4) 93, (2,5) 91, (3,4) 97, (3,5) 94, (4,5) 97, and
  six words in their own table. The same words in a different order with a
  stop word present: 94 when the candidate has no stop word, 93 when it has
  one (`Product Head` finds `Head of Product` at 94, the reverse 93). All the
  words of the shorter title in the longer one in exactly reverse order lose
  by direction and length: a shorter candidate (2,3) 1, (3,4) 2, (3,5) 1,
  (3,6) 2, (4,6) 1, (5,6) 2; a longer candidate (2,3) 4, (3,4) 4, (3,5) 5,
  (4,6) 4, (5,6) 2, and (3,6) no match. Other shared words out of order lose 3,
  or 5 for two shared words in a title of four. A candidate with a stop word
  whose words all occur in the existing title scores 94 where the base is 93
  (`Zorp Of` finds `Zorp of Quill`). The same content words in the same
  order are exact whatever the stop words (`Zorp Quill` finds `Zorps and
  Quill` at 100). Two titles whose content words share nothing score by the
  stop words they share (`titleStopOverlapConfidence`, by candidate words,
  existing words and shared words): a candidate inside the existing title
  (`of` finds `of the` 91, `Zorp of Quill` 89 and longer titles 88; `of it`
  finds `Zorp of IT` 94), the existing title inside the candidate (`Zorp of
  IT` finds `of it` 91; a one-word title is never found, so `Zorp IT` does not
  find `IT`), or neither (`Director of IT` and `Head of IT` 88; `Director IT`
  and `Head of IT` no match). A title of stop words alone has no content:
  `IT` finds `the it` at 91 and `the it` does not find `IT`. Counts no row
  holds do not match. One
  shared word against a five-word title matches only from the one-word side
  (`Global` finds `Senior Global Regional Operations Analyst`, not the
  reverse); Glade matches neither.
- NATIVE Zip: only the segment before the first hyphen, space, period or
  slash decides (`phone_city_zip` N5.zu1: `12345.6789` and `12345/6789` find
  `12345`); confidence 100, the extension is never scored (N5.zx1:
  `94105-0001`/`94105-0002` 100); a value under three characters matches
  nothing.

NATIVE blanks: with Match Blank Fields two blank values match (100); with the
default they never do; one blank value never matches.

## Glade fit

Recall is the share of native matching pairs Glade also matches; false matches
are pairs Glade matches that native does not; confidence MAE is the mean
absolute difference between Glade's rounded confidence and the native one over
the pairs both match. The table records the earlier Metaphone 3 fit before
the keyboard changes above; current per-row bounds and residuals are in the
conformance tests. API 67.0 rows, every ordered pair of every row. "fit"
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
| Street | fit, older rows | 2179/2179 (100.0%) | 0 | 0.000 |
| Street | holdout (unbiased), older rows | 151/151 (100.0%) | 0 | 0.000 |
| Street | fit, `fuzzy_street_c4`, `c5`, `c8` | 9348/9348 (100.0%) | 0 | 0.000 |
| Street | holdout, `fuzzy_street_c4`, `c5`, `c8` | 3428/3428 (100.0%) | 0 | 0.000 |
| Street | fit, `fuzzy_street_c9` | 218/218 (100.0%) | 0 | 0.000 |
| Street | holdout, `fuzzy_street_c9` | 906/906 (100.0%) | 0 | 0.000 |
| Street | fit, `fuzzy_street_c10` | 1522/1522 (100.0%) | 0 | 0.000 |
| Street | holdout, `fuzzy_street_c10` | 882/882 (100.0%) | 0 | 0.000 |
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

- Last Name: Glade does not reproduce the 2 of 201 unbiased-holdout and 2 of
  173 seen-once pairs that native matches and the port does not. They are long
  names the key cuts differently (`Kowalczyk`/`Kowalsky`), respellings the
  port codes apart (`Robards`/`Robertz`, `Waalker`/`Walkker`) and a
  hyphenated surname against its first part (`Fitzgerald`/`Fitzgerald-Lee`).
- Keyboard Distance: the costs are additive, and native rows that share key
  pairs disagree by up to a tenth of a point of cost, so about one confidence
  may differ by one point; previously matched pairs retain their earlier
  confidence as listed in `fuzzyNameKeysResiduals`. No row measures a
  trailing `c`, `q`, `u`, `w`, `x` or digit on a native match. The standard
  rules do not use the refit yet.
- First Name: the Name Variant dictionary and its alias keys are unpublished.
  Glade carries the pairs and alias-keyed names proven for about 2,700 common
  first names and 590 nickname families. A nickname pair outside those rows
  is a miss. A dictionary name outside those rows whose alias key differs
  from its own key can match a near spelling by Jaro-Winkler in Glade and not
  natively.
- Syllable Alignment is modeled only where the second fixture proves a rule
  (see "Syllable Alignment"). Native confidences of one-word plurals vary with
  the word beyond what the length and last letter explain; vowel variants, an
  `&` inside a word against the joined spelling (`AT&T` finds `ATT` one way)
  and acronyms outside the proven entries are misses.
- Street: see "Custom rule panels" for the custom Street method, its match
  key and what is not measured. The Street rows of the table above are the
  custom method's rates on the older rows and on the `fuzzy_street` captures;
  the standard rules keep the shared scorer.
- Title: the (k, n) table covers up to six words. A word moved to the other
  end of a title (a rotation) keeps the flat loss of 3 where the native loss
  varies, and `Director IT` finds `Director - IT` at 94 natively but `Director,
  IT` finds it at 93 (the comma rule above is inferred from both).
- Not captured: MatchBlanks on Contact fields, blank values on the last name
  and company name. The standard rules, the cross-object rules, OR groups and
  three-item rules are in the second fixture; its gaps are listed under
  "Standard rules".
- Unsupported fuzzy rules (unknown method names) are still left unenforced.
- Remaining gaps of `duplicate_runtime_behavior/` (field differences, update
  gate, entity ids, report sets): the field differences of custom rules with a
  fuzzy item and of Person Account rules are not captured, so Glade reports
  none; the fuzzy confidence behind `FDIFF2` (`fuzzyFirst`,
  `queryHasStoredNull`) is compared only for the field differences of the
  records both sides return, because the fuzzy scorer owns it; the update gate
  compares an Apex update of a record inserted in the same transaction with its
  insert-time values (NATIVE `RPT_*`), while other repeat edits compare with
  the stored values; a flow update, a standard-rule update report and
  `runAsCurrentUser` are not captured.

## Standard rules

The standard rules are off by default. `duplicateRules.standard: on` in
`glade.yml` (or `GLADE_STANDARD_DUPLICATE_RULES=on`) turns on the standard
Account, Contact and Lead rules that no project metadata defines. A project's
`duplicateRules` metadata for a standard rule (for example
`Standard_Account_Duplicate_Rule`, active or inactive) is honored exactly, and
custom duplicate rules from project metadata are always enforced, fuzzy
methods included. The conformance tests run with the setting on.

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
extension of up to five digits (`94105` finds `94105-1234`, `94105-` and
`94105-000` at 100, two different extensions score 95, `94105 1234` with a
space is not an extension, one space beside the hyphen of a four-digit
extension is, and non-US codes match only themselves with their inner spaces
as typed: `phone_city_zip` N5.szu1, N5.szx1, N5.szn1), the website drops case, the scheme, a leading `www.` and a lone
trailing slash (`acme.com/` does not find `acme.com`; `a-cme.com` finds
`acme.com` at 94), phone numbers differing in the last four digits score 94 for
an Account, 97 for a Contact and 98 for a Lead, and a letter typed for a
digit finds nothing. The standard Contact and Lead phone key leaves out the
last four digits (`phone_city_zip` N5.spt1, N5.slt1: `1-415-555-1234` finds
`1-415-555-1243`, `415-555-4321` and `+1 415 555 9999`); the verdicts of the
N5 standard phone panels are exact.

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

Each OR group is found through its own predefined match key (Salesforce Help,
"Standard Contact Matching Rule and Standard Lead Matching Rule"), and the
key decides which name comparisons run (NATIVE `duplicate_rule_match_keys`,
B2 `s_*`, and the `Standard.Field.*`, `Standard.*.NameConfidence` rows):

- The email path is keyed on Email alone, so neither name is keyed. The last
  name is scored by the Last Name algorithms (equal Metaphone 3 codes score
  100 whatever separates the words: `McAllen` finds `Mc Allen`, `Harlow`
  finds `Harlow III`, never `Harlow Jr`; Keyboard Distance otherwise). The
  first name is scored by Exact, Initials, Jaro-Winkler at 0.85, Metaphone 3
  and Name Variant (`Y` finds `Y R S`, `Jane` finds `John`, `Mary Ann` finds
  `MaryAnn`). The standard rule also scores a few pairs 100 that custom rules
  do not (`Jack` and `John`, `Ted` and `Edward`, `Molly` and `Mary`; NATIVE
  `standard_first_names`); the pairs are not transitive. Two blank last names are ignored beside two first names. The
  measured surname panels (`duplicate_save_behavior`, `surname_keyboard`,
  `surname_substitution`) keep their keyboard geometry whatever the first
  names.
- The phone path is keyed on `First_Name (1,1) Last_Name Phone`: the surname
  key applies (`McAllen` does not find `Mc Allen`), and an initial or a name
  that differs only in spaces passes when the initials agree.
- The company path keeps the standard First Name item; no row measures its
  key.

A first-name score s counts (200 + s) / 3 in the rule confidence on the email
and phone paths (`Y R` and `Y R S` score 97 on the standard rule, 95 on a
custom rule). An existing record without a first name never matches a
candidate that has one. The company branch needs the same company (a Lead's company text, a
Contact's account) or two email addresses at one domain, and then the same
title, or the same street with the same city or ZIP code. Two addresses at one
domain count whatever the domain (gmail.com included, `Standard.*.EmailDomain`).
Contacts compare accounts by id; a Lead candidate's company text never
equals a Contact's account. The confidence is the product of the last name,
first name and path items.

Custom rules key their items by the rule too: a First Name item whose every
AND group also holds a Last Name item keys on its initial,
`First_Name (1,1) Last_Name` (C1 K3, K4, K7, K8, K9: `Vorkel` finds `Volkel`,
`Y` finds `Yancey`, `Kristen` does not find `Cristen`). Alone or beside Email
it keeps its own key (K1, K6). An item that did not match counts its raw
score in an OR group; equal Metaphone 3 codes give a Last Name raw score of 100.

### Cross-object rules

`Standard_Rule_for_Contacts_with_Duplicate_Leads` and
`Standard_Rule_for_Leads_with_Duplicate_Contacts` apply the other object's
rule to the record's own fields (a Contact's account becomes the Lead's
company). NATIVE `Standard.Cross.ContactToLead` and `Standard.Cross.LeadToContact`:
the structure is the Lead rule in the first direction and the Contact rule in
the second, and a Lead candidate's company never equals a Contact's account.
Glade enforces them when the rule is active.

Custom cross-object rules (a duplicate rule whose matching rule belongs to
another object, joined by an `objectMapping`) are enforced for the captured
shape: NATIVE `XMAP` (`duplicate_runtime_behavior/`) maps one input field onto
one output field, the matching rule reads the mapped values, and every unmapped
field counts as blank. Mappings of several fields, mappings between standard
objects and Person Account custom rules are not captured and stay unenforced.

An Allow+Report rule commits one set for records that match each other.
Glade lists the matches enforcement found first, then the peers inserted
earlier by the same statement. That order is a design choice: the `REPR` rows
list sorted names, so no native row pins the order inside a set.

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
  integer confidences at both APIs on the anonymous and test routes.
  The `surname_keyboard` fixture adds 672 directed inputs in panels A through
  I, including Contact and Lead mirrors. Each method has two matching native
  repeats at APIs 62.0 and 67.0 on anonymous and named test routes. Equal
  Metaphone 3 trailing-letter pairs score 100. Four-to-five extensions always
  qualify with the same first name and email: p scores 99; s scores 98 below
  sum 42 and 99 from 42; z scores 97 at sum 16 and 98 from 18. Other tails
  keep their ordinary grade. Three-to-four extensions also qualify, with the
  captured s, p and k grade buckets pinned by the tests.
  The `surname_substitution` fixture adds panels J and K: 624 directed inputs
  plus exact and unrelated sentinels, with the same API, route and repeat
  matrix. Panel J measures first, middle and final substitutions, lengths
  four through eight, both directions and both objects. Same-row adjacent,
  vertical and diagonal neighbors all match with exact first name and email.
  Five through eight letters score 100. Four-letter neighbors score 99,
  except captured cross-row pairs whose smaller keyboard position sum is
  at least 34, which score 100. High-sum same-row four-letter grades remain
  unmeasured and keep 99. Three-letter supplementary substitutions remain
  limited to panel D's terminal, same-row neighbors, at 99.
  Far-key controls are position-sensitive through the whole-name sum: bottom
  row substitutions six columns apart score 97 at length four from smaller
  sum 40, and at length five; length six scores 97 below smaller sum 50 and
  98 from 50; lengths seven and eight score 98. Top row substitutions nine
  columns apart miss at lengths four through six; length seven misses below
  smaller sum 51 and scores 97 from 51, while length eight scores 97.
  Other far-key geometries retain the inherited gate. These are measured
  buckets, not a claim about the native distance formula or its unmeasured
  boundaries. Panel K holds missing email, different email, different first
  name and phone-only controls across positions and neighbor geometries.
  Panels J and K compare exact confidence, membership and save decisions
  without any new residuals; all 4992 comparisons agree with zero false matches.
  Predictions were frozen before native capture; the fresh 30-day scratch org
  was deleted and deletion verified. Three-letter same-row adjacent swaps score 98 through sum 29 and 99
  from 31. Five-letter terminal rotations in the one-row/one-column group
  score 98 through sum 35 and 99 from 39; vertical controls score 98 and
  wide same-row controls score 97. Wide rotations below keyboard score 94
  miss. Eight-letter nonphonetic five-column terminal swaps on the same or
  neighboring rows miss through sum 44 and score 97 from 46 through 62.
  Other geometries and higher sums retain the ordinary keyboard gate.
  Case, accents, hyphens and a shared leading token are normalized for the
  additional surname comparison. Panel H includes an ordinary same-key
  eight-letter swap at sum 46 that scores 97 with an exact first name and
  shared phone; it does not need the supplementary email path.
  All captured membership decisions now agree. Nine four-to-five pairs still
  differ by one confidence point in both directions for other trailing
  letters. The tests list each native and current grade exactly. The older
  short-edit holdout now compares the entire observation exactly, including
  the previously differing Metaphone and p-extension inputs.
  Lead probes also cover corroborated and uncorroborated initial and surname
  pairs with distinct companies. A second custom Contact phone rule captured
  a filter-field change into and out of scope: both edits saved with an
  unchanged phone, while a changed-phone upsert into a match was rejected.
  The underlying Salesforce distance function and wider input range remain open.
- Street: native scores the street sections multiplicatively (a different
  suffix word 91, a different street number 95); Glade matches the pairs the
  custom Street method matches and approximates the confidence.
- Phone: on the Account phone panels, the website, ZIP and street also supply
  matching paths. Missing records on those panels cannot all be attributed to
  the phone comparison. Identical international and extension strings in the
  Contact save fixture return no match with only the names as corroboration.
- Standard phone confidence: the Contact and Lead phone item scores 100 for
  the same national number, 91 when both numbers carry one country digit and
  only the line differs, and 88 for any other line difference; the rule
  confidence is the mean with the two exact names (and a present Lead
  company): 96 and 97 for 88, 97 and 98 for 91. Records that each carry an
  email address keep 97 and 98 for every line difference
  (`Field.Contact.Phone`, `Field.Lead.Phone`). `phone_city_zip` N5.spt1 and
  N5.slt1 are exact.
- Standard Person Account matching rule: not activatable by metadata deploy,
  so not captured; unenforced in Glade.
- Raw (below threshold) item scores for Company Name and Title count 50 in an
  OR rule's confidence, and the Keyboard Distance score of two unlike last names
  is approximate (OR rules report 0.2 to 0.4 points of confidence error).
- Syllable Alignment: only the two-word second-word rule and the one-word plural
  are modeled; first-word variants, three-word names, vowel substitutions and
  the varying plural confidences are misses.

## Custom rule panels

Fixture: `internal/apextest/testdata/conformance/duplicate_fuzzy_panels/`
(`FuzzyPanel.cls`, the matching and duplicate rule metadata, `cases.json`,
`org.tsv`, `RUN-FACTS.txt` and `residuals.tsv`); test
`TestDuplicateFuzzyPanelsNativeRoutes`. Each case inserts one record per value
and runs `Datacloud.FindDuplicates` on an unsaved copy of every value, so a
panel of n values holds n x n ordered pairs. Two fresh 30-day scratch orgs, API
62.0 and 67.0, anonymous Apex and `@IsTest`, two repeats: all 160
observations agree across API, route and repeat. The test replays the same
Apex on both Glade routes and compares every pair and confidence exactly;
`residuals.tsv` pins each pair where Glade differs, with both the native and
the Glade value. These panels informed the model and are not an independent
validation of the current implementation.

Street suffix and title changes are limited to the following shapes:

- Street, custom rules only. Fixtures `testdata/conformance/fuzzy_street_c4`,
  `fuzzy_street_c5`, `fuzzy_street_c8` and `fuzzy_street_c9` under
  `internal/dml` (API 62.0 and 67.0, both routes, two repeats, all identical). The standard rules keep the
  shared scorer.
  - Cleaning: a line break is deleted when the street is scored, not turned
    into a space (`Rd\nSuite 200` is `RdSuite 200`; a comma before the break
    goes with it); the match key reads it as a word separator. A number sign,
    slash or colon is deleted where it stands (`Ave#5`, `Ave/5` and `Ave:5` are
    `Ave5`; `Ave #5` is `Ave 5`). A hyphen stays. Accented letters read as their
    base letters (`Borüden` is `Boruden`, the same for ö, ä, é, è, ñ, ç, å, ê,
    í and the capitals). Every other character outside ASCII is deleted: ß, ø,
    æ, ł, đ, œ, þ, ð, я, α, 中 and the signs –, ’, ° and €. `Fußweg 5` is `Fuweg 5`
    and `Hauptstraße Süd` scores as `Hauptstrae Sud`. The words zero to nineteen
    and twenty to ninety are digits (`Suite Five` is `Suite 5`, `Ten St` is `10
    St`); `Hundred`, `Thousand`, `Million`, `Eleventh` and `Twentieth` are not.
  - Match key, checked first even when two addresses clean to one string
    (`Calder Rd\nSuite 200` finds nothing in `Calder RdSuite 200`): the first
    five characters of each of the first two words that are not stop words,
    hyphens removed. `7 Morven Rd` finds `7 Morvex Rd` and not `7 Morvxn Rd`;
    `12345 Morven Rd` finds `123456 Morven Rd` and not `12346 Morven Rd`. Stop
    words are the known suffix words, the suite designators, the letters `A C E
    F I K O P U Y Z`, `SE`, `De La Et And The`, `Rue`, `Str Sts Boul Avenu Pk
    Pky Pkway Ctr Cres Rte Rt`, and 110 of 216 probed street words (`Park`,
    `Hill`, `Orchard`, `Plaza`, `Court`, `Way`, `Calle`, `Via`, ...; the list is
    `streetKeyStopWords`). 74 more suffix spellings were probed one word per
    pair (`qj1` to `qj5`); the ten above are stop words and the rest are not.
    With `Park` and `Ln` left out, `101 Park Ln W` keys on `101` and `W`: it
    finds `101 W Park Ln` and not `101 Park Ln N`, while `121 Tully Ln W` finds
    `121 Tully Ln N`. A PO box keys on its number. Native never rejects a pair
    for its last word or for a low name similarity apart from this key:
    `2 Harbor Lp` and `2 Harbour Loop` differ in the key (`Harbor` is a stop
    word), `1 Infinite Lp` and `1 Infinity Loop` share it and match at 82.
  - Decision: with equal keys a pair matches when the section score reaches 80
    or the edit similarity of the cleaned addresses reaches 87, whatever the
    section score. Four edits in 31 characters (87.10) match; three in 23
    (86.96) do not.
  - Confidence: `(S + E) / 2`, rounded half up. The edit similarity is 0 when
    the edit distance equals the candidate's length: `15 Elm` finds `15 Elm
    Drive` at 50 and the reverse pair reports 75; `77 Wren St` finds `77 Wren
    Str` at 60 and the reverse reports 78. A street number on one side only
    counts as a mismatch. An identical street reports `(S + 100) / 2` (`Test
    Street` 88).
  - Sections: a directional between the number and a name with a known suffix
    is dropped from the name (`101 N Park Ln` finds `101 Park Ln N` at 85);
    other single letters stay in it. After a known suffix the street may carry
    nothing, a directional (a compass word counts), or one suite value, with or
    without `Suite`, `Ste`, `Apt`, `Unit`, `Rm`, `Bldg`, `Fl`, `Floor`, `Room`,
    `Building` or `Apartment`. A suite value is a number, a number and one
    letter, one letter, a letter and a number, or either with one hyphen
    (`5`, `5A`, `A`, `A5`, `5-A`, `A-5`), under Match Blank Fields as well as
    NullNotAllowed. Anything else after the word (`Suite 5th`, `Suite 5 5`,
    `Suite AB`, `Blvd Jr`, `Fl 5th`) means that word is not a suffix: the next
    known suffix word is tried, and with none every word belongs to the name.
    `Level`, `Lot`, `Dept`, `No`, `PMB`, `Stop`, `Trailer` and `Space` are not
    designators. `Aly` and `Alley` are one suffix (93); `Lp`, `Loop`, `Pkwy`,
    `Parkway` and the other unlisted spellings are name words. `Rue` is a word
    of the name: `7 Rue Royale` finds `7 Rue Royal` at 92.
  - Streets that do not split: a word of letters then a digit (`Ave#5`,
    `Box410`, `RR4`) that is not a suite value, a hyphen outside a number range
    or a suite value (`72-A`, `Vane-Oak`), a number before the last word of a
    street that does not start with one (`Suite 9 73 Morrow Rd`, `RR 4 Box 22`),
    a rural route (`RR 4`, `R R 4`, `Rural Route 4`), or a name of four words or
    more after the number, a leading directional and the suffix (`100 N Martin
    Luther King Jr Blvd`, `14 Hawthorne Meadow Blvd Jr`, `Morven Tully Harbor
    Tower`; `100 N Martin Luther King Blvd` has three and splits). Such a street
    reports 100 for itself. As the stored value it scores the edit similarity
    alone; as the candidate against a split street its section score is 0.
    `Ave #5` finds `Ave#5` at 93 and `Ave#5` finds `Ave #5` at 47. These pairs
    match only at edit similarity 87.
  - PO box: `PO Box N`, `P.O. Box N`, `P O Box N` and `POBox N` are one address;
    `PO Box410` scores as a whole string against them (90 both ways). `POB`,
    `Post Office Box`, `Box` and `POBox52` are not PO boxes. A street that is
    only a suffix word, `PO Box` and `PO Box A` match nothing.
  - Measured, every ordered pair of every panel that has a street on both
    sides (`fuzzy_street_c4`, `c5` and `c8`): the fit panels agree on 66,023 of
    66,023 pairs and the holdout panels on 23,876 of 23,876 (0 misses, 0 extra
    matches, 0 differing confidences in either set). The previous scorer
    (`fc2dfc7863`) disagreed on the same pairs with 1,356 misses, 1,366 extra
    matches and 956 confidences in the fit panels and 368 misses, 686 extra
    matches and 491 confidences in the holdout panels. Row by row, 115,057 pairs
    compared with the previous scorer: 5,223 changed, all to the native value;
    0 regressions and 0 new false matches. The shared scorer the standard rules
    call returns the same verdict and score on all 68,965 pairs it was run on.
  - Holdout: `hu1` began as a holdout panel and informed a rule, so it counts
    as fit. The `Orchard Ln` holdout panel `hd1` scored 114 extra matches
    against an earlier model; `Orchard` was then probed as one word pair
    (`ng7`) and the panel scored again unread. The 42 twin panels of `c8`
    (`RUN-FACTS.txt` lists the 31 that stay holdout) were each scored once,
    against the model as it stood: 31 agreed on every pair. The other eleven (`hqg2`,
    `hqg3`, `hqg4`, `hqs1` to `hqs6`, `hqv1`, `gqd2`) disagreed on the words
    after a suffix (a suite value such as `5th`, `Fl 5th` or `Level 5`), on
    number words, or on the section score of a whole candidate under Match
    Blank Fields. They were read, new panels captured, and they are stored as
    fit. `fuzzy_street_c4` hs1 and `fuzzy_street_c5` mj1, never read, agree on
    every pair now.
  - Review panels, `fuzzy_street_c9` (three captures in shapes the earlier
    fixtures do not hold). Eight panels were scored once against `2a69555ed7`
    and shaped no rule: names of four words and more, compound number words,
    floor words in other positions, two differing name words, streets with no
    number under both blank modes, more suite values, `Aly`, `Calle` and `Via`.
    They agree on 4,340 of 4,348 pairs (0 misses, 8 extra matches, 0
    confidences); the previous scorer (`fc2dfc7863`) had 186 misses, 204 extra
    matches and 55 confidences there. Four panels (`r1`, `r1m`, `r7`, `r7m`)
    were read and shaped three rules:
    - A street name on one side only is a mismatch, as a street number is.
      `12` finds neither `12 St` nor `12 Park` nor `12 Park St`; `12 Park`
      finds `12 Park St` at 85, because a suffix on one side is left out.
    - A street with no key word finds nothing, itself included: `Park St`, and
      an address with no ASCII letter or digit (`港区`).
    - A street of one word that is not a number is the name: `3F` reports 86
      for itself, 100 under Match Blank Fields.

    The deletion of characters outside ASCII holds for whole scripts: natively
    `Тверская 5` finds `Арбатская 5` and `5` at 100 and not `Тверская 7`;
    `東京都港区1-2-3` finds `大阪市北区1-2-3`. Two addresses in such a script
    are one street when their ASCII digits and letters agree. The twins of the
    four panels (`g1`, `g1m`, `g7`, `g7m`: other scripts, words and numbers)
    were scored once after the three rules were frozen and agree on all 2,034
    pairs.
  - Ordinal and compound number words, `fuzzy_street_c10` (two captures; the
    eight false matches that `fuzzy_street_c9/residuals.tsv` pinned are closed
    and the file is gone). Three rules, read from the 1,522 native matches of
    the fit panels:
    - A name of whole numbers only before the suffix (`500 Twenty One St`,
      `500 21 Ave`, `1 Hundred 1 St`), or a directional before a name that
      opens with a number and goes on to numbers or ordinals (`300 E Twenty
      Third St`), is compared as one string: it matches only at edit
      similarity 87, as a stored value it reports that similarity, and as a
      candidate against a split street half of it. `500 Twenty One St` finds
      neither `500 Twenty One Street` nor `500 Twenty First St` (no suffix
      spelling matches it, `Rd` and `Road` included) while `500 Twenty First
      Street` finds `500 Twenty One Street` at 88; `300 E Twenty Third
      Street` finds `300 Twenty Third Street` at 45 and the reverse at 89.
      A name that ends in an ordinal splits as any other street (`500 Twenty
      First St` finds `500 Twenty First Street` at 88).
    - Whole numbers after the street number belong to the number when a word
      follows: `500 Twenty First Street` finds `500 Twenty Tenth Street` at
      80 and `500 Twenty Third Street` finds `500 Twenty Second Street` at 75,
      not at 86 and 84.
    - An ordinal first word is no street number: `3rd Ave`, `Third Ave` and
      `101st St` report 88 for themselves, as a street with no number does.

    Twelve panels (`h1`, `h2`, `h3`, `h6`, `h7`, `h8` and their Match Blank
    twins: other numbers and names) were scored after the model's predictions
    for them were frozen in a file (hash in `RUN-FACTS.txt`) and agree on every
    pair. The directional rule is read for `E`, `W`, `N` and `East` before a
    number name; other directionals before one are not measured. Not measured
    either: a number name with no suffix word, a suffix word before a tail.
  - Not measured: Match Blank Fields for a street that is whole as the stored
    value, a blank street number with a name of four words, non-ASCII white
    space, and suite values longer than the shapes in `qs1` to `qs7`.
- Title: when the shorter title is an exact-word subset of a six-word title,
  two to five shared words score 89, 93, 95 and 98; five shared words out of
  order score 96. An exact-word subset candidate can match a five-word title
  with one shared word, or a five- or six-word title with two out-of-order
  shared words. A three-word title with two exact shared words in order also
  scores 89 against a six-word title. These apply only when neither title
  holds a stop word or a repeated word. Other shapes retain the existing
  confidence table.

First Name and Company Name retain the origin/main scorers. The panel fixture
records native results for these methods, including cases that baseline matches
falsely. Those cases remain open while rule-level matching is investigated.
