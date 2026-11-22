package dml

import (
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

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

// duplicateTermVerdicts memoizes one fuzzy term only for the duration of one
// scan. Unusual field representations use the original scorer. The filter
// only skips records the scorer would reject, so turning it off never changes
// an outcome. Callers use it only for fuzzy terms (an Exact verdict costs no
// more than a memo lookup), and it switches off once the memo is full and
// lookups mostly missed (highly distinct values), so distinct data pays at
// most one bounded warm-up and no per-scan allocation beyond the pooled memo.
type duplicateTermVerdicts struct {
	term   int
	values map[string]bool
	pooled *map[string]bool
	hits   int
	misses int
	off    bool
}

const maxDuplicateTermMemoValues = 96

var duplicateTermMemoPool = sync.Pool{New: func() any {
	values := make(map[string]bool, maxDuplicateTermMemoValues)
	return &values
}}

func newDuplicateTermVerdicts(term int) duplicateTermVerdicts {
	return duplicateTermVerdicts{term: term}
}

// mayMatch reports false only when the term's scorer rejects the record. It
// stays small enough to inline so a memo that switched off costs one test.
func (v *duplicateTermVerdicts) mayMatch(p *preparedCustomMatch, existing storage.Record) bool {
	return v.off || v.lookup(p, existing)
}

func (v *duplicateTermVerdicts) lookup(p *preparedCustomMatch, existing storage.Record) bool {
	item := &p.items[v.term]
	value, ok := existing.Fields[item.field]
	if !ok || value.Kind != storage.ValueString {
		return true
	}
	if v.values == nil {
		v.pooled = duplicateTermMemoPool.Get().(*map[string]bool)
		v.values = *v.pooled
	} else if matched, ok := v.values[value.String]; ok {
		v.hits++
		return matched
	}
	if len(v.values) >= maxDuplicateTermMemoValues {
		if v.hits < v.misses {
			v.off = true
			return true
		}
		matched, _ := p.verdict(v.term, existing)
		return matched
	}
	v.misses++
	matched, _ := p.verdict(v.term, existing)
	v.values[value.String] = matched
	return matched
}

// release returns the memo to the pool at the end of a scan.
func (v *duplicateTermVerdicts) release() {
	if v.pooled == nil {
		return
	}
	clear(v.values)
	duplicateTermMemoPool.Put(v.pooled)
	v.values, v.pooled = nil, nil
}
