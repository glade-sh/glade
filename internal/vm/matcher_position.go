package vm

import (
	"sort"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// matcherEmptyMidPair observes a zero-width match at the UTF-16 boundary
// within a supplementary rune. The surrounding runes are kept for assertions.
func matcherEmptyMidPair(plan *regexp2Plan, text string, runes []rune, at int) (*regexp2.Match, error) {
	if at < 0 || at >= len(runes) || runes[at] <= 0xffff || plan.grapheme != nil || plan.linear != nil || plan.publicGroupCount() != 0 {
		return nil, nil
	}
	// Preserve assertion-dependent empty matches, whose mid-pair behavior
	// is not established by the captured empty-subject-capable patterns.
	empty, err := searchRegexp2(plan.re, "", func() (*regexp2.Match, error) {
		return plan.re.FindStringMatch("")
	})
	if err != nil || empty == nil || empty.Length != 0 {
		return nil, err
	}
	r := runes[at] - 0x10000
	probe := make([]rune, 0, len(runes)+1)
	probe = append(probe, runes[:at]...)
	probe = append(probe, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
	probe = append(probe, runes[at+1:]...)
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
