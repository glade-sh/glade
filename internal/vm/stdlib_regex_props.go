package vm

import (
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Java resolves \p{Name} through Pattern.family. The names below follow that
// order: "gc=", "sc=" and "blk=" keys, "In" blocks, "Is" binary properties,
// general categories and scripts, then the POSIX and java* tables. Each name
// resolves to a set of code points that the translator emits as a plain class.
// Names stay unresolved (and the pattern is rejected) when Java rejects them.

// runeSet is a sorted list of inclusive [lo, hi] pairs.
type runeSet []rune

const maxRune = 0x10FFFF

func runeSetFromTable(t *unicode.RangeTable) runeSet {
	var set runeSet
	for _, r := range t.R16 {
		if r.Stride == 1 {
			set = append(set, rune(r.Lo), rune(r.Hi))
			continue
		}
		for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
			set = append(set, c, c)
		}
	}
	for _, r := range t.R32 {
		if r.Stride == 1 {
			set = append(set, rune(r.Lo), rune(r.Hi))
			continue
		}
		for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
			set = append(set, c, c)
		}
	}
	return normalizeRuneSet(set)
}

func normalizeRuneSet(set runeSet) runeSet {
	n := len(set) / 2
	if n == 0 {
		return nil
	}
	pairs := make([][2]rune, n)
	for i := 0; i < n; i++ {
		pairs[i] = [2]rune{set[2*i], set[2*i+1]}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	out := make(runeSet, 0, len(set))
	cur := pairs[0]
	for _, p := range pairs[1:] {
		if p[0] <= cur[1]+1 {
			if p[1] > cur[1] {
				cur[1] = p[1]
			}
			continue
		}
		out = append(out, cur[0], cur[1])
		cur = p
	}
	return append(out, cur[0], cur[1])
}

func (a runeSet) union(others ...runeSet) runeSet {
	all := append(runeSet(nil), a...)
	for _, o := range others {
		all = append(all, o...)
	}
	return normalizeRuneSet(all)
}

func (a runeSet) complement() runeSet {
	var out runeSet
	next := rune(0)
	for i := 0; i < len(a); i += 2 {
		if a[i] > next {
			out = append(out, next, a[i]-1)
		}
		next = a[i+1] + 1
	}
	if next <= maxRune {
		out = append(out, next, maxRune)
	}
	return out
}

func (a runeSet) subtract(b runeSet) runeSet {
	return intersectRuneSets(a, b.complement())
}

func intersectRuneSets(a, b runeSet) runeSet {
	var out runeSet
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		lo, hi := a[i], a[i+1]
		if b[j] > lo {
			lo = b[j]
		}
		if b[j+1] < hi {
			hi = b[j+1]
		}
		if lo <= hi {
			out = append(out, lo, hi)
		}
		if a[i+1] < b[j+1] {
			i += 2
		} else {
			j += 2
		}
	}
	return out
}

func asciiRanges(pairs ...rune) runeSet { return normalizeRuneSet(runeSet(pairs)) }

func tableSet(names ...string) runeSet {
	var set runeSet
	for _, name := range names {
		if t, ok := unicode.Categories[name]; ok {
			set = append(set, runeSetFromTable(t)...)
		} else if t, ok := unicode.Properties[name]; ok {
			set = append(set, runeSetFromTable(t)...)
		}
	}
	return normalizeRuneSet(set)
}

var (
	javaPropsOnce sync.Once

	javaSetLetter, javaSetDigit, javaSetUnassigned, javaSetAlphabetic runeSet
	javaSetLowercase, javaSetUppercase, javaSetWhiteSpace             runeSet
	javaSetControl, javaSetPunct, javaSetHexDigit, javaSetSpaceChar   runeSet
)

