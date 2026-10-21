package vm

import (
	"strings"

	"github.com/dlclark/regexp2"
)

type regexCaptureOptions struct {
	extended bool
}

// Apex counts capturing parentheses from left to right, including named
// captures. regexp2 assigns named slots after unnamed slots. Translate only
// public access to those slots; matching and backreference compilation keep
// their existing engine representation.
func regexp2OpeningGroupNumbers(source string, re *regexp2.Regexp, expected int) ([]int, bool) {
	out := []int{0}
	seen := map[int]bool{0: true}
	appendGroup := func(number int) bool {
		if number < 1 || seen[number] {
			return false
		}
		seen[number] = true
		out = append(out, number)
		return true
	}
	var classes javaRegexClassState
	var stack []regexCaptureOptions
	options := regexCaptureOptions{}
	quoted, unnamed := false, 0
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if quoted {
			if ch == '\\' && i+1 < len(source) && source[i+1] == 'E' {
				quoted = false
				i++
			}
			continue
		}
		if ch == '\\' && i+1 < len(source) {
			i++
			if source[i] == 'Q' {
				quoted = true
			} else if source[i] == 'c' && i+1 < len(source) {
				// A control escape's operand may itself be a delimiter.
				i++
			}
			continue
		}
		if options.extended && !classes.inside() && ch == '#' {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				break
			}
			i += end
			continue
		}
		inside := classes.inside()
		classes.advance(source, i)
		if inside || classes.inside() {
			continue
		}
		if ch == ')' {
			if len(stack) > 0 {
				options = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if ch != '(' {
			continue
		}
		if strings.HasPrefix(source[i:], "(?#") {
			end := strings.IndexByte(source[i+3:], ')')
			if end < 0 {
				return nil, false
			}
			i += end + 3
			continue
		}
		if i+1 >= len(source) || source[i+1] != '?' {
			unnamed++
			if !appendGroup(unnamed) {
				return nil, false
			}
			stack = append(stack, options)
			continue
		}
		if strings.HasPrefix(source[i:], "(?<") && i+3 < len(source) && source[i+3] != '=' && source[i+3] != '!' {
			end := strings.IndexByte(source[i+3:], '>')
			if end < 0 {
				return nil, false
			}
			end += i + 3
			name := source[i+3 : end]
			// Explicit numeric slots and balancing groups are engine-specific;
			// retain their previous mapping rather than reinterpret them here.
			if len(name) == 0 || name[0] >= '0' && name[0] <= '9' || strings.Contains(name, "-") || !appendGroup(re.GroupNumberFromName(name)) {
				return nil, false
			}
			stack = append(stack, options)
			i = end
			continue
		}
		j, scoped, enabled, sawFlag := i+2, options, true, false
		for j < len(source) && strings.ContainsRune("idmsuxU-", rune(source[j])) {
			switch source[j] {
			case '-':
				enabled = false
			default:
				sawFlag = true
				switch source[j] {
				case 'x':
					scoped.extended = enabled
				}
			}
			j++
		}
		if sawFlag && j < len(source) && (source[j] == ':' || source[j] == ')') {
			if source[j] == ':' {
				stack = append(stack, options)
			}
			options = scoped
			i = j
			continue
		}
		if !strings.HasPrefix(source[i:], "(?:") && !strings.HasPrefix(source[i:], "(?=") && !strings.HasPrefix(source[i:], "(?!") && !strings.HasPrefix(source[i:], "(?<=") && !strings.HasPrefix(source[i:], "(?<!") && !strings.HasPrefix(source[i:], "(?>") {
			return nil, false
		}
		stack = append(stack, options)
	}
	return out, len(out) == expected
}
