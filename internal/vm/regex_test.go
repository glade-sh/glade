package vm

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestExecPatternMatcherStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('([A-Z]+)([0-9]+)?');
System.assertEquals('([A-Z]+)([0-9]+)?', p.PATTERN());
Matcher m = p.MATCHER('abc DEF42 ghi XYZ');
System.assertEquals(2, m.GROUPCOUNT());
System.assert(m.FIND());
System.assertEquals('DEF42', m.GROUP());
System.assertEquals('DEF42', m.group(0));
System.assertEquals('DEF', m.group(1));
System.assertEquals('42', m.group(2));
System.assertEquals(4, m.start());
System.assertEquals(9, m.end());
System.assertEquals(4, m.start(1));
System.assertEquals(7, m.end(1));
System.assert(m.find());
System.assertEquals('XYZ', m.group(1));
System.assertEquals(null, m.group(2));
System.assertEquals(-1, m.start(2));
System.assertEquals(-1, m.end(2));

Pattern headPattern = Pattern.compile('[A-Z]+');
Matcher head = headPattern.matcher('ABC def');
System.assert(head.lookingAt());
System.assertEquals('ABC', head.group());
System.assertEquals(0, head.start());
System.assertEquals(3, head.end());
Pattern lowerHeadPattern = Pattern.compile('[A-Z]+');
Matcher lowerHead = lowerHeadPattern.matcher('abc DEF');
System.assert(!lowerHead.lookingAt());

Pattern fullPattern = Pattern.compile('[A-Z]+');
Matcher full = fullPattern.matcher('ABC');
System.assert(full.matches());
System.assertEquals('ABC', full.group());
Pattern notFullPattern = Pattern.compile('[A-Z]+');
Matcher notFull = notFullPattern.matcher('ABC def');
System.assert(!notFull.matches());

Pattern aStarB = Pattern.compile('a*b');
Matcher replaceAllMatcher = aStarB.matcher('aabxyzaabxyzabxyzb');
System.assertEquals('-xyz-xyz-xyz-', replaceAllMatcher.replaceAll('-'));
Pattern dog = Pattern.compile('dog');
Matcher replaceFirstMatcher = dog.matcher('zzzdogzzzdogzzz');
System.assertEquals('zzzcatzzzdogzzz', replaceFirstMatcher.replaceFirst('cat'));
Pattern captures = Pattern.compile('([A-Z]+)([0-9]+)');
Matcher captureReplace = captures.matcher('A1 B22');
System.assertEquals('1-A 22-B', captureReplace.replaceAll('$2-$1'));
	System.assert(Pattern.matches('(?i)\\Qhello.world\\E', 'HELLO.WORLD'));
	System.assert(!Pattern.matches('(?i)\\Qhello.world\\E', 'HELLOXWORLD'));
	Id accountId = '001000000000001AAA';
	System.assert(Pattern.matches('001[0-9A-Za-z]+', accountId));
	Matcher subquery = Pattern.compile('(?i)(?s)\\(\\s*SELECT\\s.*?\\)(?=\\s*,|\\s*FROM\\s|\\s*$)').matcher('SELECT Id, (SELECT Id FROM Lines__r WHERE (Status__c = \'Open\')) FROM Account');
	System.assert(subquery.find());
	System.assertEquals('(SELECT Id FROM Lines__r WHERE (Status__c = \'Open\'))', subquery.group());

	Pattern regionWords = Pattern.compile('[A-Z]+');
Matcher regionMatcher = regionWords.matcher('aa ABC bb DEF cc');
System.assertEquals(0, regionMatcher.regionStart());
System.assertEquals(16, regionMatcher.regionEnd());
regionMatcher.region(3, 10);
System.assertEquals(3, regionMatcher.regionStart());
System.assertEquals(10, regionMatcher.regionEnd());
System.assert(regionMatcher.find());
System.assertEquals('ABC', regionMatcher.group());
System.assertEquals(3, regionMatcher.start());
System.assertEquals(6, regionMatcher.end());
System.assert(!regionMatcher.find());
regionMatcher.reset();
System.assertEquals(0, regionMatcher.regionStart());
System.assertEquals(16, regionMatcher.regionEnd());
regionMatcher.region(10, 13);
System.assert(regionMatcher.matches());
System.assertEquals('DEF', regionMatcher.group());
System.assertEquals(10, regionMatcher.start());
System.assertEquals(13, regionMatcher.end());
	Matcher regionReplace = regionWords.matcher('aa ABC bb DEF cc');
	regionReplace.region(3, 10);
	System.assertEquals('aa x bb DEF cc', regionReplace.replaceFirst('x'));
	System.assertEquals('aa x bb DEF cc', regionReplace.replaceAll('x'));
	System.assert(regionReplace.find());
	System.assertEquals('ABC', regionReplace.group());
	System.assertEquals(3, regionReplace.start());
	System.assertEquals(6, regionReplace.end());
	Pattern digits = Pattern.compile('[0-9]+');
