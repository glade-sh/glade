package sema

import "sort"

type sourceToken struct {
	start int
	end   int
}

type sourceBrace struct {
	offset int
	match  int
	open   bool
}

// sourceFacts is an immutable, analysis-local lexical index. It preserves
// source byte offsets and the historical differences between general code
// exclusion and schema-reference masking.
type sourceFacts struct {
	source             string
	length             int
	codeBits           []uint64
	schemaBits         []uint64
	candidateStartBits []uint64
	candidateEndBits   []uint64
	candidateCount     int
	braces             []sourceBrace
}

type sourceFactsMode uint8

const (
	sourceFactsNormal sourceFactsMode = iota
	sourceFactsLineComment
	sourceFactsBlockComment
	sourceFactsSingleQuoted
	sourceFactsDoubleQuoted
)

func newSourceFacts(source string) *sourceFacts {
	words := (len(source) + 63) / 64
	bits := make([]uint64, words*4)
	facts := &sourceFacts{
		source:             source,
		length:             len(source),
		codeBits:           bits[:words],
		schemaBits:         bits[words : words*2],
		candidateStartBits: bits[words*2 : words*3],
		candidateEndBits:   bits[words*3:],
		braces:             make([]sourceBrace, 0, len(source)/32),
	}
	var braceStack [128]int
	var braceOverflow []int
	braceDepth := 0
	codeMode := sourceFactsNormal
	schemaMode := sourceFactsNormal
	codeBlockStart := -1
	schemaBlockStart := -1
	codeEscaped := false
	schemaEscaped := false
	schemaCandidateStart := -1

	for offset := 0; offset < len(source); offset++ {
		value := source[offset]
		code := false
		switch codeMode {
		case sourceFactsNormal:
			switch {
			case value == '/' && offset+1 < len(source) && source[offset+1] == '/':
				codeMode = sourceFactsLineComment
			case value == '/' && offset+1 < len(source) && source[offset+1] == '*':
				codeMode = sourceFactsBlockComment
				codeBlockStart = offset
			case value == '\'':
				codeMode = sourceFactsSingleQuoted
			case value == '"':
				codeMode = sourceFactsDoubleQuoted
			default:
				code = true
			}
		case sourceFactsLineComment:
			if value == '\n' {
				codeMode = sourceFactsNormal
				code = true
			}
		case sourceFactsBlockComment:
			if value == '/' && offset > codeBlockStart+2 && source[offset-1] == '*' {
				codeMode = sourceFactsNormal
				codeBlockStart = -1
			}
		case sourceFactsSingleQuoted, sourceFactsDoubleQuoted:
			if codeEscaped {
				codeEscaped = false
			} else if value == '\\' {
				codeEscaped = true
			} else if codeMode == sourceFactsSingleQuoted && value == '\'' ||
				codeMode == sourceFactsDoubleQuoted && value == '"' {
				codeMode = sourceFactsNormal
			}
		}
		if code {
			facts.codeBits[offset>>6] |= uint64(1) << uint(offset&63)
			switch value {
			case '{':
				facts.braces = append(facts.braces, sourceBrace{offset: offset, match: -1, open: true})
				openIndex := len(facts.braces) - 1
				if braceDepth < len(braceStack) {
					braceStack[braceDepth] = openIndex // #nosec G602 -- guarded by braceDepth < len(braceStack).
				} else {
					braceOverflow = append(braceOverflow, openIndex)
				}
				braceDepth++
			case '}':
				facts.braces = append(facts.braces, sourceBrace{offset: offset, match: -1})
				closeIndex := len(facts.braces) - 1
				if braceDepth > 0 {
					braceDepth--
					openIndex := 0
					if braceDepth < len(braceStack) {
						openIndex = braceStack[braceDepth] // #nosec G602 -- guarded by braceDepth < len(braceStack).
					} else {
						openIndex = braceOverflow[len(braceOverflow)-1]
						braceOverflow = braceOverflow[:len(braceOverflow)-1]
					}
					facts.braces[openIndex].match = offset
					facts.braces[closeIndex].match = facts.braces[openIndex].offset
				}
			}
		}

		schemaExcluded := false
		switch schemaMode {
		case sourceFactsNormal:
			switch {
			case value == '/' && offset+1 < len(source) && source[offset+1] == '/':
				schemaMode = sourceFactsLineComment
				schemaExcluded = true
			case value == '/' && offset+1 < len(source) && source[offset+1] == '*':
				schemaMode = sourceFactsBlockComment
				schemaBlockStart = offset
				schemaExcluded = true
			case value == '\'':
				schemaMode = sourceFactsSingleQuoted
				schemaExcluded = true
			}
		case sourceFactsLineComment:
			if value == '\n' || value == '\r' {
				schemaMode = sourceFactsNormal
			} else {
				schemaExcluded = true
			}
		case sourceFactsBlockComment:
			schemaExcluded = true
			if value == '/' && offset > schemaBlockStart+2 && source[offset-1] == '*' {
				schemaMode = sourceFactsNormal
				schemaBlockStart = -1
			}
		case sourceFactsSingleQuoted:
			schemaExcluded = true
			if schemaEscaped {
				schemaEscaped = false
			} else if value == '\\' {
				schemaEscaped = true
			} else if value == '\'' {
				if offset+1 < len(source) && source[offset+1] == '\'' {
					schemaEscaped = true
				} else {
					schemaMode = sourceFactsNormal
				}
			}
		}
		schemaVisible := !schemaExcluded || value == '\n' || value == '\r'
		if schemaVisible {
			facts.schemaBits[offset>>6] |= uint64(1) << uint(offset&63)
		}
		if schemaVisible && sourceFactsIdentifierByte(value) {
			if schemaCandidateStart < 0 && sourceFactsIdentifierStart(value) {
				schemaCandidateStart = offset
			}
		} else if schemaCandidateStart >= 0 {
			facts.recordSchemaCandidate(schemaCandidateStart, offset-1)
			schemaCandidateStart = -1
		}
	}
	if schemaCandidateStart >= 0 {
		facts.recordSchemaCandidate(schemaCandidateStart, len(source)-1)
	}
	// Preserve the legacy malformed-input behavior: an unterminated block
	// comment leaves the final source byte visible to schema inference.
	if schemaMode == sourceFactsBlockComment && schemaBlockStart >= 0 && len(source) > schemaBlockStart+2 && source[len(source)-1] != '\'' {
		offset := len(source) - 1
		facts.schemaBits[offset>>6] |= uint64(1) << uint(offset&63)
		if sourceFactsIdentifierStart(source[offset]) {
			facts.recordSchemaCandidate(offset, offset)
		}
	}
	return facts
}

