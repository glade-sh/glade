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
	if x == y {
		if _, lone := knownStreetSuffixes[x]; lone || x == "pobox" || streetBarePOBox.MatchString(x) {
			// NATIVE cs1, cp1, nr1: a street that is only a suffix word, "PO
			// Box" with no number and "PO Box A" find nothing.
			return false, 0
		}
		if streetWhole(x) {
			return true, 100
		}
		// NATIVE cs1: equal streets still average the section score with 100
		// (Test Street 88, Test 86, Calder Rd 500 90).
		p := parseStreetFor(a, true, !matchBlanks)
		return true, (streetSectionScore(p, p, matchBlanks, streetCandidateSimilarity, true) + 100) / 2
	}
	if !streetCustomKeysAgree(x, y) {
		return false, 0
	}
	similarity := streetCandidateSimilarity(x, y)
	gate := similarity >= streetSimilarityGate-1e-9
	wholeX, wholeY := streetWhole(x), streetWhole(y)
	if wholeX || wholeY {
		confidence := similarity
		if !wholeY && !streetPOBox.MatchString(y) {
			confidence = similarity / 2
		}
		return gate, confidence
	}
	pa, pb := parseStreetFor(a, true, !matchBlanks), parseStreetFor(b, true, !matchBlanks)
	sections := streetSectionScore(pa, pb, matchBlanks, streetCandidateSimilarity, true)
	confidence := (sections + similarity) / 2
	if gate {
		return true, confidence
	}
	if streetTwoEdits(pa, pb) {
		return false, confidence
	}
	if pa.number == "" && pb.number == "" && pa.name != pb.name && editSimilarity(pa.name, pb.name) < 80 {
		return false, confidence
	}
	return sections >= 80, confidence
}

// streetSimilarityGate is the edit similarity of the cleaned addresses at
// which two streets with one match key match whatever their section score.
// NATIVE fuzzy_street_c5/org.tsv nt1: 4 edits in 31 characters (87.10) match,
// 3 in 23 (86.96) and 2 in 15 (86.67) do not.
const streetSimilarityGate = 87

var (
	streetLettersThenDigit = regexp.MustCompile(`^[a-z]+[0-9]`)
	streetNumberRange      = regexp.MustCompile(`^[0-9]+-[0-9]+$`)
	streetBarePOBox        = regexp.MustCompile(`^po box( [a-z])?$`)
	streetPOBoxKey         = regexp.MustCompile(`^(?:p ?o box ?|pobox )([0-9]+)$`)
)

// streetWhole reports a cleaned street the custom method compares as one
// string: a word of letters followed by a digit ("Ave#5", "Box410", "RR4"), a
// hyphen in a word that is not a number range ("72-A", "Vane-Oak", but not
// "72-74"), a number before the last word of a street that does not start
// with one ("Suite 9 73 Morrow Rd", "RR 4 Box 22", "PO Box 52 Main"), or one
// more word after the known suffix (streetLoneWordAfterSuffix), or four words
// with no number and no suffix (streetLongName). NATIVE
// fuzzy_street_c5/org.tsv nh1, nn1, ns1, nr1: these report 100 for an equal
// street and the edit similarity for a near one. "PO Box 52" is a PO box and
// not whole.
func streetWhole(x string) bool {
	if streetPOBox.MatchString(x) {
		return false
	}
	words := strings.Fields(x)
	if streetLoneWordAfterSuffix(words) || streetLongName(words) {
		return true
	}
	for i, w := range words {
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

// streetLoneWordAfterSuffix reports one more word after the known suffix that
// is neither a directional nor a suite value ("77 Wren St Xq", "14 Dana Blvd
// Jr"). NATIVE cu3, nj1: such a street reports 100 for itself and finds no
// street that lacks the word or has it before the suffix.
func streetLoneWordAfterSuffix(words []string) bool {
	if len(words) > 0 && words[0][0] >= '0' && words[0][0] <= '9' {
		words = words[1:]
	}
	for i := 1; i < len(words)-1; i++ {
		if _, ok := knownStreetSuffixes[words[i]]; ok {
			last := words[i+1]
			return i+2 == len(words) && len(last) > 1 && isAlphaWord(last) && !streetDirections[last] && !streetLongDirections[last]
		}
	}
	return false
}

// streetLongName reports a street of four or more words with no street number
// and no known suffix; a trailing number is not counted. NATIVE
// fuzzy_street_c5/org.tsv nk2, nm1: "Morven Tully Harbor Tower" reports 100 for
// itself and 96, the edit similarity, for "Morven Tully Harbor Towers", while
// "Morven Tully Tower" reports 86 and 81.
func streetLongName(words []string) bool {
	if n := len(words); n > 0 && digitsOnly.MatchString(words[n-1]) {
		words = words[:n-1]
	}
	if len(words) < 4 || words[0][0] >= '0' && words[0][0] <= '9' {
		return false
	}
	for _, w := range words {
		if _, ok := knownStreetSuffixes[w]; ok {
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
var streetKeyStopWords = streetWordSet("a c e f i k o p u y z se de la et and the av avn tower towers " +
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
	if slices.Equal(streetCustomKey(x), streetCustomKey(y)) {
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