regionMatcher.usePattern(digits);
regionMatcher.reset('aa 123 bb ABC');
regionMatcher.region(3, 9);
System.assert(regionMatcher.find());
System.assertEquals('123', regionMatcher.group());

String literal = 'a+b?(x)[1]';
String quoted = Pattern.quote(literal);
System.assert(Pattern.matches(quoted, literal));
System.assert(!Pattern.matches(quoted, 'abx1'));
Matcher literalMatcher = Pattern.compile(quoted).matcher('pre a+b?(x)[1] post');
System.assert(literalMatcher.find());
System.assertEquals(literal, literalMatcher.group());

Pattern comma = Pattern.compile(',');
List<String> splitDefault = comma.split('a,b,,');
System.assertEquals(2, splitDefault.size());
System.assertEquals('a', splitDefault.get(0));
System.assertEquals('b', splitDefault.get(1));
List<String> splitKeepEmpty = comma.split('a,b,,', -1);
System.assertEquals(4, splitKeepEmpty.size());
System.assertEquals('', splitKeepEmpty.get(2));
System.assertEquals('', splitKeepEmpty.get(3));
List<String> splitLimited = comma.split('a,b,c', 2);
System.assertEquals(2, splitLimited.size());
System.assertEquals('a', splitLimited.get(0));
System.assertEquals('b,c', splitLimited.get(1));

