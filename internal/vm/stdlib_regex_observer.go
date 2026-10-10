package vm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// The observation expression follows the engine's ordered alternatives and
// quantifiers. A read attempted at EOF records a private capture and finishes
// that path. It never supplies the result or public captures of the real match.
// This includes reads on abandoned alternatives (R049-R051), greedy repetition
// (R019-R048), and failed possessive matching (R031/R033).
func regexp2EndObserverSource(source string, re *regexp2.Regexp) (string, int) {
	slot := 1
	for _, number := range re.GetGroupNumbers() {
		if number >= slot {
			slot = number + 1
		}
	}
	// Keep numeric escapes from acquiring a new backreference interpretation.
	for strings.Contains(source, `\`+strconv.Itoa(slot)) {
		slot++
	}
	name := strconv.Itoa(slot)
	mark := "(?<" + name + `>\z)`
	wrap := func(atom string, readsEnd bool) string {
		if readsEnd {
			return "(?(" + name + ")|(?:" + atom + "|" + mark + "))"
		}
		return "(?(" + name + ")|" + atom + ")"
	}
	var out strings.Builder
	// Anchor-only expressions still need a declared slot for the conditions.
	// This definition cannot capture or change the observation result.
	out.WriteString("(?:(?<" + name + ">(?!))|)")
	extended := false
	var stack []bool
	for i := 0; i < len(source); {
		start, ch := i, source[i]
		if extended && ch == '#' {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				out.WriteString(source[i:])
				break
			}
			out.WriteString(source[i : i+end+1])
			i += end + 1
			continue
		}
		if extended && strings.ContainsRune(" \t\r\n\f", rune(ch)) {
			out.WriteByte(ch)
			i++
			continue
		}
		switch ch {
		case '(':
			if strings.HasPrefix(source[i:], "(?#") {
				if end := strings.IndexByte(source[i+3:], ')'); end >= 0 {
					i += end + 4
					out.WriteString(source[start:i])
					continue
				}
			}
			if strings.HasPrefix(source[i:], "(?") {
				j, scoped, enabled, sawFlag := i+2, extended, true, false
				for j < len(source) && strings.ContainsRune("imnsxIMNSX-+", rune(source[j])) {
					if source[j] == '-' {
						enabled = false
					} else if source[j] == '+' {
						enabled = true
					} else {
						sawFlag = true
						if source[j] == 'x' || source[j] == 'X' {
							scoped = enabled
						}
					}
					j++
				}
				if sawFlag && j < len(source) && (source[j] == ':' || source[j] == ')') {
					if source[j] == ':' {
						stack = append(stack, extended)
					}
					extended = scoped
					i = j + 1
					out.WriteString(source[start:i])
					continue
				}
				if i+3 < len(source) && source[i+2] == '<' && source[i+3] != '=' && source[i+3] != '!' {
					if end := strings.IndexByte(source[i+3:], '>'); end >= 0 {
						i += end + 4
					} else {
						i += 2
					}
				} else if i+3 < len(source) && source[i+2] == '<' {
					i += 4
				} else {
					i += 3
				}
			} else {
				i++
			}
			stack = append(stack, extended)
			out.WriteString(source[start:i])
		case ')':
			if len(stack) > 0 {
				extended = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			out.WriteByte(ch)
			i++
		case '|', '*', '+', '?':
			out.WriteByte(ch)
			i++
		case '{':
			if end := strings.IndexByte(source[i:], '}'); end >= 0 && isRegexCountQuantifier(source[i+1:i+end]) {
				i += end + 1
				out.WriteString(source[start:i])
			} else {
				out.WriteString(wrap(`\{`, true))
				i++
			}
		case '[':
			i++
			if i < len(source) && source[i] == '^' {
				i++
			}
			if i < len(source) && source[i] == ']' {
				i++
			}
			for i < len(source) {
				if source[i] == '\\' && i+1 < len(source) {
					control := source[i+1] == 'c'
					i += 2
					if control && i < len(source) {
						i++
					}
					continue
				}
				i++
				if source[i-1] == ']' {
					break
				}
			}
			out.WriteString(wrap(source[start:i], true))
		case '\\':
			i += 2
			if i > len(source) {
				i = len(source)
			}
			if i < len(source) {
				switch source[start+1] {
				case 'p', 'P':
					if source[i] == '{' {
						if end := strings.IndexByte(source[i:], '}'); end >= 0 {
							i += end + 1
						}
					}
				case 'k':
					if source[i] == '<' {
						if end := strings.IndexByte(source[i:], '>'); end >= 0 {
							i += end + 1
						}
					}
				case 'u', 'x':
					count := 2
					if source[start+1] == 'u' {
						count = 4
					}
					for count > 0 && i < len(source) {
						i++
						count--
					}
				case 'c':
					i++
				default:
					if source[start+1] >= '0' && source[start+1] <= '9' {
						for i < len(source) && source[i] >= '0' && source[i] <= '9' {
							i++
						}
					}
				}
			}
			atom := source[start:i]
			switch atom {
			case `\A`, `\G`:
				out.WriteString(wrap(atom, false))
			case `\z`:
				out.WriteString(wrap(mark, false))
			case `\b`, `\B`:
				out.WriteString(wrap(atom+"(?:"+mark+")?", true))
			default:
				out.WriteString(wrap(atom, true))
			}
		case '^':
			out.WriteString(wrap("^", false))
			i++
		default:
			_, size := utf8.DecodeRuneInString(source[i:])
			i += size
			out.WriteString(wrap(source[start:i], true))
		}
	}
	return out.String(), slot
}

func (p *regexp2Plan) matchStartingAt(input string, startByte, endRune int, full bool) (*regexp2.Match, error) {
	// Put the end condition inside the expression so reluctant repetitions and
	// alternatives can backtrack to a full-region result (R028).
	source := `\G(?:` + p.matchSource + ")"
	if full {
		source += fmt.Sprintf(`(?=(?s:.{%d})\z)`, utf8.RuneCountInString(input)-endRune)
	}
	re, err := regexp2.Compile(source, regexp2.None)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = regexp2MatchTimeout
	match, err := re.FindStringMatchStartingAt(input, startByte)
	for match != nil && err == nil && !p.validGraphemeMatch(input, match) {
		match, err = re.FindNextMatch(match)
	}
	return match, err
}

func (p *regexp2Plan) operationHitEnd(input string, startByte, endRune int, anchored, full bool) (bool, error) {
	source, slot := regexp2EndObserverSource(p.matchSource, p.re)
	if anchored {
		source = `\G(?:` + source + ")"
	}
	if full {
		source += fmt.Sprintf(`(?=(?s:.{%d})\z)`, utf8.RuneCountInString(input)-endRune)
	}
	re, err := regexp2.Compile(source, regexp2.None)
	if err != nil {
		return false, err
	}
	re.MatchTimeout = regexp2MatchTimeout
	match, err := re.FindStringMatchStartingAt(input, startByte)
	if err != nil || match == nil {
		return false, err
	}
	group := match.GroupByNumber(slot)
	return group != nil && len(group.Captures) > 0, nil
}