func javaPropsInit() {
	javaPropsOnce.Do(func() {
		javaSetLetter = tableSet("L")
		javaSetDigit = tableSet("Nd")
		assigned := tableSet("L", "M", "N", "P", "S", "Z", "Cc", "Cf", "Co", "Cs")
		javaSetUnassigned = assigned.complement()
		javaSetAlphabetic = tableSet("L", "Nl", "Other_Alphabetic")
		javaSetLowercase = tableSet("Ll", "Other_Lowercase")
		javaSetUppercase = tableSet("Lu", "Other_Uppercase")
		javaSetWhiteSpace = tableSet("White_Space")
		javaSetControl = tableSet("Cc")
		javaSetPunct = tableSet("P")
		javaSetHexDigit = javaSetDigit.union(asciiRanges('0', '9', 'A', 'F', 'a', 'f', 0xFF10, 0xFF19, 0xFF21, 0xFF26, 0xFF41, 0xFF46))
		javaSetSpaceChar = tableSet("Zs", "Zl", "Zp")
	})
}

var javaGeneralCategoryNames = map[string]bool{
	"Cn": true, "Lu": true, "Ll": true, "Lt": true, "Lm": true, "Lo": true, "Mn": true, "Me": true, "Mc": true,
	"Nd": true, "Nl": true, "No": true, "Zs": true, "Zl": true, "Zp": true, "Cc": true, "Cf": true, "Co": true,
	"Cs": true, "Pd": true, "Ps": true, "Pe": true, "Pc": true, "Po": true, "Sm": true, "Sc": true, "Sk": true,
	"So": true, "Pi": true, "Pf": true, "L": true, "M": true, "N": true, "Z": true, "C": true, "P": true, "S": true,
	"LC": true,
}

func javaGeneralCategorySet(name string) (runeSet, bool) {
	if !javaGeneralCategoryNames[name] {
		return nil, false
	}
	javaPropsInit()
	switch name {
	case "Cn":
		return javaSetUnassigned, true
	case "C":
		return tableSet("Cc", "Cf", "Co", "Cs").union(javaSetUnassigned), true
	case "LC":
		return tableSet("Lu", "Ll", "Lt"), true
	}
	return tableSet(name), true
}

// javaPOSIXSet returns the ASCII sets that Java uses for \p{Alpha} and friends,
// or their Unicode forms when (?U) is active.
func javaPOSIXSet(name string, unicodeClass bool) (runeSet, bool) {
	javaPropsInit()
	if unicodeClass {
		switch name {
		case "Lower":
			return javaSetLowercase, true
		case "Upper":
			return javaSetUppercase, true
		case "Alpha":
			return javaSetAlphabetic, true
		case "Digit":
			return javaSetDigit, true
		case "Alnum":
			return javaSetAlphabetic.union(javaSetDigit), true
		case "Punct":
			return javaSetPunct, true
		case "Graph":
			return javaSetSpaceChar.union(javaSetControl, tableSet("Cs"), javaSetUnassigned).complement(), true
		case "Print":
			graph := javaSetSpaceChar.union(javaSetControl, tableSet("Cs"), javaSetUnassigned).complement()
			blank := tableSet("Zs").union(asciiRanges('\t', '\t'))
			return graph.union(blank).subtract(javaSetControl), true
		case "Blank":
			return tableSet("Zs").union(asciiRanges('\t', '\t')), true
		case "Cntrl":
			return javaSetControl, true
		case "XDigit":
			return javaSetHexDigit, true
		case "Space":
			return javaSetWhiteSpace, true
		case "ASCII":
			return asciiRanges(0, 0x7F), true
		}
		return nil, false
	}
	switch name {
	case "Lower":
		return asciiRanges('a', 'z'), true
	case "Upper":
		return asciiRanges('A', 'Z'), true
	case "ASCII":
		return asciiRanges(0, 0x7F), true
	case "Alpha":
		return asciiRanges('A', 'Z', 'a', 'z'), true
	case "Digit":
		return asciiRanges('0', '9'), true
	case "Alnum":
		return asciiRanges('0', '9', 'A', 'Z', 'a', 'z'), true
	case "Punct":
		return asciiRanges('!', '/', ':', '@', '[', '`', '{', '~'), true
	case "Graph":
		return asciiRanges(0x21, 0x7E), true
	case "Print":
		return asciiRanges(0x20, 0x7E), true
	case "Blank":
		return asciiRanges(' ', ' ', '\t', '\t'), true
	case "Cntrl":
		return asciiRanges(0, 0x1F, 0x7F, 0x7F), true
	case "XDigit":
		return asciiRanges('0', '9', 'A', 'F', 'a', 'f'), true
	case "Space":
		return asciiRanges(' ', ' ', '\t', '\r'), true
	}
	return nil, false
}

