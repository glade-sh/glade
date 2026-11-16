package dml

import (
	"regexp"
	"slices"
	"strings"
)

// Custom matching rule items score through the shared fuzzy scorers, except
// the methods below, which have their own scorer read from native
// Datacloud.FindDuplicates rows on custom rules (API 62.0 and 67.0, anonymous
// and @IsTest, two repeats, all identical). The standard rules call the shared
// scorers directly, so none of this changes them.
var customFuzzyScorers = map[string]fuzzyScorer{
	"street": customStreet,
}

// customStreet is the custom Street method. NATIVE fuzzy_street_c4/org.tsv and
// fuzzy_street_c5/org.tsv (Datacloud.FindDuplicates, API 62.0 and 67.0, both
// routes, two repeats, all identical): two streets are compared only when their
// match keys agree (streetCustomKey). A pair then matches when the weighted
// section score reaches 80, or when the edit similarity of the cleaned
// addresses reaches 87 (nt1: 87.10 matches, 86.96 does not), whatever the
// section score. The confidence averages the two.
//
// A street the parser cannot split (streetWhole) has no sections. As the
// stored value it scores the edit similarity alone; as the candidate against a
// split street its section score is 0 (nh1: "Ave #5" finds "Ave#5" at 93 and
// "Ave#5" finds "Ave #5" at 47). Such a pair matches only at 87.
func customStreet(a, b string, matchBlanks bool) (bool, float64) {
	x, y := streetCleanFor(a, true), streetCleanFor(b, true)
	// NATIVE fuzzy_street_c8/org.tsv qa1, qa3: the key comes first, so two
	// addresses that clean to one string still find nothing when a line break
	// moves a word across the key ("Calder Rd\nSuite 200" and "Calder
	// RdSuite 200"). The key reads a break as a word separator.
	kx, ky := x, y
	if strings.ContainsAny(a, "\r\n") {
		kx = streetKeyClean(a)
	}
	if strings.ContainsAny(b, "\r\n") {
		ky = streetKeyClean(b)
	}
	if !streetCustomKeysAgree(kx, ky) {
		return false, 0
	}
	pa, pb := parseStreetFor(a, true), parseStreetFor(b, true)
	wholeX, wholeY := streetWhole(x, pa), streetWhole(y, pb)
	if x == y {
		if _, lone := knownStreetSuffixes[x]; lone || x == "pobox" || streetBarePOBox.MatchString(x) {
			// NATIVE cs1, cp1, nr1: a street that is only a suffix word, "PO
			// Box" with no number and "PO Box A" find nothing.
			return false, 0
		}
		if wholeX {
			return true, 100
		}
		// NATIVE cs1: equal streets still average the section score with 100
		// (Test Street 88, Test 86, Calder Rd 500 90).
		return true, (streetSectionScore(pa, pa, matchBlanks, streetCandidateSimilarity, true) + 100) / 2
	}
	similarity := streetCandidateSimilarity(x, y)
	gate := similarity >= streetSimilarityGate-1e-9
	if wholeX || wholeY {
		confidence := similarity
		if !wholeY && !streetPOBox.MatchString(y) {
			// The candidate has no sections: its section score is 0, or 20
			// of 70 where Match Blank Fields counts two blank street
			// numbers (NATIVE fuzzy_street_c8/org.tsv gqd2: "Waldweg2 5"
			// finds "Waldweg 5" at 59 there and 45 under NullNotAllowed).
			sections := 0.0
			if matchBlanks && pa.number == "" && pb.number == "" {
				sections = 100.0 * 20 / 70
			}
			confidence = (sections + similarity) / 2
		}
		return gate, confidence
	}
	sections := streetSectionScore(pa, pb, matchBlanks, streetCandidateSimilarity, true)
	confidence := (sections + similarity) / 2
	return gate || sections >= 80, confidence
}

// streetSimilarityGate is the edit similarity of the cleaned addresses at
// which two streets with one match key match whatever their section score.
// NATIVE fuzzy_street_c5/org.tsv nt1: 4 edits in 31 characters (87.10) match,
// 3 in 23 (86.96) and 2 in 15 (86.67) do not.
const streetSimilarityGate = 87

// streetWholeNameWords is the number of name words from which a street has no
// sections.
const streetWholeNameWords = 4

var (
	streetLettersThenDigit = regexp.MustCompile(`^[a-z]+[0-9]`)
	streetNumberRange      = regexp.MustCompile(`^[0-9]+-[0-9]+$`)
	streetBarePOBox        = regexp.MustCompile(`^po box( [a-z])?$`)
	streetRuralRoute       = regexp.MustCompile(`^(?:rr|r r|rural route) [0-9]+$`)
	streetPOBoxKey         = regexp.MustCompile(`^(?:p ?o box ?|pobox )([0-9]+)$`)
)

