package dml

// Custom matching rule items score through the shared fuzzy scorers plus the
// extensions below. Each extension is read from the duplicate_fuzzy_panels
// fixture (Datacloud.FindDuplicates on custom rules, API 62.0 and 67.0,
// anonymous and @IsTest, two repeats, all identical) and is gated to the value
// shapes those rows cover. The standard rules call the shared scorers directly,
// so none of this changes them.
var customFuzzyScorers = map[string]fuzzyScorer{
	"street": customStreet,
}

// customStreet matches a street whose last word is a known suffix against the
// same street ending in an unknown word. NATIVE: such pairs match when the
// edit similarity of the cleaned addresses is at least 87.5 ("St" and "Strt",
// "Drive" and "Drv", "Hwy" and "Pkwy") and are kept apart at 85.71 and below
// ("St" and "Sreet", "Rd" and "Drv"), although the weighted section score is
// under 80 (74 to 78). The confidence is the shared scorer's.
func customStreet(a, b string, matchBlanks bool) (bool, float64) {
	return fuzzyStreetWithCustom(a, b, matchBlanks, true)
}