// javaNamedPropertySet resolves the names Java keeps in Pattern.family's
// property table that are not plain categories or POSIX classes.
func javaNamedPropertySet(name string) (runeSet, bool) {
	javaPropsInit()
	switch name {
	case "javaLowerCase":
		return javaSetLowercase, true
	case "javaUpperCase":
		return javaSetUppercase, true
	case "javaAlphabetic":
		return javaSetAlphabetic, true
	case "javaIdeographic":
		return tableSet("Ideographic"), true
	case "javaTitleCase":
		return tableSet("Lt"), true
	case "javaDigit":
		return javaSetDigit, true
	case "javaDefined":
		return javaSetUnassigned.complement(), true
	case "javaLetter":
		return javaSetLetter, true
	case "javaLetterOrDigit":
		return javaSetLetter.union(javaSetDigit), true
	case "javaJavaIdentifierStart":
		return tableSet("L", "Nl", "Sc", "Pc"), true
	case "javaJavaIdentifierPart":
		return tableSet("L", "Nl", "Sc", "Pc", "Nd", "Mn", "Mc").union(javaIdentifierIgnorableSet()), true
	case "javaUnicodeIdentifierStart":
		return tableSet("L", "Nl", "Other_ID_Start"), true
	case "javaUnicodeIdentifierPart":
		return tableSet("L", "Nl", "Other_ID_Start", "Pc", "Nd", "Mn", "Mc", "Other_ID_Continue").union(javaIdentifierIgnorableSet()), true
	case "javaIdentifierIgnorable":
		return javaIdentifierIgnorableSet(), true
	case "javaSpaceChar":
		return javaSetSpaceChar, true
	case "javaWhitespace":
		return javaSetSpaceChar.subtract(asciiRanges(0xA0, 0xA0, 0x2007, 0x2007, 0x202F, 0x202F)).union(asciiRanges(9, 0xD, 0x1C, 0x1F)), true
	case "javaISOControl":
		return asciiRanges(0, 0x1F, 0x7F, 0x9F), true
	case "javaMirrored":
		return runeSet(javaBidiMirrored), true
	}
	return nil, false
}

func javaIdentifierIgnorableSet() runeSet {
	return asciiRanges(0, 8, 0xE, 0x1B, 0x7F, 0x9F).union(tableSet("Cf"))
}

