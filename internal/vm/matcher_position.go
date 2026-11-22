package vm

import (
	"sort"
	"unicode/utf8"
)

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
