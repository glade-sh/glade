package dml

import "strings"

// FuzzyRawConfidence is the confidence a custom rule's item contributes when
// the pair did not match. Native rows show that an OR group and an AND group
// still average in the item's own score ("(1 AND 2) OR 3" with a different
// phone number reports the mean of the name group and a phone score of 40,
// where the sections that agree are the blank country and the exchange; an
// Exact item that differs counts 50). The scores below follow the matching
// scorers; Company Name and Title have no recovered raw score and count as an
// algorithm score of zero, the Exact value.
func FuzzyRawConfidence(method, left, right string, matchBlanks bool) float64 {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" || right == "" {
		return 50
	}
	left, right = capFuzzyInput(left), capFuzzyInput(right)
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "firstname":
		x, y := foldText(left), foldText(right)
		if x == y || nameVariantOf(x, y) {
			return 100
		}
		return averageWith100(100 * jaroWinkler(x, y))
	case "lastname":
		x, y := strings.ReplaceAll(foldText(left), "'", ""), strings.ReplaceAll(foldText(right), "'", "")
		if x == y {
			return 100
		}
		score, _ := keyboardSimilarity(x, y)
		return averageWith100(max(0, score))
	case "city":
		return averageWith100(editSimilarity(foldText(left), foldText(right)))
	case "phone":
		ca, aa, ea, la, okA := phoneParts(left)
		cb, ab, eb, lb, okB := phoneParts(right)
		if !okA || !okB {
			return 0
		}
		score := 0.0
		for _, section := range []struct {
			x, y   string
			weight float64
		}{{ca, cb, 10}, {aa, ab, 50}, {ea, eb, 30}, {la, lb, 10}} {
			if section.x == section.y {
				score += section.weight
			}
		}
		return score
	case "zip":
		if ok, _ := fuzzyZip(left, right, matchBlanks); ok {
			return 100
		}
		// The five digits differ; the empty extensions agree and weigh 10.
		return 10
	case "street":
		_, confidence := fuzzyStreet(left, right, matchBlanks)
		return confidence
	}
	return 50
}
