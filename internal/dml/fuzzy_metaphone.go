package dml

import "strings"

// doubleMetaphone returns the primary Double Metaphone code of a value,
// limited to maxLen characters. Double Metaphone is Lawrence Philips' public
// phonetic algorithm; the Salesforce help article on match keys names it for
// the name fields, and the native rows fit the widely used Java rendering of
// it (upper-cased input, a code of at most four characters, "Ç" as S, "Ñ" as
// N, every other non-ASCII letter skipped). The alternate code plays no part
// in the rows, so only the primary code is built.
func doubleMetaphone(value string, maxLen int) string {
	m := metaphoneInput{v: []rune(strings.ToUpper(strings.TrimSpace(value)))}
	m.slavoGermanic = strings.ContainsAny(string(m.v), "WK") || strings.Contains(string(m.v), "CZ") || strings.Contains(string(m.v), "WITZ")
	var out []rune
	add := func(s string) {
		for _, r := range s {
			if len(out) < maxLen {
				out = append(out, r)
			}
		}
	}
	i := 0
	if m.has(0, "GN", "KN", "PN", "WR", "PS") {
		i = 1
	}
	for len(out) < maxLen && i < len(m.v) {
		switch m.at(i) {
		case 'A', 'E', 'I', 'O', 'U', 'Y':
			if i == 0 {
				add("A")
			}
			i++
		case 'B':
			add("P")
			i += m.step(i, 'B')
		case 'Ç':
			add("S")
			i++
		case 'C':
			i = m.letterC(i, add)
		case 'D':
			switch {
			case m.has(i, "DG"):
				if m.has(i+2, "I", "E", "Y") {
					add("J")
					i += 3
				} else {
					add("TK")
					i += 2
				}
			case m.has(i, "DT", "DD"):
				add("T")
				i += 2
			default:
				add("T")
				i++
			}
		case 'F':
			add("F")
			i += m.step(i, 'F')
		case 'G':
			i = m.letterG(i, add)
		case 'H':
			if (i == 0 || m.vowel(i-1)) && m.vowel(i+1) {
				add("H")
				i += 2
			} else {
				i++
			}
		case 'J':
			i = m.letterJ(i, add)
		case 'K':
			add("K")
			i += m.step(i, 'K')
		case 'L':
			add("L")
			i += m.step(i, 'L')
		case 'M':
			add("M")
			if m.at(i+1) == 'M' || (m.has(i-1, "UMB") && (i+1 == len(m.v)-1 || m.has(i+2, "ER"))) {
				i += 2
			} else {
				i++
			}
		case 'N':
			add("N")
			i += m.step(i, 'N')
		case 'Ñ':
			add("N")
			i++
		case 'P':
			if m.at(i+1) == 'H' {
				add("F")
				i += 2
			} else {
				add("P")
				if m.has(i+1, "P", "B") {
					i += 2
				} else {
					i++
				}
			}
		case 'Q':
			add("K")
			i += m.step(i, 'Q')
		case 'R':
			if !(i == len(m.v)-1 && !m.slavoGermanic && m.has(i-2, "IE") && !m.has(i-4, "ME", "MA")) {
				add("R")
			}
			i += m.step(i, 'R')
		case 'S':
			i = m.letterS(i, add)
		case 'T':
			i = m.letterT(i, add)
		case 'V':
			add("F")
			i += m.step(i, 'V')
		case 'W':
			i = m.letterW(i, add)
		case 'X':
			if i == 0 {
				add("S")
				i++
				break
			}
			if !(i == len(m.v)-1 && (m.has(i-3, "IAU", "EAU") || m.has(i-2, "AU", "OU"))) {
				add("KS")
			}
			if m.has(i+1, "C", "X") {
				i += 2
			} else {
				i++
			}
		case 'Z':
			if m.at(i+1) == 'H' {
				add("J")
				i += 2
			} else {
				add("S")
				i += m.step(i, 'Z')
			}
		default:
			i++
		}
	}
	return string(out)
}

type metaphoneInput struct {
	v             []rune
	slavoGermanic bool
}

func (m metaphoneInput) at(i int) rune {
	if i < 0 || i >= len(m.v) {
		return 0
	}
	return m.v[i]
}

func (m metaphoneInput) vowel(i int) bool {
	return strings.ContainsRune("AEIOUY", m.at(i)) && m.at(i) != 0
}

// has reports whether one of the options starts at position i.
func (m metaphoneInput) has(i int, options ...string) bool {
	if i < 0 {
		return false
	}
	for _, option := range options {
		o := []rune(option)
		if i+len(o) > len(m.v) {
			continue
		}
		if string(m.v[i:i+len(o)]) == option {
			return true
		}
	}
	return false
}