Pattern words = Pattern.compile('[A-Z]+');
Matcher resetter = words.matcher('ABC DEF');
System.assert(resetter.find());
System.assertEquals('ABC', resetter.group());
resetter.reset();
System.assert(resetter.find());
System.assertEquals('ABC', resetter.group());
resetter.reset('GHI jkl');
System.assert(resetter.find());
System.assertEquals('GHI', resetter.group());
System.assert(resetter.hasAnchoringBounds());
System.assert(!resetter.hasTransparentBounds());
resetter.useAnchoringBounds(false);
resetter.useTransparentBounds(true);
System.assert(!resetter.hasAnchoringBounds());
System.assert(resetter.hasTransparentBounds());
System.assertEquals('[A-Z]+', resetter.pattern().pattern());
System.assert(!resetter.hitEnd());
System.assert(!resetter.requireEnd());
System.assertEquals('\\$1\\\\x', Matcher.quoteReplacement('$1\\x'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternCompileThrowsPatternSyntaxException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	Pattern.compile('[');
	System.assert(false);
} catch (PatternSyntaxException e) {
	System.assertEquals('[', e.getPattern());
	System.assertEquals(-1, e.getIndex());
	System.assert(e.getDescription().contains('missing closing ]'));
	System.assert(e.getMessage().contains('missing closing ]'));
	System.assertEquals('System.PatternSyntaxException', e.getTypeName());
} catch (Exception e) {
	System.assert(false);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternMatchesThrowsPatternSyntaxException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	Pattern.matches('[', 'x');
	System.assert(false);
} catch (IllegalArgumentException e) {
	System.assertEquals('System.PatternSyntaxException', e.getTypeName());
	System.assert(e.getMessage().contains('missing closing ]'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternQuoteRejectsBadArgumentShape(t *testing.T) {
	program, err := CompileAnonymous(`Pattern.quote(42);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "Pattern.quote expects String") {
		t.Fatalf("expected Pattern.quote argument error, got %v", err)
	}
}

func TestExecPatternMatchesArgumentErrorsAreCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	Pattern.matches(null, 'x');
	System.assert(false, 'expected null regex to throw');
} catch (System.NullPointerException e) {
	System.assert(e.getMessage().contains('Pattern.matches expects String argument'));
}
try {
	Pattern.matches('[0-9]+', null);
	System.assert(false, 'expected null input to throw');
} catch (System.NullPointerException e) {
	System.assert(e.getMessage().contains('Pattern.matches expects String argument'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternRejectsJavaOnlyRegex(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{name: "lookbehind", source: `Pattern.compile('(?<=a)b');`, message: "Java regex lookbehind"},
		{name: "lookahead", source: `Pattern.compile('(?=b)a');`, message: "Java regex lookahead"},
		{name: "namedGroup", source: `Pattern.compile('(?<word>a)');`, message: "Java regex named groups"},
		{name: "atomicGroup", source: `Pattern.compile('(?>a)');`, message: "Java regex atomic groups"},
		{name: "possessiveQuantifier", source: `Pattern.compile('a++');`, message: "Java regex possessive quantifiers"},
		{name: "previousMatchBoundary", source: `Pattern.compile('\Gabc');`, message: "Java regex previous-match boundary"},
		{name: "pythonNamedGroup", source: `Pattern.compile('(?P<word>a)');`, message: "Java regex named groups"},
		{name: "unicodeJavaClass", source: `Pattern.compile('\p{javaLowerCase}+');`, message: "Java regex Unicode character classes"},
		{name: "unicodeIsClass", source: `Pattern.compile('\p{IsAlphabetic}+');`, message: "Java regex Unicode character classes"},
		{name: "linebreakMatcher", source: `Pattern.compile('\R');`, message: "Java regex linebreak matcher"},
		{name: "graphemeMatcher", source: `Pattern.compile('\X');`, message: "Java regex grapheme matcher"},
		{name: "horizontalWhitespaceClass", source: `Pattern.compile('\h+');`, message: "Java regex horizontal/vertical whitespace classes"},
		{name: "verticalWhitespaceClass", source: `Pattern.compile('\V+');`, message: "Java regex horizontal/vertical whitespace classes"},
		{name: "inlineCommentsFlag", source: `Pattern.compile('(?x)a b');`, message: "Java regex inline flags"},
		{name: "inlineUnicodeFlag", source: `Pattern.compile('(?U)\w+');`, message: "Java regex inline flags"},
		{name: "classIntersection", source: `Pattern.compile('[a-z&&[^aeiou]]+');`, message: "Java regex character-class intersections"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected %q error, got %v", tc.message, err)
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
				t.Fatalf("expected UnsupportedFeature runtime error, got %T %v", err, err)
			}
		})
	}
}

func TestExecPatternNumericBackreferenceMatchesCapturedText(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern sameDelimiter = Pattern.compile('([ab])x\\1');
Matcher matcher = sameDelimiter.matcher('axa axb bxb');
System.assert(matcher.find());
System.assertEquals('axa', matcher.group());
System.assert(matcher.find());
System.assertEquals('bxb', matcher.group());
System.assert(!matcher.find());
System.assertEquals('Q axb Q', sameDelimiter.matcher('axa axb bxb').replaceAll('Q'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternMatchesNegativeLookaheadAssertions(t *testing.T) {
	program, err := CompileAnonymous(`
String ssn = '^(?!666|000|9\\d{2})\\d{3}-?(?!00)\\d{2}-?(?!0{4})\\d{4}$';
System.assertEquals(true, Pattern.matches(ssn, '123-45-6789'));
System.assertEquals(false, Pattern.matches(ssn, '666-45-6789'));
System.assertEquals(false, Pattern.matches(ssn, '123-00-6789'));
System.assertEquals(false, Pattern.matches(ssn, '123-45-0000'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringSplitPositiveLookahead(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> parts = 'BoardType'.split('(?=[A-Z])');
System.assertEquals(2, parts.size());
System.assertEquals('Board', parts[0]);
System.assertEquals('Type', parts[1]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringSplitEmptyPatternReturnsCharacters(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> parts = '123'.split('');
System.assertEquals(3, parts.size());
System.assertEquals('1', parts[0]);
System.assertEquals('3', parts[2]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringSplitUsesJavaRegexQuoteEscapes(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> parts = 'alpha.beta.gamma'.split('\\Q.\\E');
System.assertEquals(3, parts.size());
System.assertEquals('alpha', parts[0]);
System.assertEquals('beta', parts[1]);
System.assertEquals('gamma', parts[2]);
List<String> limited = 'alpha.beta.gamma'.split('\\Q.\\E', 2);
System.assertEquals(2, limited.size());
System.assertEquals('alpha', limited[0]);
System.assertEquals('beta.gamma', limited[1]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternFlagsSubset(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(2, Pattern.CASE_INSENSITIVE);
System.assertEquals(8, Pattern.MULTILINE);
System.assertEquals(16, Pattern.LITERAL);
System.assertEquals(32, Pattern.DOTALL);
System.assertEquals(64, Pattern.UNICODE_CASE);

Pattern inline = Pattern.compile('(?i)abc');
System.assert(inline.matcher('ABC').matches());

String dotallInput = String.fromCharArray(new List<Integer>{65,10,66});
Pattern flags = Pattern.compile('a.b', Pattern.CASE_INSENSITIVE + Pattern.DOTALL + Pattern.UNICODE_CASE);
Matcher flagsMatcher = flags.matcher(dotallInput);
System.assert(flagsMatcher.find());
System.assertEquals(dotallInput, flagsMatcher.group());
String multilineInput = String.fromCharArray(new List<Integer>{120,120,10,65,66,67,10,121,121});
Pattern multiline = Pattern.compile('^abc$', Pattern.CASE_INSENSITIVE + Pattern.MULTILINE);
Matcher multilineMatcher = multiline.matcher(multilineInput);
System.assert(multilineMatcher.find());
System.assertEquals('ABC', multilineMatcher.group());

Pattern literal = Pattern.compile('a.b', Pattern.LITERAL);
System.assertEquals('a.b', literal.pattern());
Matcher literalMatcher = literal.matcher('xx a.b axb 9');
System.assert(literalMatcher.find());
System.assertEquals('a.b', literalMatcher.group());
literalMatcher.usePattern(Pattern.compile('[0-9]+'));
System.assert(literalMatcher.find());
System.assertEquals('9', literalMatcher.group());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternCompileRejectsUnsupportedFlags(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "comments", source: `Pattern.compile('a b', Pattern.COMMENTS);`, want: "unsupported regex flags COMMENTS"},
		{name: "unixLines", source: `Pattern.compile('^a$', Pattern.UNIX_LINES);`, want: "unsupported regex flags UNIX_LINES"},
		{name: "canonEq", source: `Pattern.compile('a', Pattern.CANON_EQ);`, want: "unsupported regex flags CANON_EQ"},
		{name: "unicodeCharacterClass", source: `Pattern.compile('\w+', Pattern.UNICODE_CHARACTER_CLASS);`, want: "unsupported regex flags UNICODE_CHARACTER_CLASS"},
		{name: "unknown", source: `Pattern.compile('a', 1024);`, want: "unsupported regex flags unknown flags 0x400"},
		{name: "negative", source: `Pattern.compile('a', -1);`, want: "negative regex flags"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
				t.Fatalf("expected UnsupportedFeature runtime error, got %T %v", err, err)
			}
		})
	}
}

func TestExecMatcherGroupRejectsNoMatch(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('[0-9]+');
Matcher m = p.matcher('abc');
System.assert(!m.find());
m.group();
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "before a successful match") {
		t.Fatalf("expected no-match group error, got %v", err)
	}
}

func TestMatcherStdlibRejectsBadArgumentShapes(t *testing.T) {
	pattern := Object("Pattern")
	pattern.Fields["source"] = String("[A-Z]+")
	if _, _, _, handled, err := callPatternMember(pattern, "pattern", []Value{String("x")}); !handled || err == nil {
		t.Fatalf("Pattern.pattern expected handled error, handled=%v err=%v", handled, err)
	}
	if _, _, _, handled, err := callPatternMember(pattern, "split", []Value{Int(1)}); !handled || err == nil {
		t.Fatalf("Pattern.split expected handled error, handled=%v err=%v", handled, err)
	}

	matcher := Object("Matcher")
	matcher.Fields["source"] = String("([A-Z]+)")
	matcher.Fields["input"] = String("ABC")
	tests := []struct {
		method string
		args   []Value
	}{
		{method: "group", args: []Value{String("0")}},
		{method: "start", args: []Value{Int(-1)}},
		{method: "end", args: []Value{String("0")}},
		{method: "find", args: []Value{String("0")}},
		{method: "replaceAll", args: []Value{Int(1)}},
		{method: "replaceFirst", args: []Value{}},
		{method: "lookingAt", args: []Value{Int(1)}},
		{method: "matches", args: []Value{Int(1)}},
		{method: "reset", args: []Value{Int(1)}},
		{method: "hasAnchoringBounds", args: []Value{Int(1)}},
		{method: "hasTransparentBounds", args: []Value{Int(1)}},
		{method: "useAnchoringBounds", args: []Value{String("false")}},
		{method: "useTransparentBounds", args: []Value{String("true")}},
		{method: "region", args: []Value{Int(0)}},
		{method: "regionStart", args: []Value{Int(0)}},
		{method: "regionEnd", args: []Value{Int(0)}},
		{method: "usePattern", args: []Value{String("[0-9]+")}},
	}
	for _, tc := range tests {
		if _, _, _, handled, err := callMatcherMember(matcher, tc.method, tc.args); !handled || err == nil {
			t.Fatalf("%s expected handled error, handled=%v err=%v", tc.method, handled, err)
		}
	}
}

func TestMatcherRegionRejectsInvalidBounds(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "negative",
			source: `
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC');
m.region(-1, 2);
`,
			want: "bounds must be non-negative",
		},
		{
			name: "reversed",
			source: `
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC');
m.region(2, 1);
`,
			want: "start must be less than or equal to end",
		},
		{
			name: "tooLong",
			source: `
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC');
m.region(0, 4);
`,
			want: "end out of range",
		},
		{
			name: "findStartOutsideRegion",
			source: `
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC DEF');
m.region(4, 7);
m.find(0);
`,
			want: "start out of region",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestMatcherRegionAnchoringAndTransparentBounds(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern head = Pattern.compile('^ABC');
Matcher anchoredHead = head.matcher('xxABCyy');
anchoredHead.region(2, 5);
System.assert(anchoredHead.lookingAt());
System.assert(anchoredHead.matches());
anchoredHead.useAnchoringBounds(false);
System.assert(!anchoredHead.lookingAt());
System.assert(!anchoredHead.matches());

Pattern tail = Pattern.compile('ABC$');
Matcher anchoredTail = tail.matcher('xxABCyy');
anchoredTail.region(2, 5);
System.assert(anchoredTail.lookingAt());
anchoredTail.useAnchoringBounds(false);
System.assert(!anchoredTail.lookingAt());

Pattern word = Pattern.compile('\bABC\b');
Matcher opaque = word.matcher('xABC y');
opaque.region(1, 4);
System.assert(opaque.matches());
opaque.reset();
opaque.region(1, 4);
System.assert(opaque.find());

Matcher transparent = word.matcher('xABC y');
transparent.region(1, 4);
transparent.useTransparentBounds(true);
System.assert(!transparent.matches());
System.assert(!transparent.find());

Matcher transparentAtRealBoundary = word.matcher('x ABC y');
transparentAtRealBoundary.region(2, 5);
transparentAtRealBoundary.useTransparentBounds(true);
System.assert(transparentAtRealBoundary.matches());
System.assertEquals(2, transparentAtRealBoundary.start());
System.assertEquals(5, transparentAtRealBoundary.end());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMatcherFindStartResetsPreviousMatch(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC def');
System.assert(m.find());
System.assertEquals('ABC', m.group());
System.assert(!m.find(4));
m.group();
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "before a successful match") {
		t.Fatalf("expected find(start) to clear stale match, got %v", err)
	}
}

func TestMatcherFailedMatchesClearPreviousMatch(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC def');
System.assert(m.find());
System.assertEquals('ABC', m.group());
System.assert(!m.matches());
m.group();
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "before a successful match") {
		t.Fatalf("expected matches() failure to clear stale match, got %v", err)
	}
}

func TestMatcherFailedLookingAtClearsPreviousMatch(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('[A-Z]+');
Matcher m = p.matcher('ABC def');
System.assert(m.find());
System.assertEquals('ABC', m.group());
m.reset('abc DEF');
System.assert(!m.lookingAt());
m.start();
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "before a successful match") {
		t.Fatalf("expected lookingAt() failure to clear stale match, got %v", err)
	}
}

func TestMatcherGroupIndexErrorsAndOptionalGroups(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('([A-Z]+)([0-9]+)?');
Matcher m = p.matcher('ABC');
System.assert(m.matches());
System.assertEquals(null, m.group(2));
System.assertEquals(-1, m.start(2));
System.assertEquals(-1, m.end(2));
m.group(3);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "Matcher groupIndex out of range") {
		t.Fatalf("expected invalid group index error, got %v", err)
	}
}

func TestRegexSplitRejectsNullablePatterns(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "patternNullableDelimiter",
			source: `Pattern p = Pattern.compile('a*'); p.split('ab cd', -1);`,
			want:   `Pattern.split regexes that can match empty strings`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
				t.Fatalf("expected UnsupportedFeature runtime error, got %T %v", err, err)
			}
		})
	}
}

func TestMatcherAppendReplacementUnsupported(t *testing.T) {
	for _, method := range []string{"appendReplacement", "appendTail"} {
		matcher := Object("Matcher")
		matcher.Fields["source"] = String("[A-Z]+")
		matcher.Fields["input"] = String("ABC")
		_, _, _, handled, err := callMatcherMember(matcher, method, []Value{String(""), String("x")})
		if !handled || err == nil || !strings.Contains(err.Error(), "StringBuffer append semantics") {
			t.Fatalf("expected %s unsupported error, handled=%v err=%v", method, handled, err)
		}
		var runtimeErr *RuntimeError
		if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
			t.Fatalf("expected UnsupportedFeature runtime error, got %T %v", err, err)
		}
	}
}

func TestJavaReplacementEscapesDollar(t *testing.T) {
	converted, err := javaReplacementToGoTemplate("Matcher.replaceAll", `\$1`, 1)
	if err != nil {
		t.Fatal(err)
	}
	if converted != `$$1` {
		t.Fatalf("expected Go literal-dollar template, got %q", converted)
	}
	re := regexp.MustCompile(`([A-Z]+)([0-9]+)`)
	got, err := matcherReplace("Matcher.replaceAll", re, "A1 B22", matcherRegionBounds{endRune: 6, endByte: 6}, []Value{String(`\$1`)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "$1 $1" {
		t.Fatalf("expected escaped dollar replacement, got %q", got)
	}
}

func TestJavaReplacementGroupReferenceParsing(t *testing.T) {
	re := regexp.MustCompile(`([A-Z]+)([0-9]+)`)
	got, err := matcherReplace("Matcher.replaceAll", re, "A1 B22", matcherRegionBounds{endRune: 6, endByte: 6}, []Value{String(`$10`)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "A0 B0" {
		t.Fatalf("expected Java $10 fallback to group 1 plus literal 0, got %q", got)
	}

	got, err = matcherReplace("Matcher.replaceAll", re, "A1 B22", matcherRegionBounds{endRune: 6, endByte: 6}, []Value{String(`$2-$1`)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1-A 22-B" {
		t.Fatalf("expected capture replacement, got %q", got)
	}
}

func TestJavaReplacementRejectsUnsupportedReferences(t *testing.T) {
	re := regexp.MustCompile(`([A-Z]+)`)
	tests := []struct {
		name        string
		replacement string
		want        string
		unsupported bool
	}{
		{name: "missingDollarTarget", replacement: `$`, want: "missing group reference"},
		{name: "badDollarTarget", replacement: `$x`, want: "invalid group reference"},
		{name: "groupOutOfRange", replacement: `$2`, want: "groupIndex out of range"},
		{name: "trailingEscape", replacement: `abc\`, want: "trailing escape"},
		{name: "namedGroup", replacement: `${word}`, want: "named group references", unsupported: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := matcherReplace("Matcher.replaceAll", re, "ABC", matcherRegionBounds{endRune: 3, endByte: 3}, []Value{String(tc.replacement)}, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			if tc.unsupported {
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
					t.Fatalf("expected UnsupportedFeature runtime error, got %T %v", err, err)
				}
			}
		})
	}
}
