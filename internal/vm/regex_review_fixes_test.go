package vm

import (
	"strings"
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