// javaBinaryPropertySet resolves \p{IsName} property names (case-insensitive).
func javaBinaryPropertySet(name string, caseInsensitive bool) (runeSet, bool) {
	javaPropsInit()
	upper := strings.ToUpper(name)
	if caseInsensitive && (upper == "LOWERCASE" || upper == "UPPERCASE") {
		// With CASE_INSENSITIVE Java resolves these to the Cased property.
		return javaSetLowercase.union(javaSetUppercase, tableSet("Lt")), true
	}
	switch upper {
	case "ALPHABETIC":
		return javaSetAlphabetic, true
	case "ASSIGNED":
		return javaSetUnassigned.complement(), true
	case "CONTROL":
		return javaSetControl, true
	case "EMOJI":
		return runeSet(javaEmojiAll), true
	case "EMOJI_PRESENTATION":
		return runeSet(javaEmojiPresentation), true
	case "EMOJI_MODIFIER":
		return runeSet(javaEmojiModifier), true
	case "EMOJI_MODIFIER_BASE":
		return runeSet(javaEmojiModifierBase), true
	case "EMOJI_COMPONENT":
		return runeSet(javaEmojiComponent), true
	case "EXTENDED_PICTOGRAPHIC":
		return runeSet(javaEmojiExtPict), true
	case "HEXDIGIT", "HEX_DIGIT":
		return javaSetHexDigit, true
	case "IDEOGRAPHIC":
		return tableSet("Ideographic"), true
	case "JOINCONTROL", "JOIN_CONTROL":
		return tableSet("Join_Control"), true
	case "LETTER":
		return javaSetLetter, true
	case "LOWERCASE":
		return javaSetLowercase, true
	case "NONCHARACTERCODEPOINT", "NONCHARACTER_CODE_POINT":
		return tableSet("Noncharacter_Code_Point"), true
	case "TITLECASE":
		return tableSet("Lt"), true
	case "PUNCTUATION":
		return javaSetPunct, true
	case "UPPERCASE":
		return javaSetUppercase, true
	case "WHITESPACE", "WHITE_SPACE":
		return javaSetWhiteSpace, true
	case "WORD":
		return javaSetAlphabetic.union(tableSet("M", "Nd", "Pc", "Join_Control")), true
	case "DIGIT":
		return javaSetDigit, true
	}
	return nil, false
}

var javaScriptAliases = map[string]string{
	"ARAB": "Arabic", "ARMN": "Armenian", "BENG": "Bengali", "BOPO": "Bopomofo", "CYRL": "Cyrillic", "DEVA": "Devanagari",
	"ETHI": "Ethiopic", "GEOR": "Georgian", "GREK": "Greek", "GUJR": "Gujarati", "GURU": "Gurmukhi", "HANG": "Hangul",
	"HANI": "Han", "HEBR": "Hebrew", "HIRA": "Hiragana", "KANA": "Katakana", "KHMR": "Khmer", "KNDA": "Kannada",
	"LAOO": "Lao", "LATN": "Latin", "MLYM": "Malayalam", "MONG": "Mongolian", "MYMR": "Myanmar", "ORYA": "Oriya",
	"SINH": "Sinhala", "SYRC": "Syriac", "TAML": "Tamil", "TELU": "Telugu", "THAA": "Thaana", "THAI": "Thai",
	"TIBT": "Tibetan", "ZINH": "Inherited", "QAAI": "Inherited", "ZYYY": "Common",
}

func javaScriptSet(name string) (runeSet, bool) {
	upper := strings.ToUpper(name)
	if full, ok := javaScriptAliases[upper]; ok {
		name = full
	}
	for scriptName, table := range unicode.Scripts {
		if strings.EqualFold(scriptName, name) {
			return runeSetFromTable(table), true
		}
	}
	return nil, false
}

var (
	javaBlockIndexOnce sync.Once
	javaBlockIndex     map[string]javaUnicodeBlockRange
)

func javaBlockSet(name string) (runeSet, bool) {
	javaBlockIndexOnce.Do(func() {
		javaBlockIndex = map[string]javaUnicodeBlockRange{}
		for _, block := range javaUnicodeBlocks {
			upper := strings.ToUpper(block.name)
			keys := []string{
				upper,
				strings.ReplaceAll(upper, " ", ""),
				strings.ReplaceAll(strings.ReplaceAll(upper, " ", "_"), "-", "_"),
			}
			if upper == "GREEK AND COPTIC" {
				// Java names this block GREEK; only the spaced and joined forms alias it.
				keys = keys[:2]
			}
			for _, key := range keys {
				if _, exists := javaBlockIndex[key]; !exists {
					javaBlockIndex[key] = block
				}
			}
		}
		javaBlockIndex["GREEK"] = javaBlockIndex["GREEKANDCOPTIC"]
		javaBlockIndex["COMBINING_MARKS_FOR_SYMBOLS"] = javaBlockIndex["COMBININGDIACRITICALMARKSFORSYMBOLS"]
		javaBlockIndex["COMBININGMARKSFORSYMBOLS"] = javaBlockIndex["COMBININGDIACRITICALMARKSFORSYMBOLS"]
	})
	block, ok := javaBlockIndex[strings.ToUpper(name)]
	if !ok {
		return nil, false
	}
	return runeSet{block.lo, block.hi}, true
}