// streetWhole reports a cleaned street, with its parts, that the custom method
// compares as one string: a word of letters followed by a digit ("Ave#5", "Box410", "RR4"), a
// hyphen in a word that is not a number range ("72-A", "Vane-Oak", but not
// "72-74"), a number before the last word of a street that does not start
// with one ("Suite 9 73 Morrow Rd", "RR 4 Box 22", "PO Box 52 Main"), a rural
// route ("RR 4", "Rural Route 4") or a name of four words. NATIVE
// fuzzy_street_c5/org.tsv nh1, nn1, ns1, nr1, fuzzy_street_c8/org.tsv qb1, qc1:
// these report 100 for an equal street and the edit similarity for a near
// one. "PO Box 52" is a PO box and not whole.
func streetWhole(x string, parsed streetParts) bool {
	if streetPOBox.MatchString(x) {
		return false
	}
	if strings.HasPrefix(x, "r") && streetRuralRoute.MatchString(x) {
		// NATIVE fuzzy_street_c8/org.tsv qc1: "RR 4", "R R 4" and "Rural
		// Route 4" report 100 for themselves, "HC 4" and "Route 4" 88.
		return true
	}
	// NATIVE fuzzy_street_c5/org.tsv nk2, nm1, fuzzy_street_c8/org.tsv qb1,
	// qb2, qb3, qb4: a name of four words is whole wherever the fourth word
	// stands ("100 N Martin Luther King Jr Blvd", "100 Martin Luther King Sr
	// Blvd", "Morven Tully Harbor Tower", "14 Hawthorne Meadow Blvd Jr"),
	// while "100 N Martin Luther King Blvd" (three name words after the
	// directional) is split.
	if len(strings.Fields(parsed.name)) >= streetWholeNameWords {
		return true
	}
	words := strings.Fields(x)
	if streetNumericName(words) {
		return true
	}
	for i, w := range words {
		if i == len(words)-1 && parsed.suite != "" && w == parsed.suite {
			// NATIVE fuzzy_street_c8/org.tsv qs1: the suite value is not a
			// word of the name ("Suite A5", "Suite 5-A" split).
			continue
		}
		if streetLettersThenDigit.MatchString(w) {
			return true
		}
		if strings.Contains(w, "-") && !streetNumberRange.MatchString(w) {
			return true
		}
		if i > 0 && i < len(words)-1 && (words[0][0] < '0' || words[0][0] > '9') && strings.ContainsAny(w, "0123456789") {
			return true
		}
	}
	return false
}

// streetOrdinalWord reports digits followed by st, nd, rd or th.
func streetOrdinalWord(w string) bool {
	n := len(w)
	if n < 3 {
		return false
	}
	switch w[n-2:] {
	case "st", "nd", "rd", "th":
		return streetDigits(w[:n-2])
	}
	return false
}

// streetDigits reports a non-empty word of ASCII digits (digitsOnly without
// the regular expression, on the scoring path of every pair).
func streetDigits(w string) bool {
	for i := 0; i < len(w); i++ {
		if w[i] < '0' || w[i] > '9' {
			return false
		}
	}
	return w != ""
}

// streetNumericName reports a cleaned street (number words already digits)
// that starts with a street number and ends with a known suffix word, where
// every word before the suffix is a whole number or "Hundred" and the last is
// a number ("500 20 1 St" for "Twenty One", "500 21 Ave", "1 Hundred 1 St" for
// "One Hundred One"), or where a
// directional is followed by a name that opens with a number and goes on to
// numbers or ordinals ("300 E 20 3rd St" for "E Twenty Third"). NATIVE
// fuzzy_street_c10/org.tsv: such a street is compared as one string. As the
// stored value it scores the edit similarity alone and as the candidate
// against a split street half of it ("300 E Twenty Third Street" finds "300
// Twenty Third Street" at 45, the reverse at 89); it matches only at 87, so
// "Twenty One St" finds neither "Twenty One Street" nor "Twenty First St"
// while "Twenty One Street" finds "Twenty First Street" at 88. A name that
// ends in an ordinal ("23rd", "20 3rd", "E 23rd") splits as any other street.
func streetNumericName(words []string) bool {
	if len(words) < 3 || words[0][0] < '0' || words[0][0] > '9' {
		return false
	}
	rest := words[1:]
	directional := streetDirections[rest[0]] || streetCompassWords[rest[0]]
	if directional {
		rest = rest[1:]
	}
	last := len(rest) - 1
	if last < 1 {
		return false
	}
	if _, ok := customStreetSuffixes[rest[last]]; !ok {
		return false
	}
	name := rest[:last]
	// Every word of the name is a number: a word then a bare number ("500
	// Main 5 St", "100 Vane 7 St") is another shape and keeps the base rules.
	allNumbers := true
	for _, w := range name {
		if !streetDigits(w) && w != "hundred" {
			allNumbers = false
			break
		}
	}
	if allNumbers && streetDigits(name[last-1]) {
		return true
	}
	if !directional || !streetDigits(name[0]) {
		return false
	}
	for _, w := range name[1:] {
		if !streetOrdinalWord(w) {
			return false
		}
	}
	return true
}