func (f *sourceFacts) sourceLen() int {
	if f == nil {
		return 0
	}
	return f.length
}

func (f *sourceFacts) sourceText() string {
	if f == nil {
		return ""
	}
	return f.source
}

func (f *sourceFacts) containsCode(offset int) bool {
	if f == nil || offset < 0 || offset >= f.length {
		return false
	}
	return f.codeBits[offset>>6]&(uint64(1)<<uint(offset&63)) != 0
}

func (f *sourceFacts) codeSpans() semaCodeSpans {
	return semaCodeSpans{facts: f}
}

func (f *sourceFacts) openBraces(start, offset int) []int {
	if f == nil || offset <= start || offset <= 0 || start >= f.length {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if offset > f.length {
		offset = f.length
	}
	first := sort.Search(len(f.braces), func(i int) bool {
		return f.braces[i].offset >= start
	})
	var open []int
	for i := first; i < len(f.braces) && f.braces[i].offset < offset; i++ {
		if f.braces[i].open {
			open = append(open, f.braces[i].offset)
		} else if len(open) > 0 {
			open = open[:len(open)-1]
		}
	}
	return open
}

func (f *sourceFacts) matchingBrace(offset int) int {
	if f == nil {
		return -1
	}
	index := sort.Search(len(f.braces), func(i int) bool {
		return f.braces[i].offset >= offset
	})
	if index == len(f.braces) || f.braces[index].offset != offset || !f.braces[index].open {
		return -1
	}
	return f.braces[index].match
}

func (f *sourceFacts) schemaScanSource() string {
	if f == nil {
		return ""
	}
	out := []byte(f.source)
	for offset := range out {
		if f.schemaBits[offset>>6]&(uint64(1)<<uint(offset&63)) == 0 {
			out[offset] = ' '
		}
	}
	return string(out)
}

func (f *sourceFacts) schemaCandidates() []sourceToken {
	if f == nil {
		return nil
	}
	candidates := make([]sourceToken, 0, f.candidateCount)
	for offset := 0; offset < f.length; offset++ {
		if !sourceFactsBitContains(f.candidateStartBits, offset) {
			continue
		}
		end := offset
		for end < f.length && !sourceFactsBitContains(f.candidateEndBits, end) {
			end++
		}
		if end >= f.length {
			break
		}
		candidates = append(candidates, sourceToken{start: offset, end: end + 1})
		offset = end
	}
	return candidates
}

func (f *sourceFacts) retainedSchemaCandidateCount() int {
	if f == nil {
		return 0
	}
	return f.candidateCount
}

func (f *sourceFacts) recordSchemaCandidate(start, end int) {
	if f == nil || start < 0 || end < start || end >= f.length {
		return
	}
	f.candidateStartBits[start>>6] |= uint64(1) << uint(start&63)
	f.candidateEndBits[end>>6] |= uint64(1) << uint(end&63)
	f.candidateCount++
}

func sourceFactsBitContains(bits []uint64, offset int) bool {
	return offset >= 0 && offset>>6 < len(bits) && bits[offset>>6]&(uint64(1)<<uint(offset&63)) != 0
}

func sourceFactsIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func sourceFactsIdentifierByte(value byte) bool {
	return sourceFactsIdentifierStart(value) || value >= '0' && value <= '9'
}