// javaPropertyError is the description Java reports for an unresolved name.
type javaPropertyError struct {
	description string
}

// javaPropertySet resolves the text between \p{ and } the way Pattern.family
// does. unicodeClass is true under (?U). A nil set with a non-nil error means
// Java rejects the name.
func javaPropertySet(name string, unicodeClass, caseInsensitive bool) (runeSet, *javaPropertyError) {
	javaPropsInit()
	unknown := &javaPropertyError{description: "Unknown character property name {" + name + "}"}
	if eq := strings.IndexByte(name, '='); eq >= 0 {
		key, value := name[:eq], name[eq+1:]
		var set runeSet
		var ok bool
		switch key {
		case "sc", "script":
			set, ok = javaScriptSet(value)
		case "blk", "block":
			set, ok = javaBlockSet(value)
		case "gc", "general_category":
			set, ok = javaGeneralCategorySet(value)
		default:
			return nil, &javaPropertyError{description: "Unknown Unicode property {name=<" + key + ">, value=<" + value + ">}"}
		}
		if !ok {
			return nil, &javaPropertyError{description: "Unknown Unicode property {name=<" + key + ">, value=<" + value + ">}"}
		}
		return set, nil
	}
	if strings.HasPrefix(name, "In") && len(name) > 2 {
		if set, ok := javaBlockSet(name[2:]); ok {
			return set, nil
		}
		return nil, unknown
	}
	if strings.HasPrefix(name, "Is") && len(name) > 2 {
		short := name[2:]
		if set, ok := javaBinaryPropertySet(short, caseInsensitive); ok {
			return set, nil
		}
		if set, ok := javaGeneralCategorySet(short); ok {
			return set, nil
		}
		if set, ok := javaPOSIXSet(short, unicodeClass); ok {
			return set, nil
		}
		if set, ok := javaNamedPropertySet(short); ok {
			return set, nil
		}
		if set, ok := javaScriptSet(short); ok {
			return set, nil
		}
		return nil, unknown
	}
	if set, ok := javaPOSIXSet(name, unicodeClass); ok {
		return set, nil
	}
	if set, ok := javaGeneralCategorySet(name); ok {
		return set, nil
	}
	if set, ok := javaNamedPropertySet(name); ok {
		return set, nil
	}
	return nil, unknown
}

// runeSetClass renders a set as a regexp2 character class.
func runeSetClass(set runeSet) string {
	var out strings.Builder
	out.WriteByte('[')
	writeRune := func(r rune) {
		switch {
		case r < 0x80:
			out.WriteString(`\x`)
			out.WriteString(string("0123456789ABCDEF"[r>>4]))
			out.WriteString(string("0123456789ABCDEF"[r&15]))
		default:
			out.WriteRune(r)
		}
	}
	emit := func(lo, hi rune) {
		writeRune(lo)
		if hi != lo {
			out.WriteByte('-')
			writeRune(hi)
		}
	}
	for i := 0; i < len(set); i += 2 {
		lo, hi := set[i], set[i+1]
		if hi < 0xD800 || lo > 0xDFFF {
			emit(lo, hi)
			continue
		}
		if lo < 0xD800 {
			emit(lo, 0xD7FF)
		}
		if hi > 0xDFFF {
			emit(0xE000, hi)
		}
	}
	out.WriteByte(']')
	return out.String()
}
