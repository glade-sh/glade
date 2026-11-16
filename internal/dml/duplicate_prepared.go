package dml

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

// preparedCustomMatch is customDuplicateMatch with the candidate side read
// once per scan instead of once per stored record. For every stored record it
// returns exactly what customDuplicateMatch returns: the item verdicts come
// from the same scorers, the filter verdict from the same evaluator (a truth
// table of evaluateLookupFilterLogic), and the confidence from the same
// formula. It skips only work that cannot change the answer: an item is
// scored only while the filter is undecided and its verdict can still decide
// it, and the raw confidence of an unmatched item is computed only for a pair
// that matched (the only result that reads it).
type preparedCustomMatch struct {
	rule  storage.MatchingRule
	items [maxPreparedItems]preparedCustomItem
	n     int
	plan  *filterPlan
}

type preparedCustomItem struct {
	field       string
	method      string
	exact       bool
	matchBlanks bool
	initialKey  bool
	leftBlank   bool
	// text is the candidate's comparison text: trimmed and cut to the Exact
	// limit for an Exact item, the raw value text for a fuzzy item.
	text string
	// scorer and scored are FuzzyScore's scorer for the method and its
	// trimmed, capped candidate text; first is the candidate side of an
	// initial-keyed First Name item.
	scorer fuzzyScorer
	scored string
	first  firstNameInitialSide
}

// maxPreparedItems bounds the plan (a matching rule has at most ten items);
// a larger rule keeps the pairwise path.
const maxPreparedItems = 10

// filterOutcome encodes an evaluateLookupFilterLogic result.
type filterOutcome int8

const (
	filterFalseUnsupported filterOutcome = iota + 1
	filterFalseSupported
	filterTrueUnsupported
	filterTrueSupported
)

func newFilterOutcome(matched, supported bool) filterOutcome {
	switch {
	case matched && supported:
		return filterTrueSupported
	case matched:
		return filterTrueUnsupported
	case supported:
		return filterFalseSupported
	}
	return filterFalseUnsupported
}

func (o filterOutcome) result() (matched, supported bool) {
	return o == filterTrueSupported || o == filterTrueUnsupported, o == filterTrueSupported || o == filterFalseSupported
}

// filterPlan drives the item scoring of one filter expression. A state holds
// one base-3 digit per item: 0 not scored yet, 1 did not match, 2 matched.
// step[state] is the next item to score (0 to n-1), or the negated outcome when
// every verdict of the items not yet scored gives the same outcome.
type filterPlan struct {
	n     int
	pow3  [maxPreparedItems + 1]int
	step  []int8
	table []filterOutcome // outcome of each full verdict mask (bit i is item i)
}

type filterPlanKey struct {
	expression string
	items      int
	order      string // preferred scoring order, one byte per item index
}

// filterPlans memoizes plans: evaluateLookupFilterLogic is a pure function of
// the expression and the verdicts, so a plan is shared by every scan. The cache
// stops growing at maxFilterPlans entries; later plans are built per scan.
var filterPlans struct {
	sync.Mutex
	plans map[filterPlanKey]*filterPlan
}

const maxFilterPlans = 256

func cachedFilterPlan(expression string, order []int) *filterPlan {
	key := filterPlanKey{expression: expression, items: len(order)}
	var ob [maxPreparedItems]byte
	for i, item := range order {
		ob[i] = byte(item)
	}
	key.order = string(ob[:len(order)])
	filterPlans.Lock()
	plan, ok := filterPlans.plans[key]
	filterPlans.Unlock()
	if ok {
		return plan
	}
	plan = newFilterPlan(expression, order)
	filterPlans.Lock()
	if filterPlans.plans == nil {
		filterPlans.plans = map[filterPlanKey]*filterPlan{}
	}
	if len(filterPlans.plans) < maxFilterPlans {
		filterPlans.plans[key] = plan
	}
	filterPlans.Unlock()
	return plan
}

