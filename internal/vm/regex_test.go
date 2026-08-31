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
	regionReplace.region(10, 13);
	System.assertEquals('aa x bb DEF cc', regionReplace.replaceFirst('x'));
	System.assertEquals(0, regionReplace.regionStart());
	System.assertEquals(16, regionReplace.regionEnd());
	regionReplace.region(3, 10);
	System.assertEquals('aa x bb x cc', regionReplace.replaceAll('x'));
	System.assertEquals(0, regionReplace.regionStart());
	System.assertEquals(16, regionReplace.regionEnd());
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

func TestExecMatcherEndStateAfterFailedFind(t *testing.T) {
	program, err := CompileAnonymous(`
Matcher matcher = Pattern.compile('z').matcher('abc');
System.assertEquals(false, matcher.find());
System.assertEquals(true, matcher.hitEnd());
System.assertEquals(false, matcher.requireEnd());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMatcherMutatorsReturnDistinctSharedWrappersAndFailedFindHitsEnd(t *testing.T) {
	pattern, err := patternCompile([]Value{String("a")})
	if err != nil {
		t.Fatal(err)
	}
	matcher, _, _, handled, err := callPatternMember(pattern, "matcher", []Value{String("aba")})
	if err != nil || !handled {
		t.Fatal(err)
	}

	otherPattern, err := patternCompile([]Value{String("b")})
	if err != nil {
		t.Fatal(err)
	}
	mutators := []struct {
		method string
		args   []Value
	}{
		{method: "region", args: []Value{Int(0), Int(2)}},
		{method: "usePattern", args: []Value{otherPattern}},
		{method: "useAnchoringBounds", args: []Value{Bool(false)}},
		{method: "useTransparentBounds", args: []Value{Bool(true)}},
	}
	for _, tc := range mutators {
		returned, updated, mutated, handled, err := callMatcherMember(matcher, tc.method, tc.args)
		if err != nil || !handled || !mutated {
			t.Fatalf("%s: handled=%v mutated=%v err=%v", tc.method, handled, mutated, err)
		}
		if returned.Ref == matcher.Ref {
			t.Fatalf("%s returned receiver Ref %d", tc.method, matcher.Ref)
		}
		if returned.Equal(matcher) {
			t.Fatalf("%s returned wrapper equals receiver", tc.method)
		}
		if updated.Ref != matcher.Ref {
			t.Fatalf("%s updated receiver Ref = %d, want %d", tc.method, updated.Ref, matcher.Ref)
		}
		returned.Fields["shared"] = String(tc.method)
		if got := matcher.Fields["shared"]; got.Kind != ValueString || got.Text != tc.method {
			t.Fatalf("%s returned wrapper does not share matcher fields", tc.method)
		}
	}

	failedPattern, err := patternCompile([]Value{String("z")})
	if err != nil {
		t.Fatal(err)
	}
	failedMatcher, _, _, _, err := callPatternMember(failedPattern, "matcher", []Value{String("abc")})
	if err != nil {
		t.Fatal(err)
	}
	found, updated, mutated, handled, err := callMatcherMember(failedMatcher, "find", nil)
	if err != nil || !handled || !mutated || found.Kind != ValueBool || found.Bool {
		t.Fatalf("find: value=%#v handled=%v mutated=%v err=%v", found, handled, mutated, err)
	}
	if !matcherBoolField(updated, "hitEnd", false) || matcherBoolField(updated, "requireEnd", true) {
		t.Fatalf("failed find end state: hitEnd=%v requireEnd=%v", matcherBoolField(updated, "hitEnd", false), matcherBoolField(updated, "requireEnd", true))
	}
}

func TestExecMatcherEndStateUpdatesAfterMatchOperation(t *testing.T) {
	program, err := CompileAnonymous(`
Matcher lookingAt = Pattern.compile('z').matcher('abc');
System.assertEquals(false, lookingAt.find());
System.assertEquals(false, lookingAt.lookingAt());
System.assertEquals(false, lookingAt.hitEnd());
System.assertEquals(false, lookingAt.requireEnd());

Matcher matches = Pattern.compile('z').matcher('abc');
System.assertEquals(false, matches.find());
System.assertEquals(false, matches.matches());
System.assertEquals(false, matches.hitEnd());
System.assertEquals(false, matches.requireEnd());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternRejectsNonSalesforceSurface(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "compile overload", source: `Pattern.compile('a', 2);`},
		{name: "CASE_INSENSITIVE", source: `Integer flag = Pattern.CASE_INSENSITIVE;`},
		{name: "COMMENTS", source: `Integer flag = Pattern.COMMENTS;`},
		{name: "MULTILINE", source: `Integer flag = Pattern.MULTILINE;`},
		{name: "LITERAL", source: `Integer flag = Pattern.LITERAL;`},
		{name: "DOTALL", source: `Integer flag = Pattern.DOTALL;`},
		{name: "UNICODE_CASE", source: `Integer flag = Pattern.UNICODE_CASE;`},
		{name: "UNIX_LINES", source: `Integer flag = Pattern.UNIX_LINES;`},
		{name: "CANON_EQ", source: `Integer flag = Pattern.CANON_EQ;`},
		{name: "UNICODE_CHARACTER_CLASS", source: `Integer flag = Pattern.UNICODE_CHARACTER_CLASS;`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err == nil {
				t.Fatalf("expected non-Salesforce Pattern API to be rejected")
			}
		})
	}
}

