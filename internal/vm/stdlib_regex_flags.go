package vm

import "strings"

const regexp2UnixDollarMarker = "(?#__gladeRegexUnixDollar)"

type javaRegexOptions struct {
	unixLines bool
	dotall    bool
	extended  bool
}

// K006-K023: retain every nesting level and treat a leading ] as a literal,
// including after ^. Callers skip escaped brackets before advancing this state.
type javaRegexClassState struct {
	starts []int
}

func (c javaRegexClassState) inside() bool {
	return len(c.starts) > 0
}

func (c *javaRegexClassState) advance(source string, i int) {
	switch source[i] {
	case '[':
		c.starts = append(c.starts, i)
	case ']':
		depth := len(c.starts)
		if depth == 0 {
			return
		}
		start := c.starts[depth-1]
		if i == start+1 || (i == start+2 && source[start+1] == '^') {
			return
		}
		c.starts = c.starts[:depth-1]
	}
}

// R069/R070/R273 exercise Java's d/u options. regexp2 already folds Unicode
// under i; d controls line terminators and must be translated at each scope.
func rewriteJavaRegexFlagsForRegexp2(source string) string {
	var out strings.Builder
	options := javaRegexOptions{}
	var stack []javaRegexOptions
	var classes javaRegexClassState
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch == '\\' {
			out.WriteByte(ch)
			if i+1 < len(source) {
				i++
				out.WriteByte(source[i])
				if source[i] == 'c' && i+1 < len(source) {
					i++
					out.WriteByte(source[i])
				}
			}
			continue
		}
		if options.extended && !classes.inside() && ch == '#' {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				out.WriteString(source[i:])
				out.WriteByte('\n')
				break
			}
			out.WriteString(source[i : i+end+1])
			i += end
			continue
		}
		classes.advance(source, i)
		if classes.inside() {
			out.WriteByte(ch)
			continue
		}
		if ch == '(' {
			if strings.HasPrefix(source[i:], "(?#") {
				if end := strings.IndexByte(source[i+3:], ')'); end >= 0 {
					end += i + 3
					out.WriteString(source[i : end+1])
					i = end
					continue
				}
			}
			if strings.HasPrefix(source[i:], "(?") {
				j, enabled, scoped, sawFlag := i+2, true, options, false
				var flags strings.Builder
				for j < len(source) && strings.ContainsRune("idmsuxU-", rune(source[j])) {
					flag := source[j]
					if flag == '-' {
						enabled = false
						flags.WriteByte(flag)
					} else {
						sawFlag = true
						switch flag {
						case 'd':
							scoped.unixLines = enabled
						case 'u', 'U':
							// Unicode classes are rewritten by the shared class pass.
						default:
							flags.WriteByte(flag)
							if flag == 's' {
								scoped.dotall = enabled
							} else if flag == 'x' {
								scoped.extended = enabled
							}
						}
					}
					j++
				}
				if sawFlag && j < len(source) && (source[j] == ')' || source[j] == ':') {
					if source[j] == ':' {
						stack = append(stack, options)
					}
					options = scoped
					engineFlags := strings.TrimSuffix(flags.String(), "-")
					if engineFlags != "" {
						out.WriteString("(?" + engineFlags + string(source[j]))
					} else if source[j] == ':' {
						out.WriteString("(?:")
					}
					i = j
					continue
				}
			}
			stack = append(stack, options)
		} else if ch == ')' && len(stack) > 0 {
			options = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		} else if ch == '.' && !options.dotall {
			if options.unixLines {
				out.WriteString(`[^\n]`)
			} else {
				out.WriteString(`[^\n\r\u0085\u2028\u2029]`)
			}
			continue
		} else if ch == '$' && options.unixLines {
			out.WriteString(regexp2UnixDollarMarker)
		}
		out.WriteByte(ch)
	}
	return out.String()
}
