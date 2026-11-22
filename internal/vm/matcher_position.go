package vm

import (
	"regexp/syntax"
	"sort"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// Captured inputs have either one supplementary character, or one between
// two single-byte characters. Other subjects retain their prior behavior.
func matcherCapturedPositionSubject(text string) bool {
	if len(text) == 4 {
		r, size := utf8.DecodeRuneInString(text)
		return size == 4 && r > 0xffff
	}
	if len(text) == 6 && text[0] < utf8.RuneSelf && text[5] < utf8.RuneSelf {
		r, size := utf8.DecodeRuneInString(text[1:5])
		return size == 4 && r > 0xffff
	}
	return false
}

// matcherMidPairSubject reuses the view with one supplementary rune split.
// Moving the split forward shifts only the runes crossed by the search.
type matcherMidPairSubject struct {
	runes   []rune
	bytes   []int
	split   []rune
	splitAt int
	valid   bool
	plan    *regexp2Plan
	allowed bool
}

func newMatcherMidPairSubject(text string) *matcherMidPairSubject {
	runes := []rune(text)
	bytes := make([]int, 0, len(runes)+1)
	for at := range text {
		bytes = append(bytes, at)
	}
	bytes = append(bytes, len(text))
	return &matcherMidPairSubject{runes: runes, bytes: bytes, splitAt: -1, valid: utf8.ValidString(text)}
}

func (s *matcherMidPairSubject) runeAtByte(at int) int {
	return sort.SearchInts(s.bytes, at)
}

func (s *matcherMidPairSubject) splitAtRune(at int) []rune {
	if s.split == nil {
		s.split = make([]rune, len(s.runes)+1)
	}
	if s.splitAt < 0 || at < s.splitAt {
		copy(s.split[:at], s.runes[:at])
		copy(s.split[at+2:], s.runes[at+1:])
	} else if at > s.splitAt {
		s.split[s.splitAt] = s.runes[s.splitAt]
		copy(s.split[s.splitAt+1:at], s.split[s.splitAt+2:at+1])
	}
	r := s.runes[at] - 0x10000
	s.split[at], s.split[at+1] = 0xd800+(r>>10), 0xdc00+(r&0x3ff)
	s.splitAt = at
	return s.split
}

// The rows cover an empty expression and a star over one literal. Optional
// expressions and assertions retain the previous search path.
func (s *matcherMidPairSubject) allows(plan *regexp2Plan) (bool, error) {
	if s.plan == plan {
		return s.allowed, nil
	}
	allowed := false
	if plan.grapheme == nil && plan.linear == nil && plan.publicGroupCount() == 0 {
		if tree, err := syntax.Parse(plan.originalSource, syntax.Perl); err == nil {
			allowed = tree.Op == syntax.OpEmptyMatch || (tree.Op == syntax.OpStar && len(tree.Sub) == 1 && tree.Sub[0].Op == syntax.OpLiteral && len(tree.Sub[0].Rune) == 1)
		}
	}
	s.plan, s.allowed = plan, allowed
	return allowed, nil
}

func matcherEmptyFindPattern(matcher Value, input string) bool {
	plan, err := matcherRegexp2PlanForInput(matcher, input)
	if err != nil || plan.grapheme != nil || plan.linear != nil || plan.publicGroupCount() != 0 {
		return false
	}
	tree, err := syntax.Parse(plan.originalSource, syntax.Perl)
	return err == nil && tree.Op == syntax.OpEmptyMatch
}

// matcherEmptyMidPair observes a zero-width match at the UTF-16 boundary
// within a supplementary rune. The surrounding runes are kept for assertions.
func matcherEmptyMidPair(plan *regexp2Plan, text string, subject *matcherMidPairSubject, at int, allowStar bool) (*regexp2.Match, error) {
	if at < 0 || at >= len(subject.runes) || subject.runes[at] <= 0xffff || !subject.valid {
		return nil, nil
	}
	tree, err := syntax.Parse(plan.originalSource, syntax.Perl)
	if err != nil {
		return nil, nil
	}
	if allowStar {
		if (tree.Op == syntax.OpEmptyMatch && len(text) != 6) || (tree.Op != syntax.OpEmptyMatch && len(text) != 4) {
			return nil, nil
		}
	} else if tree.Op != syntax.OpEmptyMatch {
		return nil, nil
	}
	allowed, err := subject.allows(plan)
	if err != nil || !allowed {
		return nil, err
	}
	probe := subject.splitAtRune(at)
	match, err := searchRegexp2(plan.re, text, func() (*regexp2.Match, error) {
		return plan.re.FindRunesMatchStartingAt(probe, at+1)
	})
	if err != nil || match == nil || match.Index != at+1 || match.Length != 0 {
		return nil, err
	}
	if err := plan.checkStackDepth(match); err != nil {
		return nil, err
	}
	return match, nil
}

// matcherPositions maps between UTF-16 boundaries and byte boundaries. A
// boundary within a surrogate pair is represented by the pair's first byte
// and inside=true; it cannot be used directly to slice the Go string.
type matcherPositions struct {
	length int
	bytes  []int
	inside []bool
}

func newMatcherPositions(text string) matcherPositions {
	p := matcherPositions{length: len(text)}
	nonASCII := 0
	for nonASCII < len(text) && text[nonASCII] < utf8.RuneSelf {
		nonASCII++
	}
	if nonASCII == len(text) {
		return p
	}
	p.bytes = make([]int, 0, len(text)+1)
	p.inside = make([]bool, 0, len(text)+1)
	for i := 0; i < len(text); {
		r, size := decodeApexRune(text[i:])
		p.bytes = append(p.bytes, i)
		p.inside = append(p.inside, false)
		if apexRuneWidth(r) == 2 {
			p.bytes = append(p.bytes, i)
			p.inside = append(p.inside, true)
		}
		i += size
	}
	p.bytes = append(p.bytes, len(text))
	p.inside = append(p.inside, false)
	p.length = len(p.bytes) - 1
	return p
}

func (p matcherPositions) byteAtUnit(unit int) (int, bool, bool) {
	if unit < 0 || unit > p.length {
		return 0, false, false
	}
	if p.bytes == nil {
		return unit, false, true
	}
	return p.bytes[unit], p.inside[unit], true
}

func (p matcherPositions) unitAtByte(at int) (int, bool) {
	if p.bytes == nil {
		return at, at >= 0 && at <= p.length
	}
	i := sort.SearchInts(p.bytes, at)
	return i, i < len(p.bytes) && p.bytes[i] == at
}

// matcherSingleLowSurrogateRegion identifies the captured one-unit region
// containing the low half of a supplementary character.
func matcherSingleLowSurrogateRegion(input string, start, end int) bool {
	if start != 1 || end != 2 || len(input) != 4 || apexStringLength(input) != 2 {
		return false
	}
	_, err := byteIndexForApexStringIndex(input, start)
	return err != nil
}

// The captured region search uses a single, non-capturing any-character
// expression. Other patterns retain their prior region validation behavior.
func matcherCapturedLowRegion(matcher Value, input string, start, end int) bool {
	if !matcherSingleLowSurrogateRegion(input, start, end) || matcherUsesFullInputBounds(matcher) {
		return false
	}
	plan, err := matcherRegexp2PlanForInput(matcher, input)
	if err != nil || plan.publicGroupCount() != 0 {
		return false
	}
	tree, err := syntax.Parse(plan.originalSource, syntax.Perl)
	return err == nil && tree.Op == syntax.OpAnyCharNotNL
}

func matcherFindSingleLowRegion(matcher Value, input string, cache *preparedMatcherCache) (bool, bool, error) {
	start, end := matcher.Fields["regionStart"], matcher.Fields["regionEnd"]
	if start.Kind != ValueInt || end.Kind != ValueInt || !matcherCapturedLowRegion(matcher, input, int(start.Int), int(end.Int)) {
		return false, false, nil
	}
	positions := matcherPositionsFor(cache, matcher.Ref, input)
	startByte, _, _ := positions.byteAtUnit(int(start.Int))
	endByte, _, _ := positions.byteAtUnit(int(end.Int))
	cursor := matcher.Fields["index"]
	at := 0
	if cursor.Kind == ValueInt {
		switch int(cursor.Int) {
		case -startByte - 2:
			at = 0
		case endByte:
			at = 1
		default:
			matcherClearMatch(matcher)
			return false, true, nil
		}
	}
	text := apexSubstringUnits(input, int(start.Int), int(end.Int))
	r, _ := decodeApexRune(text)
	runes := []rune{r}
	plan, err := matcherRegexp2PlanForInput(matcher, text)
	if err != nil {
		return false, true, err
	}
	if plan.grapheme != nil || plan.linear != nil {
		return false, false, nil
	}
	match, err := searchRegexp2(plan.re, text, func() (*regexp2.Match, error) {
		return plan.re.FindRunesMatchStartingAt(runes, at)
	})
	if err != nil {
		return false, true, err
	}
	observer, slot, err := plan.preparedFindObserver()
	if err != nil {
		return false, true, err
	}
	observation, err := searchRegexp2(observer, text, func() (*regexp2.Match, error) {
		return observer.FindRunesMatchStartingAt(runes, at)
	})
	if err != nil {
		return false, true, err
	}
	hitEnd := false
	if observation != nil {
		group := observation.GroupByNumber(slot)
		hitEnd = group != nil && len(group.Captures) != 0
	}
	if match == nil {
		matcherClearMatch(matcher)
		matcher.Fields["hitEnd"] = Bool(plan.failedFindHitEnd(hitEnd))
		matcherSetRequireEnd(matcher, false)
		return false, true, nil
	}
	if err := plan.checkStackDepth(match); err != nil {
		return false, true, err
	}
	encode := func(unit int) int {
		b, inside, _ := positions.byteAtUnit(int(start.Int) + unit)
		if inside {
			return -b - 2
		}
		return b
	}
	indices := make([]int, 0, 2*len(plan.publicGroupNumbers))
	for _, number := range plan.publicGroupNumbers {
		group := match.GroupByNumber(number)
		if group == nil || len(group.Captures) == 0 {
			indices = append(indices, -1, -1)
		} else {
			indices = append(indices, encode(group.Index), encode(group.Index+group.Length))
		}
	}
	matcherSaveMatch(matcher, indices)
	requiresEnd := plan.matchRequiresEndRunes(runes, match)
	matcher.Fields["hitEnd"] = Bool(hitEnd || requiresEnd)
	matcherSetRequireEnd(matcher, requiresEnd)
	next := match.Index + match.Length
	if match.Length == 0 {
		next++
	}
	if next > 1 {
		matcher.Fields["index"] = Int(int64(endByte + 1))
	} else {
		matcher.Fields["index"] = Int(int64(encode(next)))
	}
	return true, true, nil
}