func TestExecPatternCompileThrowsStringException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	Pattern.compile('[');
	System.assert(false);
} catch (StringException e) {
	System.assert(e.getMessage().contains('missing closing ]'));
	System.assertEquals('System.StringException', e.getTypeName());
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

func TestExecPatternMatchesThrowsStringException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	Pattern.matches('[', 'x');
	System.assert(false);
} catch (StringException e) {
	System.assertEquals('System.StringException', e.getTypeName());
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

func TestExecPatternMalformedInputThrowsStringException(t *testing.T) {
	program, err := CompileAnonymous(`
for (Integer i = 0; i < 2; i++) {
	try {
		if (i == 0) {
			Pattern.compile('[');
		} else {
			Pattern.matches('[', 'x');
		}
		System.assert(false);
	} catch (StringException e) {
		System.assertEquals('System.StringException', e.getTypeName());
	} catch (Exception e) {
		System.assert(false);
	}
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
		{name: "pythonNamedGroup", source: `Pattern.compile('(?P<word>a)');`, message: "Java regex named groups"},
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

func TestExecPatternSupportsJavaLookaroundNamedGroupsAndClassIntersection(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(!Pattern.matches('(?>a+)a', 'aa'));
System.assert(Pattern.matches('a+a', 'aa'));
System.assert(!Pattern.matches('a++a', 'aa'));
System.assert(Pattern.matches('a++', 'aaa'));
System.assert(Pattern.matches('[a-z]++', 'abc'));
System.assert(Pattern.matches('\\w++', 'abc_123'));

Pattern lookbehind = Pattern.compile('(?<=foo)bar');
Matcher behind = lookbehind.matcher('xx foobar baz');
System.assert(behind.find());
System.assertEquals('bar', behind.group());
System.assertEquals(6, behind.start());
System.assertEquals(9, behind.end());
System.assert(!behind.find());

Pattern negativeBehind = Pattern.compile('(?<!foo)bar');
Matcher notBehind = negativeBehind.matcher('bar foobar');
System.assert(notBehind.find());
System.assertEquals(0, notBehind.start());
System.assertEquals('bar', notBehind.group());
System.assert(!notBehind.find());

Pattern ahead = Pattern.compile('foo(?=bar)');
Matcher aheadMatcher = ahead.matcher('xx foobar foozap');
System.assert(aheadMatcher.find());
System.assertEquals('foo', aheadMatcher.group());
System.assertEquals(3, aheadMatcher.start());
System.assert(!aheadMatcher.find());

Pattern named = Pattern.compile('(?<word>[A-Z]+)-(?<num>[0-9]+)');
Matcher namedMatcher = named.matcher('abc DEF-42 ghi');
System.assert(namedMatcher.find());
System.assertEquals('DEF-42', namedMatcher.group());
System.assertEquals('DEF', namedMatcher.group(1));
System.assertEquals('42', namedMatcher.group(2));
System.assertEquals(2, namedMatcher.groupCount());
System.assert(Pattern.matches('(?<word>[A-Z]+)-\\k<word>', 'ABC-ABC'));
System.assert(!Pattern.matches('(?<word>[A-Z]+)-\\k<word>', 'ABC-DEF'));

Pattern consonants = Pattern.compile('[a-z&&[^aeiou]]+');
Matcher consonantMatcher = consonants.matcher('aei bcdf ou');
System.assert(consonantMatcher.find());
System.assertEquals('bcdf', consonantMatcher.group());
System.assert(!Pattern.matches('[a-z&&[^aeiou]]+', 'ae'));
System.assert(Pattern.matches('[a-z&&[^aeiou]]+', 'bcdf'));
System.assert(Pattern.matches('[a-z&&[m-p]]+', 'mnop'));
System.assert(!Pattern.matches('[a-z&&[m-p]]+', 'abc'));
System.assert(Pattern.matches('[a-z&&[^aeiou]&&[^xyz]]+', 'bcdf'));
System.assert(!Pattern.matches('[a-z&&[^aeiou]&&[^xyz]]+', 'ae'));
System.assert(!Pattern.matches('[a-z&&[^aeiou]&&[^xyz]]+', 'xyz'));
System.assert(Pattern.matches('[\\p{L}&&[^\\p{Lu}]]+', 'abcé'));
System.assert(!Pattern.matches('[\\p{L}&&[^\\p{Lu}]]+', 'ABC'));
System.assert(Pattern.matches('[\\w&&[^\\d]]+', 'abc_'));
System.assert(!Pattern.matches('[\\w&&[^\\d]]+', '123'));

System.assert(Pattern.matches('(?x)a b', 'ab'));
System.assert(Pattern.matches('(?x)a\\ b', 'a b'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternSupportsJavaUnicodeAliasesAndClassFlag(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{name: "plain word ascii", pattern: `\w+`, input: "abc_123", want: true},
		{name: "plain word excludes omega", pattern: `\w+`, input: "Ω", want: false},
		{name: "unicode word includes omega", pattern: `(?U)\w+`, input: "Ω", want: true},
		{name: "java lower", pattern: `\p{javaLowerCase}+`, input: "abcé", want: true},
		{name: "java lower rejects upper", pattern: `\p{javaLowerCase}+`, input: "ABC", want: false},
		{name: "java upper", pattern: `\p{javaUpperCase}+`, input: "ABCÉ", want: true},
		{name: "java digit", pattern: `\p{javaDigit}+`, input: "123", want: true},
		{name: "java whitespace includes controls", pattern: `\p{javaWhitespace}+`, input: "\t\n\r", want: true},
		{name: "java whitespace complement", pattern: `\P{javaWhitespace}+`, input: "abc", want: true},
		{name: "is alphabetic", pattern: `\p{IsAlphabetic}+`, input: "abcΩ", want: true},
		{name: "is alphabetic rejects digit", pattern: `\p{IsAlphabetic}+`, input: "abc1", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := patternMatches([]Value{String(tc.pattern), String(tc.input)})
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != ValueBool || got.Bool != tc.want {
				compiled, compileErr := compileRegexp2Source("Pattern.matches", tc.pattern)
				t.Fatalf("Pattern.matches(%q, %q) compiled %q err %v = %#v, want %v", tc.pattern, tc.input, compiled, compileErr, got, tc.want)
			}
		})
	}
}

func TestExecMatcherReplacementUsesRegexp2Features(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern behind = Pattern.compile('(?<=foo)(bar)');
System.assertEquals('foobar! baz', behind.matcher('foobar baz').replaceAll('$1!'));

Pattern consonants = Pattern.compile('[a-z&&[^aeiou]]+');
System.assertEquals('x aei', consonants.matcher('bcdf aei').replaceFirst('x'));

Pattern named = Pattern.compile('(?<word>[A-Z]+)-\\k<word>');
System.assertEquals('ok bad', named.matcher('ABC-ABC bad').replaceAll('ok'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMatcherFindSupportsZeroWidthJavaAssertions(t *testing.T) {
	program, err := CompileAnonymous(`
Matcher previous = Pattern.compile('\\G\\w').matcher('ab cd');
System.assert(previous.find());
System.assertEquals('a', previous.group());
System.assert(previous.find());
System.assertEquals('b', previous.group());
System.assert(!previous.find());
Matcher previousFromStart = Pattern.compile('\\G\\w').matcher('ab cd');
System.assert(previousFromStart.find(3));
System.assertEquals('c', previousFromStart.group());

Matcher m = Pattern.compile('(?=a)').matcher('aba');
System.assert(m.find());
System.assertEquals('', m.group());
System.assertEquals(0, m.start());
System.assertEquals(0, m.end());
System.assert(m.find());
System.assertEquals(2, m.start());
System.assertEquals(2, m.end());
System.assert(!m.find());

Matcher wordEnd = Pattern.compile('(?<=\\w)(?=\\W|$)').matcher('ab cd');
System.assert(wordEnd.find());
System.assertEquals(2, wordEnd.start());
System.assert(wordEnd.find());
System.assertEquals(5, wordEnd.start());
System.assert(!wordEnd.find());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternSupportsJavaLinebreakAndWhitespaceClasses(t *testing.T) {
	program, err := CompileAnonymous(`
String linebreaks = String.fromCharArray(new List<Integer>{65,13,10,66,10,67});
Matcher r = Pattern.compile('\\R').matcher(linebreaks);
System.assert(r.find());
System.assertEquals(1, r.start());
System.assertEquals(3, r.end());
System.assert(r.find());
System.assertEquals(4, r.start());
List<String> rows = linebreaks.split('\\R');
System.assertEquals(3, rows.size());
System.assertEquals('A', rows[0]);
System.assertEquals('B', rows[1]);
System.assertEquals('C', rows[2]);

String horizontal = String.fromCharArray(new List<Integer>{9,32,160});
System.assert(Pattern.matches('\\h+', horizontal));
System.assert(Pattern.matches('\\H+', 'abc'));
String vertical = String.fromCharArray(new List<Integer>{10,13,11,12});
System.assert(Pattern.matches('\\v+', vertical));
System.assert(Pattern.matches('\\V+', 'abc'));
System.assert(Pattern.matches('[\\h]+', horizontal));
System.assert(Pattern.matches('[\\v]+', vertical));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternSupportsJavaGraphemeMatcherForCombiningMarks(t *testing.T) {
	program, err := CompileAnonymous(`
String combining = 'e' + String.fromCharArray(new List<Integer>{769});
String mark = String.fromCharArray(new List<Integer>{769});
System.assert(Pattern.matches('\\X', 'e'));
System.assert(Pattern.matches('\\X', combining));
System.assert(Pattern.matches('\\X', mark));
System.assert(!Pattern.matches('\\X', 'ee'));

Matcher m = Pattern.compile('\\X').matcher(combining + 'x');
System.assert(m.find());
System.assertEquals(combining, m.group());
System.assertEquals(0, m.start());
System.assertEquals(2, m.end());
System.assert(m.find());
System.assertEquals('x', m.group());
System.assertEquals(2, m.start());
System.assertEquals(3, m.end());
System.assert(!m.find());

List<String> split = (combining + 'x').split('\\X', -1);
System.assertEquals(3, split.size());
System.assertEquals('', split[0]);
System.assertEquals('', split[1]);
System.assertEquals('', split[2]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternSupportsExtendedGraphemeClustersAndBoundaries(t *testing.T) {
	program, err := CompileAnonymous(`
String gx = String.fromCharArray(new List<Integer>{92}) + 'X';
String bg = String.fromCharArray(new List<Integer>{92}) + 'b{g}';
String crlf = String.fromCharArray(new List<Integer>{13,10});
String jamo = String.fromCharArray(new List<Integer>{4352,4449});
String flagUS = String.fromCharArray(new List<Integer>{55356,56826,55356,56824});
String thumbTone = String.fromCharArray(new List<Integer>{55357,56397,55356,57341});
String family = String.fromCharArray(new List<Integer>{55357,56424,8205,55357,56425,8205,55357,56423,8205,55357,56422});
String mark = String.fromCharArray(new List<Integer>{769});
String combining = 'e' + mark;

System.assert(Pattern.matches(gx, crlf));
System.assert(Pattern.matches(gx, jamo));
System.assert(Pattern.matches(gx, flagUS));
System.assert(Pattern.matches(gx, thumbTone));
System.assert(Pattern.matches(gx, family));
System.assert(Pattern.matches(gx, mark));
System.assert(Pattern.matches(gx, combining));

Matcher thumb = Pattern.compile(gx).matcher(thumbTone + 'x');
System.assert(thumb.find());
System.assertEquals(thumbTone, thumb.group());
System.assertEquals(0, thumb.start());
System.assertEquals(4, thumb.end());
System.assert(thumb.find());
System.assertEquals('x', thumb.group());
System.assertEquals(4, thumb.start());
System.assertEquals(5, thumb.end());

Matcher boundary = Pattern.compile(bg).matcher(combining + 'x');
System.assert(boundary.find());
System.assertEquals(0, boundary.start());
System.assert(boundary.find());
System.assertEquals(2, boundary.start());
System.assert(boundary.find());
System.assertEquals(3, boundary.start());
System.assert(!boundary.find());

List<String> stringParts = (thumbTone + 'x').split(gx, -1);
System.assertEquals(3, stringParts.size());
System.assertEquals('', stringParts[0]);
System.assertEquals('', stringParts[1]);
System.assertEquals('', stringParts[2]);

List<String> patternParts = Pattern.compile(gx).split(thumbTone + 'x', -1);
System.assertEquals(3, patternParts.size());
System.assertEquals('', patternParts[0]);
System.assertEquals('', patternParts[1]);
System.assertEquals('', patternParts[2]);

System.assertEquals('Qx', Pattern.compile(gx).matcher(thumbTone + 'x').replaceFirst('Q'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternSupportsNestedJavaClassIntersectionShapes(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(Pattern.matches('[a-z&&[m-p]&&[n-o]]+', 'no'));
System.assert(!Pattern.matches('[a-z&&[m-p]&&[n-o]]+', 'mp'));
System.assert(Pattern.matches('[a-z&&[m-p]&&[^o]]+', 'mnp'));
System.assert(!Pattern.matches('[a-z&&[m-p]&&[^o]]+', 'o'));
System.assert(Pattern.matches('[a-z&&[m-p&&[^o]]]+', 'mnp'));
System.assert(!Pattern.matches('[a-z&&[m-p&&[^o]]]+', 'o'));
System.assert(Pattern.matches('[a-c[x-z]]+', 'abcxyz'));
System.assert(!Pattern.matches('[a-c[x-z]]+', 'm'));
System.assert(Pattern.matches('[\\p{L}&&[\\p{Ll}]&&[^x]]+', 'abcé'));
System.assert(!Pattern.matches('[\\p{L}&&[\\p{Ll}]&&[^x]]+', 'x'));
List<String> split = 'amnoq'.split('[a-z&&[m-p]&&[^o]]', -1);
System.assertEquals(3, split.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
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

List<String> keepTrailing = '123'.split('', -1);
System.assertEquals(4, keepTrailing.size());
System.assertEquals('', keepTrailing[3]);

List<String> limited = '123'.split('', 2);
System.assertEquals(2, limited.size());
System.assertEquals('1', limited[0]);
System.assertEquals('23', limited[1]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringSplitSupportsZeroWidthWordBoundary(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> boundary = 'ab cd'.split('\\b');
System.assertEquals(3, boundary.size());
System.assertEquals('ab', boundary[0]);
System.assertEquals(' ', boundary[1]);
System.assertEquals('cd', boundary[2]);

List<String> keepTrailing = 'ab cd'.split('\\b', -1);
System.assertEquals(4, keepTrailing.size());
System.assertEquals('', keepTrailing[3]);

List<String> limited = 'ab cd'.split('\\b', 3);
System.assertEquals(3, limited.size());
System.assertEquals('cd', limited[2]);
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

func TestExecPatternSupportsInlineFlags(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern inline = Pattern.compile('(?i)abc');
System.assert(inline.matcher('ABC').matches());

String dotallInput = String.fromCharArray(new List<Integer>{65,10,66});
Pattern dotall = Pattern.compile('(?is)a.b');
Matcher dotallMatcher = dotall.matcher(dotallInput);
System.assert(dotallMatcher.find());
System.assertEquals(dotallInput, dotallMatcher.group());

String multilineInput = String.fromCharArray(new List<Integer>{120,120,10,65,66,67,10,121,121});
Pattern multiline = Pattern.compile('(?im)^abc$');
Matcher multilineMatcher = multiline.matcher(multilineInput);
System.assert(multilineMatcher.find());
System.assertEquals('ABC', multilineMatcher.group());

Pattern unicodeClass = Pattern.compile('(?U)\\w+');
System.assert(unicodeClass.matcher('Ω').matches());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
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

func TestRegexSplitSupportsNullablePatterns(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern p = Pattern.compile('a*');
List<String> keepTrailing = p.split('ab cd', -1);
System.assertEquals(7, keepTrailing.size());
System.assertEquals('', keepTrailing[0]);
System.assertEquals('', keepTrailing[1]);
System.assertEquals('b', keepTrailing[2]);
System.assertEquals('', keepTrailing[6]);

List<String> trimTrailing = p.split('ab cd');
System.assertEquals(6, trimTrailing.size());
System.assertEquals('d', trimTrailing[5]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
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