func newFilterPlan(expression string, order []int) *filterPlan {
	n := len(order)
	plan := &filterPlan{n: n, table: make([]filterOutcome, 1<<n)}
	matches := make([]bool, n)
	for mask := range plan.table {
		for i := range matches {
			matches[i] = mask&(1<<i) != 0
		}
		plan.table[mask] = newFilterOutcome(evaluateLookupFilterLogic(expression, matches))
	}
	plan.pow3[0] = 1
	for i := 1; i <= n; i++ {
		plan.pow3[i] = plan.pow3[i-1] * 3
	}
	plan.step = make([]int8, plan.pow3[n])
	for state := range plan.step {
		known, value := 0, 0
		for i, rest := 0, state; i < n; i, rest = i+1, rest/3 {
			switch rest % 3 {
			case 1:
				known |= 1 << i
			case 2:
				known |= 1 << i
				value |= 1 << i
			}
		}
		unknown := (1<<n - 1) &^ known
		if outcome, decided := plan.constant(value, unknown); decided {
			plan.step[state] = -int8(outcome)
			continue
		}
		// Score the first item in the preferred order whose verdict can still
		// change the outcome. One exists: the outcome is not constant.
		for _, item := range order {
			if unknown&(1<<item) != 0 && plan.relevant(value, unknown, item) {
				plan.step[state] = int8(item)
				break
			}
		}
	}
	return plan
}

// constant reports the outcome shared by every completion of the unknown bits.
func (p *filterPlan) constant(value, unknown int) (filterOutcome, bool) {
	first := p.table[value]
	for sub := unknown; sub != 0; sub = (sub - 1) & unknown {
		if p.table[value|sub] != first {
			return 0, false
		}
	}
	return first, true
}

// relevant reports whether some completion changes outcome with item's bit.
func (p *filterPlan) relevant(value, unknown, item int) bool {
	bit := 1 << item
	rest := unknown &^ bit
	for sub := rest; ; sub = (sub - 1) & rest {
		if p.table[value|sub] != p.table[value|sub|bit] {
			return true
		}
		if sub == 0 {
			return false
		}
	}
}

// fuzzyMethodCostRank orders fuzzy items for scoring, cheapest first; the
// order only decides which items a decided filter skips, never a verdict.
func fuzzyMethodCostRank(method string) int {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "zip", "phone":
		return 1
	case "firstname", "lastname", "city", "title":
		return 2
	}
	return 3 // companyname, street
}

// newPreparedCustomMatch prepares the candidate side of a custom matching
// rule. ok is false for a rule the prepared path does not cover; callers then
// use customDuplicateMatch for each pair.
func newPreparedCustomMatch(rule storage.MatchingRule, keys []fuzzyKeyMode, candidate storage.Record) (preparedCustomMatch, bool) {
	n := len(rule.Items)
	if n == 0 || n > maxPreparedItems {
		return preparedCustomMatch{}, false
	}
	p := preparedCustomMatch{rule: rule, n: n}
	var order [maxPreparedItems]int
	placed := 0
	for i, item := range rule.Items {
		left, lok := candidate.GetField(item.Field)
		prepared := preparedCustomItem{
			field:       item.Field,
			method:      item.Method,
			exact:       strings.EqualFold(item.Method, "Exact"),
			matchBlanks: strings.EqualFold(item.BlankBehavior, "MatchBlanks"),
			initialKey:  i < len(keys) && keys[i] == fuzzyKeyFirstInitial,
			leftBlank:   duplicateBlank(left, lok),
		}
		switch {
		case prepared.leftBlank:
		case prepared.exact:
			prepared.text = strings.TrimSpace(workflowValueString(left))
			if len(prepared.text) > duplicateExactLimit {
				prepared.text = duplicateExactPrefix(prepared.text)
			}
		default:
			prepared.text = workflowValueString(left)
			if prepared.initialKey {
				prepared.first = newFirstNameInitialSide(prepared.text)
			} else {
				prepared.scorer, prepared.scored = fuzzyScorerFor(item.Method, prepared.text)
			}
		}
		p.items[i] = prepared
	}
	// Preferred order: Exact items, then fuzzy items by cost rank, each in
	// item order. It is a property of the rule, so the plan is shared.
	for rank := 0; rank <= 3; rank++ {
		for i, item := range rule.Items {
			r := 0
			if !strings.EqualFold(item.Method, "Exact") {
				r = fuzzyMethodCostRank(item.Method)
			}
			if r == rank {
				order[placed] = i
				placed++
			}
		}
	}
	p.plan = cachedFilterPlan(rule.BooleanFilter, order[:n])
	return p, true
}