// step is the advance past a letter and its doubled twin.
func (m metaphoneInput) step(i int, twin rune) int {
	if m.at(i+1) == twin {
		return 2
	}
	return 1
}

func (m metaphoneInput) germanic() bool {
	return m.has(0, "VAN ", "VON ", "SCH")
}

func (m metaphoneInput) letterC(i int, add func(string)) int {
	switch {
	case m.has(i, "CHIA"), i > 1 && !m.vowel(i-2) && m.has(i-1, "ACH") && ((m.at(i+2) != 'I' && m.at(i+2) != 'E') || m.has(i-2, "BACHER", "MACHER")):
		add("K")
		return i + 2
	case i == 0 && m.has(i, "CAESAR"):
		add("S")
		return i + 2
	case m.has(i, "CH"):
		switch {
		case i > 0 && m.has(i, "CHAE"):
			add("K")
		case i == 0 && (m.has(i+1, "HARAC", "HARIS") || m.has(i+1, "HOR", "HYM", "HIA", "HEM")) && !m.has(0, "CHORE"):
			add("K")
		case m.germanic(), m.has(i-2, "ORCHES", "ARCHIT", "ORCHID"), m.has(i+2, "T", "S"),
			(m.has(i-1, "A", "O", "U", "E") || i == 0) && (m.has(i+2, "L", "R", "N", "M", "B", "H", "F", "V", "W", " ") || i+1 == len(m.v)-1):
			add("K")
		case i > 0 && m.has(0, "MC"):
			add("K")
		default:
			add("X")
		}
		return i + 2
	case m.has(i, "CZ") && !m.has(i-2, "WICZ"):
		add("S")
		return i + 2
	case m.has(i+1, "CIA"):
		add("X")
		return i + 3
	case m.has(i, "CC") && !(i == 1 && m.at(0) == 'M'):
		if m.has(i+2, "I", "E", "H") && !m.has(i+2, "HU") {
			if (i == 1 && m.at(i-1) == 'A') || m.has(i-1, "UCCEE", "UCCES") {
				add("KS")
			} else {
				add("X")
			}
			return i + 3
		}
		add("K")
		return i + 2
	case m.has(i, "CK", "CG", "CQ"):
		add("K")
		return i + 2
	case m.has(i, "CI", "CE", "CY"):
		add("S")
		return i + 2
	}
	add("K")
	switch {
	case m.has(i+1, " C", " Q", " G"):
		return i + 3
	case m.has(i+1, "C", "K", "Q") && !m.has(i+1, "CE", "CI"):
		return i + 2
	}
	return i + 1
}

func (m metaphoneInput) letterG(i int, add func(string)) int {
	next := m.at(i + 1)
	switch {
	case next == 'H':
		switch {
		case i > 0 && !m.vowel(i-1):
			add("K")
		case i == 0:
			if m.at(i+2) == 'I' {
				add("J")
			} else {
				add("K")
			}
		case (i > 1 && m.has(i-2, "B", "H", "D")) || (i > 2 && m.has(i-3, "B", "H", "D")) || (i > 3 && m.has(i-4, "B", "H")):
		default:
			if i > 2 && m.at(i-1) == 'U' && m.has(i-3, "C", "G", "L", "R", "T") {
				add("F")
			} else if i > 0 && m.at(i-1) != 'I' {
				add("K")
			}
		}
		return i + 2
	case next == 'N':
		switch {
		case i == 1 && m.vowel(0) && !m.slavoGermanic:
			add("KN")
		case !m.has(i+2, "EY") && next != 'Y' && !m.slavoGermanic:
			add("N")
		default:
			add("KN")
		}
		return i + 2
	case m.has(i+1, "LI") && !m.slavoGermanic:
		add("KL")
		return i + 2
	case i == 0 && (next == 'Y' || m.has(i+1, "ES", "EP", "EB", "EL", "EY", "IB", "IL", "IN", "IE", "EI", "ER")):
		add("K")
		return i + 2
	case (m.has(i+1, "ER") || next == 'Y') && !m.has(0, "DANGER", "RANGER", "MANGER") && !m.has(i-1, "E", "I") && !m.has(i-1, "RGY", "OGY"):
		add("K")
		return i + 2
	case m.has(i+1, "E", "I", "Y") || m.has(i-1, "AGGI", "OGGI"):
		if m.germanic() || m.has(i+1, "ET") {
			add("K")
		} else {
			add("J")
		}
		return i + 2
	case next == 'G':
		add("K")
		return i + 2
	}
	add("K")
	return i + 1
}

