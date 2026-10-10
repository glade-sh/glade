package vm

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

type regexp2EndOptions struct {
	extended  bool
	multiline bool
}

type regexp2EndAssertion struct {
	group     int
	multiline bool
}

// End assertions are recorded on the successful engine path, rather than
// inferred from the match's end offset or the original pattern's spelling.
// Explicit numeric slots retain public named and numbered capture identities.
func instrumentRegexp2EndAssertions(source string, original *regexp2.Regexp) (string, []regexp2EndAssertion) {
	const maxSlot = 2147483647
	next := 1
	for _, number := range original.GetGroupNumbers() {
		if number >= next {
			next = number + 1
		}
	}
	// Adding a capture must not reinterpret an octal escape as a backreference.
	reserved := make(map[int]bool)
	for i := 0; i+1 < len(source); i++ {
		if source[i] != '\\' {
			continue
		}
		number := 0
		for j := i + 1; j < len(source) && source[j] >= '0' && source[j] <= '9'; j++ {
			digit := int(source[j] - '0')
			if number > (maxSlot-digit)/10 {
				break
			}
			number = number*10 + digit
			reserved[number] = true
		}
		i++
	}

	var out strings.Builder
	var groups []regexp2EndAssertion
	var optionStack []regexp2EndOptions
	options := regexp2EndOptions{}
	unixDollar := false
	inClass, firstInClass := false, false
	classStart := -1
	for i := 0; i < len(source); i++ {
		ch := source[i]
		defaultEnd := false
		if ch == '\\' && !inClass && i+1 < len(source) && source[i+1] == 'Z' {
			// Uppercase Z uses default end semantics even inside a multiline scope.
			ch, defaultEnd = '$', true
			i++
		}
		if ch == '\\' {
			if !inClass && i+1 < len(source) && source[i+1] == 'b' {
				for next <= maxSlot && reserved[next] {
					next++
				}
				if next > maxSlot {
					return source, nil
				}
				out.WriteString("(?<" + strconv.Itoa(next) + `>\b)`)
				groups = append(groups, regexp2EndAssertion{group: next, multiline: true})
				next++
				i++
				continue
			}
			out.WriteByte(ch)
			if i+1 < len(source) {
				i++
				out.WriteByte(source[i])
				// The operand of a control escape can itself be a delimiter.
				if source[i] == 'c' && i+1 < len(source) {
					i++
					out.WriteByte(source[i])
				}
			}
			if inClass {
				firstInClass = false
			}
			continue
		}
		if inClass {
			out.WriteByte(ch)
			if ch == ']' && !firstInClass {
				inClass = false
			} else if ch != '^' || !firstInClass || i != classStart+1 {
				firstInClass = false
			}
			continue
		}
		if options.extended && ch == '#' {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				out.WriteString(source[i:])
				break
			}
			out.WriteString(source[i : i+end])
			i += end - 1
			continue
		}
		switch ch {
		case '[':
			inClass, firstInClass = true, true
			classStart = i
		case '(':
			if strings.HasPrefix(source[i:], "(?#") {
				end := strings.IndexByte(source[i+3:], ')')
				if end >= 0 {
					end += i + 3
					unixDollar = source[i:end+1] == regexp2UnixDollarMarker
					out.WriteString(source[i : end+1])
					i = end
					continue
				}
			}
			if i+2 < len(source) && source[i+1] == '?' {
				j, enabled, scopedOptions, sawFlag := i+2, true, options, false
				for j < len(source) && strings.ContainsRune("imnsxduIMNSXDU-+", rune(source[j])) {
					if source[j] == '-' {
						enabled = false
					} else if source[j] == '+' {
						enabled = true
					} else {
						sawFlag = true
						switch source[j] {
						case 'x', 'X':
							scopedOptions.extended = enabled
						case 'm', 'M':
							scopedOptions.multiline = enabled
						}
					}
					j++
				}
				if sawFlag && j < len(source) && (source[j] == ':' || source[j] == ')') {
					if source[j] == ':' {
						optionStack = append(optionStack, options)
					}
					options = scopedOptions
					out.WriteString(source[i : j+1])
					i = j
					continue
				}
			}
			optionStack = append(optionStack, options)
		case ')':
			if len(optionStack) > 0 {
				options = optionStack[len(optionStack)-1]
				optionStack = optionStack[:len(optionStack)-1]
			}
		case '$':
			for next <= maxSlot && reserved[next] {
				next++
			}
			if next > maxSlot {
				// Preserve the existing pattern if no private slot is available.
				return source, nil
			}
			out.WriteString("(?<" + strconv.Itoa(next) + ">")
			multiline := options.multiline && !defaultEnd
			if unixDollar && multiline {
				out.WriteString(`(?:\z|(?=\n))`)
			} else if unixDollar {
				out.WriteString(`(?:\z|(?=\n\z))`)
			} else if multiline {
				out.WriteString(`(?:\z|(?<!\r)(?=\n)|(?=[\r\u0085\u2028\u2029]))`)
			} else {
				// A final CRLF is one terminator; never anchor between its bytes.
				out.WriteString(`(?:\z|(?=\r\n\z)|(?<!\r)(?=\n\z)|(?=[\r\u0085\u2028\u2029]\z))`)
			}
			out.WriteByte(')')
			groups = append(groups, regexp2EndAssertion{group: next, multiline: multiline})
			unixDollar = false
			next++
			continue
		}
		out.WriteByte(ch)
	}
	return out.String(), groups
}

// Only surviving end assertions can make a successful match end-dependent.
// Complete lookaround/attempted-path semantics remain open.
func (p *regexp2Plan) matchRequiresEnd(input string, match *regexp2.Match) bool {
	end := utf8.RuneCountInString(input)
	terminatorStart := -1
	if strings.HasSuffix(input, "\r\n") {
		terminatorStart = end - 2
	} else {
		last, _ := utf8.DecodeLastRuneInString(input)
		switch last {
		case '\n', '\r', '\u0085', '\u2028', '\u2029':
			terminatorStart = end - 1
		}
	}
	for _, assertion := range p.endAssertionGroups {
		group := match.GroupByNumber(assertion.group)
		if group == nil {
			continue
		}
		for _, capture := range group.Captures {
			if capture.Index == end || (!assertion.multiline && terminatorStart >= 0 && capture.Index == terminatorStart) {
				return true
			}
		}
	}
	return false
}
