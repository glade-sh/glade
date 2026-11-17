package vm

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func runRegexApex(t *testing.T, body string) {
	t.Helper()
	program, err := CompileAnonymous(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

// Java's $ also matches before a final line terminator; RE2's does not. The
// linear precheck must not answer "no match" for these (F1).
func TestRegexLinearPrecheckKeepsJavaDollarSemantics(t *testing.T) {
	runRegexApex(t, `
String a = 'aaaaaaaaaaaaaaaaaaaa';
System.assertEquals(true, Pattern.compile('(a+)+$').matcher(a + '\n').find());
System.assertEquals(true, Pattern.compile('(?:a+)+b$').matcher('aaaaaaaaaaaaaaaaaaa' + 'b\n').find());
System.assertEquals(true, Pattern.compile('(?m)(a+)+$').matcher(a + '\r\nb').find());
System.assertEquals(true, Pattern.matches('(a+)+$\n', a + '\n'));
System.assertEquals('X\n', (a + '\n').replaceAll('(a+)+$', 'X'));
System.assertEquals(2, (a + '\n').split('(a+)+$').size());
System.assertEquals(true, Pattern.compile('^(\\w+\\s?)*$').matcher('hello world today\n').find());
System.assertEquals(true, Pattern.compile('^(\\w+)+$').matcher('helloworldtodayxx\n').find());
System.assertEquals(true, Pattern.compile('(?m)(\\w+\\s?)+$').matcher('aaaa bbbb cccc dddd\r\nzzz').find());
System.assertEquals(true, Pattern.compile('(?i)(\\w+\\s?)+$').matcher('aaaa bbbb cccc dddd\n').find());
`)
}

func TestRegexLinearAnchorsSafe(t *testing.T) {
	for source, want := range map[string]bool{
		`(a+)+b`:       true,
		`^(\w+\s?)*x`:  true,
		`(a+)+$`:       false,
		`(a+)+\Z`:      false,
		`(a+)+\z`:      false,
		`\A(a+)+b`:     false,
		`(?m)^(a+)+b`:  false,
		`(?im)^(a+)+b`: false,
		`(?i)(a+)+b`:   true,
		`(a+)+\$b`:     true,
		`(?d)(a+)+b`:   false,
		`(?U)(\d+)+\b`: false,
		`(a+)+\B`:      false,
	} {
		if got := regexLinearAnchorsSafe(source); got != want {
			t.Errorf("regexLinearAnchorsSafe(%q) = %v, want %v", source, got, want)
		}
	}
}

// A trailing literal hyphen must stay a literal after the case variants are added (F2).
func TestRegexFoldClassTrailingHyphen(t *testing.T) {
	runRegexApex(t, `
System.assertEquals('éX', 'é-b'.replaceAll('(?i)[a-z_-]+', 'X'));
System.assertEquals(true, Pattern.compile('(?i)[A-Za-z0-9_-]+').matcher('é-b_9').find());
System.assertEquals('éX', 'é-b_9'.replaceAll('(?i)[A-Za-z0-9_-]+', 'X'));
System.assertEquals('é 5%', 'José 5%'.replaceAll('(?i)[a-z#-]', ''));
System.assertEquals('xé', 'axé-'.replaceAll('(?i)[a-]', ''));
`)
}

func TestRegexTraitsNarrowSlowPaths(t *testing.T) {
	latin := regexTraitsOf("café straße")
	if latin.foldUnsafe || !latin.wordNonASCII || !latin.nonASCII {
		t.Errorf("latin traits = %+v", latin)
	}
	punct := regexTraitsOf("a – b € c")
	if punct.foldUnsafe || punct.wordNonASCII {
		t.Errorf("punctuation traits = %+v", punct)
	}
	for _, s := range []string{"İ", "K"} {
		if !regexTraitsOf(s).foldUnsafe {
			t.Errorf("%U not fold unsafe", []rune(s)[0])
		}
	}
	if got := regexTraitsFor(`(?i)hello`, "café hello"); got.foldUnsafe || got.nonASCII != true {
		t.Errorf("(?i) latin traits = %+v", got)
	}
	if regexSourceNeedsExplicitFold(`(?i)hello`, regexTraitsFor(`(?i)hello`, "café hello")) {
		t.Error("ASCII (?i) pattern on a Latin subject must keep regexp2's fold")
	}
	if !regexSourceNeedsExplicitFold(`(?i)hello`, regexTraitsFor(`(?i)hello`, "Khello")) {
		t.Error("ASCII (?i) pattern on a Kelvin sign subject needs the explicit fold")
	}
	if !regexSourceNeedsExplicitFold(`(?iu)hello`, regexTraitsFor(`(?iu)hello`, "ſhello")) {
		t.Error("(?iu) pattern on a non-ASCII subject needs the explicit fold")
	}
	if strings.Contains(compileForTest(t, `\bfoo\b`, "– foo"), "A-Za-z0-9_") {
		t.Error("punctuation-only subject should keep the native \\b")
	}
	if !strings.Contains(compileForTest(t, `\bfoo\b`, "café foo"), "A-Za-z0-9_") {
		t.Error("accented subject needs the lookaround \\b")
	}
}

func compileForTest(t *testing.T, source, input string) string {
	t.Helper()
	out, _, err := compileRegexp2SourceLoops("Pattern.compile", source, regexTraitsFor(source, input))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The linear precheck may say "no match" only when the backtracking engine
// finds none either. The sweep covers anchors and line terminators (F1).
func TestRegexLinearPrecheckNeverContradictsBacktracking(t *testing.T) {
	prefixes := []string{"", "^", "(?m)", "(?m)^", "(?i)", "\\A", "(?d)"}
	bodies := []string{`(a+)+`, `(\w+\s?)*`, `(\w+)+`, `([^,]+,?)+`, `(?:a+)+b`, `(?:a+)+`}
	tails := []string{"", "$", "$\n", "b$", "\\z", "\\Z", "x"}
	base := strings.Repeat("a", 20)
	subjects := []string{}
	for _, tail := range []string{"", "\n", "\r\n", "b", "b\n", "\r\nb", "\u0085", " ", " x\n", "b\r\n", ",a,b\n"} {
		subjects = append(subjects, base+tail)
	}
	checked := 0
	for _, pre := range prefixes {
		for _, body := range bodies {
			for _, tail := range tails {
				source := pre + body + tail
				for _, s := range subjects {
					plan, err := compileRegexp2PlanForInput("Pattern.compile", source, s)
					if err != nil {
						continue
					}
					checked++
					if !plan.noLinearMatch(s) {
						continue
					}
					plan.re.MatchTimeout = 5 * time.Second
					m, err := plan.re.FindStringMatch(s)
					if err == nil && m != nil {
						t.Errorf("%q on %q: precheck says no match, backtracking matches", source, s)
					}
				}
			}
		}
	}
	if checked < 500 {
		t.Fatalf("only %d cases ran", checked)
	}
}

func TestRegexLinearPrecheckKeepsUnicodeWordBoundary(t *testing.T) {
	runRegexApex(t, `
String s = 'aaaaaaaaaaaaaaaa' + '\u0660\u0661';
System.assertEquals(true, Pattern.compile('(?U)(\\d+)+\\b').matcher(s).find());
`)
}

// A failed unanchored find always sets hitEnd; a ^-anchored pattern makes one
// attempt, and hitEnd is true only when that attempt reads past the end (F4).
func TestRegexHitEndAfterLinearPrecheckMiss(t *testing.T) {
	runRegexApex(t, `
Matcher m1 = Pattern.compile('^(a+)+b').matcher('a'.repeat(24));
System.assertEquals(false, m1.find());
System.assertEquals(true, m1.hitEnd());
Matcher m2 = Pattern.compile('^(a+)+b').matcher('a'.repeat(5));
System.assertEquals(false, m2.find());
System.assertEquals(true, m2.hitEnd());
Matcher m3 = Pattern.compile('^(a+)+b').matcher('a'.repeat(23) + 'c');
System.assertEquals(false, m3.find());
System.assertEquals(false, m3.hitEnd());
Matcher m4 = Pattern.compile('(a+)+b').matcher('a'.repeat(23) + 'c');
System.assertEquals(false, m4.find());
System.assertEquals(true, m4.hitEnd());
`)
}

// Java keeps its Start wrapper unless the root node is a bare ^, and (?i)
// without u folds ASCII only (H1, H2; native rows d15_* in regex_java_divergence).
func TestRegexHitEndRootAnchorAndLongSFold(t *testing.T) {
	runRegexApex(t, `
Matcher g = Pattern.compile('(^(a+)+b)').matcher('a'.repeat(23) + 'c');
System.assertEquals(false, g.find());
System.assertEquals(true, g.hitEnd());
Matcher n = Pattern.compile('(?:^(a+)+b)').matcher('a'.repeat(23) + 'c');
System.assertEquals(false, n.find());
System.assertEquals(true, n.hitEnd());
Matcher f = Pattern.compile('(?i)^(s+)+x').matcher('s'.repeat(20) + '\u017F');
System.assertEquals(false, f.find());
System.assertEquals(false, f.hitEnd());
`)
}

func TestRegexRootIsBareCaret(t *testing.T) {
	for source, want := range map[string]bool{
		`^(a+)+b`: true, `(?i)^(a+)+b`: true, `(^(a+)+b)`: false, `(?:^(a+)+b)`: false,
		`^a|(b+)+c`: false, `^(a|b)+c`: true, `^\Qa\E(a+)+b`: true, `^(a+)+[]|]b`: true, `^(a+)+[^]|]b`: true, `^(a+)+[a[|]]b`: true, `(?x) ^(a+)+b`: true, `(?x) (^(a+)+b)`: false, `(a+)+b`: false, `^[|](a+)+b`: true, `(?i:^a)`: false,
	} {
		if got := regexRootIsBareCaret(source); got != want {
			t.Errorf("regexRootIsBareCaret(%q) = %v, want %v", source, got, want)
		}
	}
}

// (?x) patterns get the linear precheck too (D6). The plan is checked
// directly, so no wall-clock bound is needed.
func TestRegexExtendedPatternsGetLinearPrecheck(t *testing.T) {
	miss := strings.Repeat("a", 23) + "c"
	for _, tc := range []struct {
		pattern  string
		anchored bool
	}{
		{"(?x) ^(a+)+b # comment", true},
		{"(?x) (a+)+ b", false},
		{"(?ix) ^ ( A+ )+ \\  b", true},
	} {
		plan, err := compileRegexp2PlanForInput("Pattern.compile", tc.pattern, miss)
		if err != nil {
			t.Fatal(err)
		}
		if plan.linear == nil {
			t.Fatalf("%q: no linear precheck", tc.pattern)
		}
		if tc.anchored && plan.linearProg == nil {
			t.Fatalf("%q: no linear program", tc.pattern)
		}
		if !plan.noLinearMatch(miss) {
			t.Errorf("%q: precheck should report no match", tc.pattern)
		}
		if regexNormalizeExtended(tc.pattern) == tc.pattern {
			t.Errorf("%q: not normalized", tc.pattern)
		}
	}
	runRegexApex(t, `
String s = 'a'.repeat(23) + 'c';
Matcher m = Pattern.compile('(?x) ^(a+)+b # comment').matcher(s);
System.assertEquals(false, m.find());
System.assertEquals(false, m.hitEnd());
System.assertEquals(false, Pattern.compile('(?x) (a+)+ b').matcher(s).find());
System.assertEquals(true, Pattern.compile('(?x) (a+)+ b # c').matcher('a'.repeat(23) + 'b').find());
System.assertEquals(true, Pattern.compile('(?ix) ^ ( A+ )+ \\  b').matcher('a'.repeat(23) + ' b').find());
System.assertEquals(false, Pattern.compile('^\\Qa\\E(a+)+b').matcher(s).find());
Matcher q = Pattern.compile('^\\Qa\\E(a+)+b').matcher(s);
q.find();
System.assertEquals(false, q.hitEnd());
Matcher c = Pattern.compile('^(a+)+[]|]b').matcher(s);
c.find();
System.assertEquals(false, c.hitEnd());
`)
}

// Java ends a (?x) comment at \n, \r, U+0085, U+2028 or U+2029 (only \n under
// d), and reads whitespace inside tokens (native rows d16_*).
func TestRegexExtendedCommentsAndSplitTokens(t *testing.T) {
	runRegexApex(t, `
System.assertEquals(true, Pattern.compile('(?x)a #c\r|b').matcher('b').find());
System.assertEquals(true, Pattern.compile('(?x)a #c\u2028|b').matcher('b').find());
System.assertEquals(false, Pattern.compile('(?xd)a #c\r|b').matcher('b').find());
Matcher g = Pattern.compile('(?x)( ?:a)').matcher('a');
System.assertEquals(true, g.find());
System.assertEquals(0, g.groupCount());
System.assertEquals(true, Pattern.compile('(?x)\\0 1 4 2').matcher('b').find());
System.assertEquals(true, Pattern.compile('(?x)\\x2 3').matcher('#').find());
`)
}

func TestRegexNormalizeExtended(t *testing.T) {
	for source, want := range map[string]string{
		"(?x) ^(a+)+b # c\n":    "^(a+)+b",
		"(?ix) a\\ b":           "(?i)a\\ b",
		"(?x)(a+)+(?i:b)":       "(a+)+(?i:b)",
		"(?x:a b) c":            "(?:ab) c",
		"(?x)a (?-x) b #":       "a(?-x) b #",
		"(?x)[a - c]":           "[a-c]",
		"(?x)[a b]{1, 2}":       "[ab]{1,2}",
		"(?x)[a b]\\x2 3":       "[ab]\\x23",
		"(?x)\\x{4 1}":          "\\x{41}",
		"(?x)[a b]\\0 4 3":      "[ab]\\043",
		"(?x)[a b]\\u0 0 2 3":   "[ab]\\u0023",
		"(?x)[a b]a* +a":        "[ab]a*+a",
		"(?x)a#c\u0085b":        "a\\x{85}b",
		"(?dx)a#c\x00b":         "(?d)a\\x00b",
		"(?x)(? :a)":            "(?:a)",
		"(?x)caf\u00e9 \\p {L}": "caf\u00e9\\p{L}",
	} {
		got, ok := javaRegexExtendedNormalize(source)
		if !ok || got != want {
			t.Errorf("javaRegexExtendedNormalize(%q) = %q, %v; want %q", source, got, ok, want)
		}
	}
	for _, source := range []string{"(?x)(? =a)a", "(?x)(?-x )a", "(?x)a{ 1,2}", "a b"} {
		if got, ok := javaRegexExtendedNormalize(source); ok {
			t.Errorf("javaRegexExtendedNormalize(%q) = %q, want no normalization", source, got)
		}
	}
}

// Probes that regexp2's own x mode got wrong (X1).
func TestRegexExtendedClassAndEscapeProbes(t *testing.T) {
	runRegexApex(t, `
Matcher a = Pattern.compile('(?x)[a b]{1, 2}').matcher('ab');
System.assertEquals(true, a.find());
System.assertEquals(2, a.end());
System.assertEquals(true, Pattern.compile('(?x)[a b] x{2 ,3}').matcher('axxx').find());
System.assertEquals(true, Pattern.compile('(?x)[a b]a{1 0}').matcher('b' + 'a'.repeat(10)).find());
Matcher h = Pattern.compile('(?x)[a b]\\x2 3').matcher('a#aa');
System.assertEquals(true, h.find());
System.assertEquals('a#', h.group());
System.assertEquals(true, Pattern.compile('(?x)[a b]\\0 4 3').matcher('a#aa').find());
System.assertEquals(true, Pattern.compile('(?x)[a b]\\u0 0 2 3').matcher('a#aa').find());
Matcher p = Pattern.compile('(?x)[a b]a* +a').matcher('baaa');
System.assertEquals(false, p.find());
Matcher x = Pattern.compile('(?x)\\x{4 1}').matcher('A');
System.assertEquals(true, x.find());
System.assertEquals(true, Pattern.matches('(?x)\\x{4 1}', 'A'));
Matcher range = Pattern.compile('(?x)[a - c]').matcher('b-');
System.assertEquals(true, range.find());
System.assertEquals('b', range.group());
`)
}

// Under (?x) Java skips whitespace and comments on ordinary reads, but reads
// the character after (? , after a backslash, after { and at the end of a
// class range raw (native rows d17_*).
func TestRegexExtendedRawReads(t *testing.T) {
	for _, source := range []string{"(?x)(? =a)a", "(?x)(? !b)", "(?x)(? <=a)", "(?x)(? >a)", "(?x)(? <n>a)", "(?x)a{ 1,2}"} {
		if e := javaRegexSyntaxCheck(source); e == nil {
			t.Errorf("%q should be rejected", source)
		} else if source != "(?x)a{ 1,2}" && e.description != "Unknown inline modifier" {
			t.Errorf("%q: %s", source, e.description)
		}
	}
	for _, source := range []string{"(?x)(? :a)", "(?x)(? i)a", "(?x)( ?<n>a)\\k<n>", "(?x)\\x2 3", "(?x)\\0 1 4 2", "(?x)\\u00 41", "(?x)[a#c\rb]", "(?x)a #c\r|b", "(?xd)a #c\r|b", "(?x)a{1 ,2}"} {
		if e := javaRegexSyntaxCheck(source); e != nil {
			t.Errorf("%q rejected: %s", source, e.description)
		}
	}
	runRegexApex(t, `
for (String p : new List<String>{'(?x)(? =a)a', '(?x)(? !b)', '(?x)(? <=a)', '(?x)(? >a)', '(?x)(? <n>a)'}) {
    try { Pattern.compile(p); System.assert(false, 'compiled ' + p); }
    catch (StringException e) { System.assert(e.getMessage().contains('Unknown inline modifier'), e.getMessage()); }
}
Matcher g = Pattern.compile('(?x)(? :a)').matcher('a');
System.assertEquals(true, g.find());
System.assertEquals(0, g.groupCount());
Matcher n = Pattern.compile('(?x)( ?<n>a)\\k<n>').matcher('aa');
System.assertEquals(true, n.find());
System.assertEquals(1, n.groupCount());
System.assertEquals(true, Pattern.compile('(?x)[a#c\r' + 'b]').matcher('b').find());
`)
}

// Java COMMENTS-mode rows C11, C13, C22, C23 and C32 (native d17x_*).
func TestRegexExtendedCountedRepeatsAndTerminatorChars(t *testing.T) {
	runRegexApex(t, `
Matcher a = Pattern.compile('(?x)a{2, 3}').matcher('aaaa');
System.assertEquals(true, a.find());
System.assertEquals(3, a.end());
System.assertEquals(true, Pattern.matches('(?x)a{1 2}', 'a'.repeat(12)));
System.assertEquals(false, Pattern.compile('(?x)a#c\u0085b').matcher('ab').find());
System.assertEquals(true, Pattern.compile('(?x)a#c\u0085b').matcher('a\u0085b').find());
String z = String.fromCharArray(new List<Integer>{0});
System.assertEquals(true, Pattern.matches('(?dx)a#c' + z + 'b', 'a' + z + 'b'));
try { Pattern.compile('(?x)(?-x )a'); System.assert(false, 'compiled'); }
catch (StringException e) { System.assert(e.getMessage().contains('Unknown inline modifier'), e.getMessage()); }
`)
}

// A literal run followed by \p used to leave the parser's cursor behind the
// backslash and loop forever.
func TestRegexSyntaxCheckLiteralRunBeforeProperty(t *testing.T) {
	for _, source := range []string{`a\p{L}`, "\u00e9\\p{L}", `a\pL`, `x\p{Lu}b`, `ab\P{L}cd`} {
		done := make(chan *javaSyntaxError, 1)
		go func() { done <- javaRegexSyntaxCheck(source) }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("%q: %s", source, err.description)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%q: parser loops", source)
		}
	}
	runRegexApex(t, `
System.assertEquals(true, Pattern.compile('a\\p{L}').matcher('ab').find());
System.assertEquals('b', 'ab'.replaceAll('a\\p{L}', 'b'));
`)
}

// \N{name} and \p{name} bodies are read raw under (?x) (R1), and an unknown
// character name is Java's syntax error at the closing brace (R2).
func TestRegexExtendedNamedCharacters(t *testing.T) {
	for source, want := range map[string]string{
		`(?x)\N{LATIN SMALL LETTER A}`:      `\N{LATIN SMALL LETTER A}`,
		`(?x)[a b]\N{LATIN SMALL LETTER A}`: `[ab]\N{LATIN SMALL LETTER A}`,
		`(?x) \N{LATIN SMALL LETTER A} b`:   `\N{LATIN SMALL LETTER A}b`,
	} {
		if got, ok := javaRegexExtendedNormalize(source); !ok || got != want {
			t.Errorf("javaRegexExtendedNormalize(%q) = %q, %v; want %q", source, got, ok, want)
		}
	}
	err := javaRegexSyntaxCheck(`\N{FOO BAR}`)
	if err == nil || err.description != "Unknown character name [FOO BAR]" || err.index != 10 {
		t.Errorf("\\N{FOO BAR}: %+v", err)
	}
	runRegexApex(t, `
System.assertEquals(true, Pattern.compile('(?x)\\N{LATIN SMALL LETTER A}').matcher('a').find());
System.assertEquals(true, Pattern.compile('(?x)[a b]\\N{LATIN SMALL LETTER A}').matcher('ba').find());
try { Pattern.compile('\\N{FOO BAR}'); System.assert(false, 'compiled'); }
catch (StringException e) { System.assert(e.getMessage().contains('Unknown character name [FOO BAR]'), e.getMessage()); }
`)
}

// Character.codePointOf rules, from the native d20_* and d21_* rows.
func TestJavaCharacterByNameFollowsCodePointOf(t *testing.T) {
	for name, want := range map[string]rune{
		"GREEK SMALL LETTER ALPHA": 0x3B1, "greek small letter alpha": 0x3B1, "  LATIN SMALL LETTER A  ": 'a',
		"LATIN SMALL LETTER A WITH GRAVE": 0xE0, "NO-BREAK SPACE": 0xA0, "GRINNING FACE": 0x1F600,
		"NULL": 0, "DELETE": 0x7F, "LINE FEED (LF)": 10, "NEXT LINE (NEL)": 0x85, "APPLICATION PROGRAM COMMAND": 0x9F,
		"CJK UNIFIED IDEOGRAPHS 4E00": 0x4E00, "cjk unified ideographs 4e00": 0x4E00, "HANGUL SYLLABLES AC00": 0xAC00,
		"CJK UNIFIED IDEOGRAPHS EXTENSION B 20000": 0x20000, "PRIVATE USE AREA E000": 0xE000, "TANGUT 17000": 0x17000,
	} {
		if got, ok := javaCharacterByName(name); !ok || got != want {
			t.Errorf("javaCharacterByName(%q) = %U, %v; want %U", name, got, ok, want)
		}
	}
	for _, name := range []string{"LATIN_SMALL_LETTER_A", "LATIN-SMALL-LETTER-A", "LATIN SMALL LETTER  A", "LINE FEED", "FOO BAR",
		"CJK UNIFIED IDEOGRAPH-4E00", "CJK UNIFIED IDEOGRAPH 4E00", "HANGUL SYLLABLE GA", "BASIC LATIN 41", "BASIC LATIN 0041",
		"CJK UNIFIED IDEOGRAPHS 04E00", "CJK UNIFIED IDEOGRAPHS +4E00", "CJK UNIFIED IDEOGRAPHS A000", "LATIN 1 SUPPLEMENT 80", "EMOTICONS 1F600"} {
		if got, ok := javaCharacterByName(name); ok {
			t.Errorf("javaCharacterByName(%q) = %U, want unknown", name, got)
		}
	}
	runRegexApex(t, `
System.assertEquals(true, Pattern.compile('\\N{GREEK SMALL LETTER ALPHA}').matcher('α').find());
System.assertEquals(true, Pattern.compile('\\N{greek small letter alpha}').matcher('α').find());
System.assertEquals(true, Pattern.compile('[\\N{GREEK SMALL LETTER ALPHA}x]').matcher('α').find());
System.assertEquals(true, Pattern.compile('(?x)\\N{GREEK SMALL LETTER ALPHA}').matcher('α').find());
try { Pattern.compile('\\N{LATIN_SMALL_LETTER_A}'); System.assert(false, 'compiled'); }
catch (StringException e) { System.assert(e.getMessage().contains('Unknown character name [LATIN_SMALL_LETTER_A]'), e.getMessage()); }
`)
}

func BenchmarkJavaCharacterNamesFirstUse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		javaNamesOnce = sync.Once{}
		javaNameToRune = nil
		javaCharacterByName("GREEK SMALL LETTER ALPHA")
	}
}