func (m metaphoneInput) letterJ(i int, add func(string)) int {
	if m.has(i, "JOSE") || m.has(0, "SAN ") {
		if (i == 0 && m.at(i+4) == ' ') || len(m.v) == 4 || m.has(0, "SAN ") {
			add("H")
		} else {
			add("J")
		}
		return i + 1
	}
	switch {
	case i == 0:
		add("J")
	case m.vowel(i-1) && !m.slavoGermanic && (m.at(i+1) == 'A' || m.at(i+1) == 'O'):
		add("J")
	case i == len(m.v)-1:
		add("J")
	case !m.has(i+1, "L", "T", "K", "S", "N", "M", "B", "Z") && !m.has(i-1, "S", "K", "L"):
		add("J")
	}
	return i + m.step(i, 'J')
}

func (m metaphoneInput) letterS(i int, add func(string)) int {
	switch {
	case m.has(i-1, "ISL", "YSL"):
		return i + 1
	case i == 0 && m.has(i, "SUGAR"):
		add("X")
		return i + 1
	case m.has(i, "SH"):
		if m.has(i+1, "HEIM", "HOEK", "HOLM", "HOLZ") {
			add("S")
		} else {
			add("X")
		}
		return i + 2
	case m.has(i, "SIO", "SIA"):
		add("S")
		return i + 3
	case (i == 0 && m.has(i+1, "M", "N", "L", "W")) || m.has(i+1, "Z"):
		add("S")
		if m.has(i+1, "Z") {
			return i + 2
		}
		return i + 1
	case m.has(i, "SC"):
		switch {
		case m.at(i+2) == 'H':
			switch {
			case m.has(i+3, "ER", "EN"):
				add("X")
			case m.has(i+3, "OO", "UY", "ED", "EM"):
				add("SK")
			default:
				add("X")
			}
		case m.has(i+2, "I", "E", "Y"):
			add("S")
		default:
			add("SK")
		}
		return i + 3
	}
	if !(i == len(m.v)-1 && m.has(i-2, "AI", "OI")) {
		add("S")
	}
	if m.has(i+1, "S", "Z") {
		return i + 2
	}
	return i + 1
}

func (m metaphoneInput) letterT(i int, add func(string)) int {
	switch {
	case m.has(i, "TION"), m.has(i, "TIA", "TCH"):
		add("X")
		return i + 3
	case m.has(i, "TH"), m.has(i, "TTH"):
		if m.has(i+2, "OM", "AM") || m.germanic() {
			add("T")
		} else {
			add("0")
		}
		return i + 2
	}
	add("T")
	if m.has(i+1, "T", "D") {
		return i + 2
	}
	return i + 1
}

func (m metaphoneInput) letterW(i int, add func(string)) int {
	switch {
	case m.has(i, "WR"):
		add("R")
		return i + 2
	case i == 0 && (m.vowel(i+1) || m.has(i, "WH")):
		add("A")
		return i + 1
	case (i == len(m.v)-1 && m.vowel(i-1)) || m.has(i-1, "EWSKI", "EWSKY", "OWSKI", "OWSKY") || m.has(0, "SCH"):
		return i + 1
	case m.has(i, "WICZ", "WITZ"):
		add("TS")
		return i + 4
	}
	return i + 1
}

// jaroWinkler is the Jaro-Winkler similarity as the native first-name
// confidences show it: the Jaro part halves the transposition count with
// integer division ("Daniel" against "Diane" scores 0.89, not 0.87), the
// prefix bonus counts the whole common prefix, its scale is the smaller of
// 0.1 and one over the longer length (a 16-letter pair that differs at the
// ninth letter scores 0.946), and it applies from a Jaro value of 0.7.
func jaroWinkler(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if string(ra) == string(rb) {
		return 1
	}
	long, short := ra, rb
	if len(rb) > len(ra) {
		long, short = rb, ra
	}
	if len(short) == 0 {
		return 0
	}
	window := max(len(long)/2-1, 0)
	matchedAt := make([]int, len(short))
	used := make([]bool, len(long))
	matches := 0
	for i, r := range short {
		matchedAt[i] = -1
		for j := max(i-window, 0); j < min(i+window+1, len(long)); j++ {
			if !used[j] && r == long[j] {
				matchedAt[i], used[j] = j, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	var fromShort, fromLong []rune
	for i, r := range short {
		if matchedAt[i] >= 0 {
			fromShort = append(fromShort, r)
		}
	}
	for j, r := range long {
		if used[j] {
			fromLong = append(fromLong, r)
		}
	}
	transposed := 0
	for i := range fromShort {
		if fromShort[i] != fromLong[i] {
			transposed++
		}
	}
	transposed /= 2
	prefix := 0
	for prefix < len(short) && ra[prefix] == rb[prefix] {
		prefix++
	}
	m := float64(matches)
	jaro := (m/float64(len(ra)) + m/float64(len(rb)) + (m-float64(transposed))/m) / 3
	if jaro < 0.7 {
		return jaro
	}
	return jaro + min(0.1, 1/float64(len(long)))*float64(prefix)*(1-jaro)
}
