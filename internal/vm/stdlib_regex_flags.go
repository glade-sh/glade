package vm

import (
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

const regexp2UnixDollarMarker = "(?#__gladeRegexUnixDollar)"

type javaRegexOptions struct {
	unixLines   bool
	dotall      bool
	extended    bool
	multiline   bool
	caseInsens  bool
	unicodeCase bool
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

// regexTraits describes the subject text, so the translator can keep regexp2's
// faster native forms whenever Java and regexp2 agree on that text.
type regexTraits struct {
	nonASCII     bool // text holds a code point above U+007F
	foldUnsafe   bool // text holds U+0130, U+017F or U+212A, which can fold onto ASCII letters
	wordNonASCII bool // text holds a non-ASCII character that regexp2 treats as a word character
	lineTerms    bool // text holds \r, U+0085, U+2028 or U+2029
	longText     bool // text is long enough to overflow Java's loop recursion
}

// regexTraitsAll is the conservative choice when no subject text is known.
var regexTraitsAll = regexTraits{nonASCII: true, foldUnsafe: true, wordNonASCII: true, lineTerms: true}

func regexTraitsOf(input string) regexTraits {
	var t regexTraits
	for i := 0; i < len(input); {
		b := input[i]
		switch {
		case b >= 0x80:
			t.nonASCII = true
			r, size := utf8.DecodeRuneInString(input[i:])
			switch {
			case r == 0x85 || r == 0x2028 || r == 0x2029:
				t.lineTerms = true
			case r == 0x130 || r == 0x212A || r == 0x17F:
				t.foldUnsafe = true
			}
			if !t.wordNonASCII && regexp2WordRune(r) {
				t.wordNonASCII = true
			}
			i += size
			if t.lineTerms && t.foldUnsafe && t.wordNonASCII {
				return t
			}
			continue
		case b == '\r':
			t.lineTerms = true
		}
		i++
	}
	return t
}

// regexp2WordRune mirrors regexp2's \w and \b word class for one rune.
func regexp2WordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || unicode.IsDigit(r) || unicode.Is(unicode.Pc, r) || r == 0x200C || r == 0x200D
}

// regexTraitsFor returns the traits of input that source can depend on. The
// subject text is scanned only for a pattern that reads those traits, and the
// scan is remembered for the next call on the same text, so a find loop over a
// long input does not rescan it for every match.
func regexTraitsFor(source, input string) regexTraits {
	needFold := regexSourceHasInlineFlag(source, 'i')
	needWord := strings.Contains(source, `\b`) || strings.Contains(source, `\B`)
	needNonASCII := needFold || needWord
	needLineTerms := strings.Contains(source, "^") && regexSourceHasInlineFlag(source, 'm')
	needLong := len(input) >= regexLoopMinInput && strings.Contains(source, ")") && strings.ContainsAny(source, "*+{")
	if !needNonASCII && !needLineTerms && !needLong {
		return regexTraits{}
	}
	var out regexTraits
	if needNonASCII || needLineTerms {
		scanned := regexTraitsOfCached(input)
		out.nonASCII = needFold && scanned.nonASCII
		out.foldUnsafe = needFold && scanned.foldUnsafe
		out.wordNonASCII = needWord && scanned.wordNonASCII
		out.lineTerms = needLineTerms && scanned.lineTerms
	}
	if needLong && regexSourceHasLoopGroups(source) {
		// The parser decides whether a group is a recursive loop, and only when
		// the text is long enough to matter.
		out.longText = true
	}
	return out
}

var (
	regexLoopSourceMu    sync.Mutex
	regexLoopSourceCache = map[string]bool{}
)

// regexSourceHasLoopGroups memoises the parse for the sources it has seen.
func regexSourceHasLoopGroups(source string) bool {
	regexLoopSourceMu.Lock()
	known, ok := regexLoopSourceCache[source]
	regexLoopSourceMu.Unlock()
	if ok {
		return known
	}
	has := len(javaRegexLoopGroups(javaRegexRemoveQuoting(source))) > 0
	regexLoopSourceMu.Lock()
	if len(regexLoopSourceCache) >= 1024 {
		regexLoopSourceCache = map[string]bool{}
	}
	regexLoopSourceCache[source] = has
	regexLoopSourceMu.Unlock()
	return has
}

type regexTraitsMemoEntry struct {
	input  string
	traits regexTraits
}

var regexTraitsMemo atomic.Pointer[regexTraitsMemoEntry]

func regexTraitsOfCached(input string) regexTraits {
	if len(input) < 128 {
		return regexTraitsOf(input)
	}
	if entry := regexTraitsMemo.Load(); entry != nil && len(entry.input) == len(input) && entry.input == input {
		return entry.traits
	}
	traits := regexTraitsOf(input)
	regexTraitsMemo.Store(&regexTraitsMemoEntry{input: input, traits: traits})
	return traits
}

// regexSourceHasInlineFlag reports whether a (?flags) group in source sets flag.
func regexSourceHasInlineFlag(source string, flag byte) bool {
	for i := 0; i+2 < len(source); i++ {
		if source[i] == '\\' {
			i++
			continue
		}
		if source[i] != '(' || source[i+1] != '?' {
			continue
		}
		for j := i + 2; j < len(source); j++ {
			c := source[j]
			if c == flag {
				return true
			}
			if !strings.ContainsRune("idmsucxU-", rune(c)) {
				break
			}
		}
	}
	return false
}

func regexSourceNeedsExplicitFold(source string, traits regexTraits) bool {
	if !regexSourceHasInlineFlag(source, 'i') {
		return false
	}
	// For an ASCII-only pattern, regexp2's own folding differs from Java's only
	// on U+0130, U+017F and U+212A, unless the u flag asks for Unicode folding.
	ascii := true
	for i := 0; i < len(source); i++ {
		if source[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii && !strings.Contains(source, `\u`) && !strings.Contains(source, `\x`) {
		if regexSourceHasInlineFlag(source, 'u') {
			if !traits.nonASCII {
				return false
			}
		} else if !traits.foldUnsafe {
			return false
		}
	}
	// A back reference compares captured text; regexp2's own folding stays.
	for i := 0; i+1 < len(source); i++ {
		if source[i] == '\\' {
			if source[i+1] >= '1' && source[i+1] <= '9' || source[i+1] == 'k' {
				return false
			}
			i++
		}
	}
	return true
}

const (
	regexCaretAnyTerminator = `(?:(?!\z)(?:\A|(?<=[\n\u0085\u2028\u2029])|(?<=\r)(?!\n)))`
	regexCaretUnixLines     = `(?:(?!\z)(?:\A|(?<=\n)))`
	regexASCIIWordBoundary  = `(?:(?<=[A-Za-z0-9_])(?![A-Za-z0-9_])|(?<![A-Za-z0-9_])(?=[A-Za-z0-9_]))`
	regexASCIINotBoundary   = `(?:(?<=[A-Za-z0-9_])(?=[A-Za-z0-9_])|(?<![A-Za-z0-9_])(?![A-Za-z0-9_]))`
)

// R069/R070/R273 exercise Java's d/u options. d controls line terminators and
// must be translated at each scope. Java folds case for ASCII letters only
// unless u is set, and ^ with m follows Java's line terminators; both differ
// from regexp2, so they are translated here when the subject text needs it.
func rewriteJavaRegexFlagsForRegexp2(source string, traits regexTraits, unicodeClasses bool) string {
	var out strings.Builder
	options := javaRegexOptions{}
	explicitFold := regexSourceNeedsExplicitFold(source, traits)
	var stack []javaRegexOptions
	var classes javaRegexClassState
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch == '\\' {
			if i+1 < len(source) && !classes.inside() && (source[i+1] == 'b' || source[i+1] == 'B') && traits.wordNonASCII && !unicodeClasses && !(i+2 < len(source) && source[i+2] == '{') {
				if source[i+1] == 'b' {
					out.WriteString(regexASCIIWordBoundary)
				} else {
					out.WriteString(regexASCIINotBoundary)
				}
				i++
				continue
			}
			if explicitFold && options.caseInsens && !classes.inside() && i+1 < len(source) {
				if consumed, text, ok := foldEscapeAtom(source[i:], options.unicodeCase); ok {
					out.WriteString(text)
					i += consumed - 1
					continue
				}
				switch source[i+1] {
				case 'p', 'P', 'x':
					if i+2 < len(source) && source[i+2] == '{' {
						if end := strings.IndexByte(source[i:], '}'); end >= 0 {
							out.WriteString(source[i : i+end+1])
							i += end
							continue
						}
					}
				}
			}
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
		if explicitFold && options.caseInsens && ch == '[' && !classes.inside() {
			if end := javaRegexCharClassEnd(source, i); end >= 0 {
				out.WriteString(foldClassText(source[i:end+1], options.unicodeCase, options.extended))
				i = end
				continue
			}
		}
		classes.advance(source, i)
		if classes.inside() {
			if options.extended && ch != '[' && ch != ']' {
				// Java applies the x flag inside classes: whitespace is ignored and
				// # starts a comment (R-rows d9_x_class_*).
				if javaASCIISpace(rune(ch)) {
					continue
				}
				if ch == '#' {
					if end := strings.IndexByte(source[i:], '\n'); end >= 0 {
						i += end
						continue
					}
					break
				}
			}
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
			if strings.HasPrefix(source[i:], "(?<") && i+3 < len(source) && source[i+3] != '=' && source[i+3] != '!' {
				if end := strings.IndexByte(source[i:], '>'); end >= 0 {
					out.WriteString(source[i : i+end+1])
					i += end
					stack = append(stack, options)
					continue
				}
			}
			if strings.HasPrefix(source[i:], "(?") {
				j, enabled, scoped, sawFlag := i+2, true, options, false
				var flags strings.Builder
				for j < len(source) && strings.ContainsRune("idmsucxU-", rune(source[j])) {
					flag := source[j]
					if flag == '-' {
						enabled = false
						flags.WriteByte(flag)
					} else {
						sawFlag = true
						switch flag {
						case 'd':
							scoped.unixLines = enabled
						case 'u':
							scoped.unicodeCase = enabled
						case 'U':
							scoped.unicodeCase = enabled
						case 'c':
							// CANON_EQ is not modelled.
						case 'i':
							scoped.caseInsens = enabled
							if !explicitFold {
								flags.WriteByte(flag)
							}
						default:
							flags.WriteByte(flag)
							if flag == 's' {
								scoped.dotall = enabled
							} else if flag == 'x' {
								scoped.extended = enabled
							} else if flag == 'm' {
								scoped.multiline = enabled
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
		} else if ch == '^' && options.multiline {
			switch {
			case options.unixLines:
				out.WriteString(regexCaretUnixLines)
			case traits.lineTerms:
				out.WriteString(regexCaretAnyTerminator)
			default:
				// Only \n ends a line here, so regexp2's multiline anchor works once
				// the position at the end of the text is excluded.
				out.WriteString(`(?:(?!\z)^)`)
			}
			continue
		} else if ch == '$' && options.unixLines {
			out.WriteString(regexp2UnixDollarMarker)
		} else if explicitFold && options.caseInsens && !strings.ContainsRune("()[]{}.*+?^$|", rune(ch)) && !(options.extended && javaASCIISpace(rune(ch))) {
			r, size := utf8.DecodeRuneInString(source[i:])
			if variants := javaFoldVariants(r, options.unicodeCase); len(variants) > 1 {
				out.WriteString(runeSetClassFromList(variants))
				i += size - 1
				continue
			}
		}
		out.WriteByte(ch)
	}
	return out.String()
}
