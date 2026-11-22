package dml

import "github.com/glade-sh/glade/internal/storage"

// requiredDuplicateTerms selects terms which must match for every supported
// successful outcome. Unknown filter outcomes retain the normal scan.
func requiredDuplicateTerms(p *preparedCustomMatch) (terms [2]int, count int) {
	if p == nil || p.plan == nil {
		return terms, 0
	}
	necessary := (1 << p.n) - 1
	hasMatch := false
	for mask, outcome := range p.plan.table {
		switch outcome {
		case filterTrueSupported:
			necessary &= mask
			hasMatch = true
		case filterFalseSupported:
		default:
			return terms, 0
		}
	}
	if hasMatch {
		for i := 0; i < p.n; i++ {
			if necessary&(1<<i) != 0 {
				terms[count] = i
				count++
				if count == len(terms) {
					break
				}
			}
		}
	}
	return terms, count
}

// duplicateTermVerdicts memoizes one term only for the duration of one scan.
// Unusual field representations use the original scorer.
type duplicateTermVerdicts struct {
	term   int
	values map[string]bool
}

const maxDuplicateTermMemoValues = 96

func (v *duplicateTermVerdicts) mayMatch(p *preparedCustomMatch, existing storage.Record) bool {
	item := p.items[v.term]
	value, ok := existing.Fields[item.field]
	if !ok || value.Kind != storage.ValueString {
		return true
	}
	if matched, ok := v.values[value.String]; ok {
		return matched
	}
	matched, _ := p.verdict(v.term, existing)
	if len(v.values) < maxDuplicateTermMemoValues {
		v.values[value.String] = matched
	}
	return matched
}
