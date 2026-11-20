package vm

import (
	"sort"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// Two recent Matcher identities per VM, cleared at transaction exit. The entry
// contains immutable subject data. The visible cursor, groups and end flags
// remain exclusively in the Apex Matcher object.
type preparedMatcherCache struct {
	entries [2]preparedMatcherEntry
	next    int
}

type preparedMatcherEntry struct {
	ref       uint64
	input     string
	checked   bool
	prepared  *preparedMatcherInput
	hasRegion bool
	region    matcherRegionBounds
}

type preparedMatcherInput struct {
	runes []rune
	bytes []uint32
	units []uint32
}

func (c *preparedMatcherCache) reset() {
	*c = preparedMatcherCache{}
}

func (c *preparedMatcherCache) release() {
	c.entries = [2]preparedMatcherEntry{}
}

func (c *preparedMatcherCache) forget(ref uint64) {
	if c == nil {
		return
	}
	for i := range c.entries {
		if c.entries[i].ref == ref {
			c.entries[i] = preparedMatcherEntry{}
		}
	}
}

func (c *preparedMatcherCache) lookup(ref uint64, input string) *preparedMatcherEntry {
	if c == nil || ref == 0 {
		return nil
	}
	for i := range c.entries {
		e := &c.entries[i]
		if e.ref == ref && e.input == input {
			return e
		}
	}
	return nil
}

func (c *preparedMatcherCache) use(ref uint64, input string) *preparedMatcherEntry {
	if c == nil || ref == 0 || len(input) < 8 || len(input) > 64<<10 {
		return nil
	}
	if entry := c.lookup(ref, input); entry != nil {
		if !entry.checked {
			entry.checked = true
			// Glade can hold encoded lone UTF-16 surrogates. The stock path
			// handles them and every other unusual UTF-8 input.
			if utf8.ValidString(input) {
				entry.prepared = prepareMatcherInput(input)
			}
		}
		return entry
	}
	e := &c.entries[c.next]
	c.next = (c.next + 1) % len(c.entries)
	*e = preparedMatcherEntry{ref: ref, input: input}
	return e
}

func prepareMatcherInput(text string) *preparedMatcherInput {
	s := &preparedMatcherInput{runes: []rune(text)}
	if len(s.runes) == len(text) {
		return s // ASCII has identical rune, byte and UTF-16 positions.
	}
	s.bytes = make([]uint32, len(s.runes)+1)
	s.units = make([]uint32, len(s.runes)+1)
	i, count := 0, uint32(0)
	for offset, r := range text {
		s.bytes[i], s.units[i] = uint32(offset), count
		i++
		count += uint32(apexRuneWidth(r))
	}
	s.bytes[i], s.units[i] = uint32(len(text)), count
	return s
}

func (s *preparedMatcherInput) runeAtByte(offset int) (int, bool) {
	if offset < 0 {
		return 0, false
	}
	if s.bytes == nil {
		return offset, offset <= len(s.runes)
	}
	at := sort.Search(len(s.bytes), func(i int) bool { return int(s.bytes[i]) >= offset })
	return at, at < len(s.bytes) && int(s.bytes[at]) == offset
}

func (s *preparedMatcherInput) byteAtRune(index int) int {
	if s.bytes == nil {
		return index
	}
	return int(s.bytes[index])
}

func (s *preparedMatcherInput) unitAtRune(index int) int {
	if s.units == nil {
		return index
	}
	return int(s.units[index])
}

func preparedMatcherRegion(e *preparedMatcherEntry, m Value, input string) (matcherRegionBounds, error) {
	if e != nil && e.prepared != nil && e.hasRegion {
		start, end := m.Fields["regionStart"], m.Fields["regionEnd"]
		if start.Kind == ValueInt && end.Kind == ValueInt && start.Int == int64(e.region.startIndex) && end.Int == int64(e.region.endIndex) {
			return e.region, nil
		}
	}
	r, err := matcherRegion(m, input)
	if e != nil && err == nil {
		e.hasRegion, e.region = true, r
	}
	return r, err
}

func preparedMatcherGroupBounds(c *preparedMatcherCache, m Value, input string, group int) (int, int, error) {
	e := c.lookup(m.Ref, input)
	if e == nil || e.prepared == nil {
		return matcherGroupBounds(m, input, group)
	}
	start, end, err := matcherGroupByteBounds(m, group)
	if err != nil {
		return 0, 0, err
	}
	if start < 0 || end < 0 {
		return -1, -1, nil
	}
	i, okI := e.prepared.runeAtByte(start)
	j, okJ := e.prepared.runeAtByte(end)
	if !okI || !okJ {
		return matcherGroupBounds(m, input, group)
	}
	return e.prepared.unitAtRune(i), e.prepared.unitAtRune(j), nil
}

func (p *regexp2Plan) preparedFindObserver() (*regexp2.Regexp, int, error) {
	p.preparedObserverOnce.Do(func() {
		source, slot := regexp2EndObserverSource(p.matchSource, p.re)
		re, err := regexp2.Compile(source, regexp2.None)
		if err == nil {
			re.MatchTimeout = regexp2MatchTimeout
		}
		p.preparedObserverRE, p.preparedObserverSlot, p.preparedObserverErr = re, slot, err
	})
	return p.preparedObserverRE, p.preparedObserverSlot, p.preparedObserverErr
}

func preparedMatcherFind(c *preparedMatcherCache, e *preparedMatcherEntry, m Value, input string, region matcherRegionBounds, startByte int) ([]int, bool, bool, *requireEndProbe, error) {
	if c == nil || region.startByte != 0 || region.endByte != len(input) || matcherUsesFullInputBounds(m) {
		return matcherRegexp2FindIndices(m, input, region, startByte)
	}
	p, err := matcherRegexp2PlanForInput(m, input)
	if err != nil {
		return nil, false, false, nil, err
	}
	// Keep the grapheme filter, linear observer and every regional variant on
	// the existing path. The checked subset below follows the same search order.
	if p.grapheme != nil || p.linear != nil {
		return matcherRegexp2FindIndices(m, input, region, startByte)
	}
	var subject *preparedMatcherInput
	if e != nil {
		subject = e.prepared
	}
	startRune := 0
	if subject != nil {
		var ok bool
		startRune, ok = subject.runeAtByte(startByte)
		if !ok {
			return matcherRegexp2FindIndices(m, input, region, startByte)
		}
	}
	run := func(re *regexp2.Regexp) (*regexp2.Match, error) {
		return searchRegexp2(re, input, func() (*regexp2.Match, error) {
			if subject == nil {
				return re.FindStringMatchStartingAt(input, startByte)
			}
			return re.FindRunesMatchStartingAt(subject.runes, startRune)
		})
	}
	match, err := run(p.re)
	if err != nil {
		return nil, false, false, nil, err
	}
	if match != nil {
		if err := p.checkStackDepth(match); err != nil {
			return nil, false, false, nil, err
		}
	}
	observer, slot, err := p.preparedFindObserver()
	if err != nil {
		return nil, false, false, nil, err
	}
	observation, err := run(observer)
	if err != nil {
		return nil, false, false, nil, err
	}
	hit := false
	if observation != nil {
		g := observation.GroupByNumber(slot)
		hit = g != nil && len(g.Captures) > 0
	}
	if match == nil {
		return nil, p.failedFindHitEnd(hit), false, p.requireEndProbe(input, startByte, region.endRune), nil
	}
	if subject == nil {
		idx, err := p.matchByteIndices(input, match, 0)
		req := p.matchRequiresEnd(input, match)
		return idx, hit || req, req, nil, err
	}
	indices := make([]int, 0, 2*len(p.publicGroupNumbers))
	for _, num := range p.publicGroupNumbers {
		group := match.GroupByNumber(num)
		if group == nil || len(group.Captures) == 0 {
			indices = append(indices, -1, -1)
			continue
		}
		indices = append(indices, subject.byteAtRune(group.Index), subject.byteAtRune(group.Index+group.Length))
	}
	req := p.matchRequiresEndRunes(subject.runes, match)
	return indices, hit || req, req, nil, nil
}