// streetKeyStopWords are left out of the match key, with the known suffix
// words, the suite designators and "Rue". NATIVE:
//   - single letters: fuzzy_street_c4/org.tsv cl1, cl2 ("100 A Main St" finds
//     "100 Main St" and "100 C Main St"; "100 B Main St" finds neither);
//   - "de", "la", "et", "and", "the": duplicate_fuzzy_matching/org.tsv ("100
//     De Main St" finds "100 La Main St" and "100 A Main St");
//   - street words: fuzzy_street_c5/org.tsv ng1 to ng12, one row per word.
//     "101 Park Ln W" does not find "101 Park Ln N": with Park and Ln left
//     out the key is the number and the directional. "121 Tully Ln W" finds
//     "121 Tully Ln N" at 96. 110 of 216 probed words behave like Park; the
//     rest ("Lake", "Creek", "River", "Garden", "Valley", "Hills", "Main",
//     "Old", "San", ...) stay in the key. np1 and np3 run the directional
//     panel on Park: "101 Park Ln E", "101 Park Ln SE" and "101 Park Ln" share
//     a key, so "se" is left out like "e";
//   - "av", "avn": np3 "103 Park Ave" finds "103 Park Av" and "103 Park Avn";
//   - "tower", "towers": nq1 "Morven Tower West" does not find "Morven Tower
//     Wests" while "Morven Tully West" finds "Morven Tully Wests"; nk2 "Morven
//     Plaza Tower" finds "Morven Plaza Towers".
var streetKeyStopWords = streetWordSet("a c e f i k o p u y z se de la et and the av avn str sts boul avenu pk pky pkway ctr cres rte rt tower towers " +
	"park hill ridge forest meadow grove glen harbor island mount point view village center " +
	"circle court place square terrace trail way loop run walk row path crossing heights " +
	"highway parkway alley bend cove dale field green hollow key knoll landing manor pass " +
	"port vista ct pl plz sq ter trl pkwy el le les du des von van of via calle avenida paseo " +
	"rua plaza mall estate orchard arcade beach bypass cape causeway common crescent " +
	"expressway extension flat freeway inlet mews mountain passage pines ramp route turnpike " +
	"wall gardens acres estates hts expy fwy tpke weg platz gasse allee chemin impasse cours " +
	"quai viale straat laan plein saint bay")

func streetWordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(words) {
		set[w] = true
	}
	return set
}

// streetCustomKey is the match key: the first five characters of each of the
// first two words that are not stop words, hyphens removed. NATIVE
// fuzzy_street_c5/org.tsv nk1: "7 Morven Rd" finds "7 Morvex Rd" and not "7
// Morvxn Rd"; "12345 Morven Rd" finds "123456 Morven Rd" and not "12346 Morven
// Rd". A PO box keys on its number (cp1: 410, 411 and 4100 never match; nr1:
// "PO Box52" is "PO Box 52" here and "POBox52" is not).
func streetCustomKey(x string) []string {
	if m := streetPOBoxKey.FindStringSubmatch(x); m != nil {
		return []string{"pobox", m[1]}
	}
	var key []string
	words := strings.Fields(x)
	for i, w := range words {
		if i == 1 && words[0] == "po" && strings.HasPrefix(w, "box") {
			// NATIVE nr1: "PO Box52 Main" finds "PO Box 52 Main".
			w = "box"
		}
		w = strings.ReplaceAll(w, "-", "")
		if _, suffix := knownStreetSuffixes[w]; w == "" || suffix || streetKeyStopWords[w] || streetSuiteDesignators[w] || streetFloorDesignators[w] || streetPrefixSuffixes[w] {
			continue
		}
		if r := []rune(w); len(r) > streetKeyLength {
			w = string(r[:streetKeyLength])
		}
		if key = append(key, w); len(key) == 2 {
			break
		}
	}
	return key
}

// streetCustomKeysAgree reports equal match keys. Two streets that differ only
// by spaces also agree when the join is a line break after the street number or
// after a leading suite: NATIVE cj2, ns1,
// sl1: "41\nHarlan Ave" finds "41 Harlan Ave" and "Suite 9\n73 Morrow Rd"
// finds "Suite 9 73 Morrow Rd". Other joins keep the streets apart (cl3 "100
// AB Main St" and "100 A B Main St", nn1 "72 Vane Oak St" and "72 VaneOak
// St", nr1 "RR 4 Box 22" and "RR4 Box 22").
func streetCustomKeysAgree(x, y string) bool {
	kx := streetCustomKey(x)
	if len(kx) == 0 {
		// NATIVE fuzzy_street_c9/org.tsv r1, r7: a street with no key word
		// finds nothing, itself included ("Park St", and an address with
		// no ASCII letter or digit, which cleans to nothing).
		return false
	}
	if slices.Equal(kx, streetCustomKey(y)) {
		return true
	}
	if strings.ReplaceAll(x, " ", "") != strings.ReplaceAll(y, " ", "") {
		return false
	}
	wx, wy := strings.Fields(x), strings.Fields(y)
	glued := streetGluedNumber.MatchString(wx[0]) || streetGluedNumber.MatchString(wy[0])
	if !glued && !(streetSuiteDesignators[wx[0]] && streetSuiteDesignators[wy[0]]) {
		return false
	}
	return true
}