// verdict scores one item of the pair: whether it matches and, for a matched
// fuzzy item, the method's score.
func (p *preparedCustomMatch) verdict(i int, existing storage.Record) (matched bool, score float64) {
	item := &p.items[i]
	right, rok := existing.GetField(item.field)
	// duplicateBlank, with the value text read once for the comparison too.
	raw := ""
	if rok && right.Kind != storage.ValueNull {
		raw = workflowValueString(right)
	}
	trimmed := strings.TrimSpace(raw)
	if item.leftBlank || trimmed == "" {
		return item.leftBlank && trimmed == "" && item.matchBlanks, 0
	}
	if !item.exact {
		if item.initialKey {
			return item.first.keyed(raw)
		}
		if item.scorer == nil {
			return FuzzyScore(item.method, item.text, raw, item.matchBlanks)
		}
		// FuzzyScore with the candidate side resolved: both texts are
		// trimmed and not blank here.
		return item.scorer(item.scored, capFuzzyInput(trimmed), item.matchBlanks)
	}
	if len(trimmed) > duplicateExactLimit {
		trimmed = duplicateExactPrefix(trimmed)
	}
	return strings.EqualFold(item.text, trimmed), 0
}

// match is customDuplicateMatch(rule, keys, candidate, existing) for the
// prepared rule, keys and candidate.
func (p *preparedCustomMatch) match(existing storage.Record) (matched, supported bool, confidence float64) {
	var verdicts [maxPreparedItems]bool
	var scored [maxPreparedItems]bool
	var fuzzyScores [maxPreparedItems]float64
	plan, state := p.plan, 0
	for {
		next := plan.step[state]
		if next < 0 {
			if outcome := filterOutcome(-next); outcome != filterTrueSupported {
				matched, supported = outcome.result()
				return matched, supported, 0
			}
			break
		}
		i := int(next)
		verdicts[i], fuzzyScores[i] = p.verdict(i, existing)
		scored[i] = true
		if verdicts[i] {
			state += 2 * plan.pow3[i]
		} else {
			state += plan.pow3[i]
		}
	}
	// The pair matches whatever the items not scored yet say. Score them, then
	// every item's confidence, as customDuplicateMatch does.
	n := p.n
	for i := 0; i < n; i++ {
		if !scored[i] {
			verdicts[i], fuzzyScores[i] = p.verdict(i, existing)
		}
	}
	var scores [maxPreparedItems]float64
	for i := 0; i < n; i++ {
		item := &p.items[i]
		right, rok := existing.GetField(item.field)
		switch {
		case item.leftBlank || duplicateBlank(right, rok):
			scores[i] = 100
			if !verdicts[i] {
				scores[i] = 50
			}
		case !item.exact:
			if verdicts[i] {
				scores[i] = fuzzyItemScore(item.method, fuzzyScores[i])
			} else {
				scores[i] = FuzzyRawConfidence(item.method, item.text, workflowValueString(right), item.matchBlanks)
			}
		default:
			scores[i] = 100
			if !verdicts[i] {
				scores[i] = 50
			}
		}
	}
	if _, confidence, ok := filterRuleConfidence(p.rule.BooleanFilter, verdicts[:n], scores[:n]); ok {
		return true, true, confidence
	}
	return true, true, 100
}

// fuzzyScorerFor resolves FuzzyScore's scorer for a method and the trimmed,
// capped candidate text. scorer is nil when FuzzyScore would not score the
// pair (unknown method or blank text); callers then call FuzzyScore.
func fuzzyScorerFor(method, left string) (scorer fuzzyScorer, text string) {
	key := strings.ToLower(strings.TrimSpace(method))
	scorer, ok := fuzzyScorers[key]
	if !ok {
		return nil, ""
	}
	if custom, ok := customFuzzyScorers[key]; ok {
		scorer = custom
	}
	left = strings.TrimSpace(left)
	if left == "" {
		return nil, ""
	}
	return scorer, capFuzzyInput(left)
}
