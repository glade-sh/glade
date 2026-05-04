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
System.assertEquals('([A-Z]+)([0-9]+)?', p.pattern());
Matcher m = p.matcher('abc DEF42 ghi XYZ');
System.assertEquals(2, m.groupCount());
System.assert(m.find());
System.assertEquals('DEF42', m.group());
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
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternCompileRejectsBadRegex(t *testing.T) {
	program, err := CompileAnonymous(`Pattern.compile('[');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "Pattern.compile invalid regex") {
		t.Fatalf("expected Pattern.compile invalid regex error, got %v", err)
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

func TestExecPatternRejectsJavaOnlyRegex(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{name: "lookbehind", source: `Pattern.compile('(?<=a)b');`, message: "Java regex lookbehind"},
		{name: "lookahead", source: `Pattern.matches('a(?=b)', 'ab');`, message: "Java regex lookahead"},
		{name: "backreference", source: `Pattern.compile('(a)\1');`, message: "Java regex backreferences"},
		{name: "namedGroup", source: `Pattern.compile('(?<word>a)');`, message: "Java regex named groups"},
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
	converted := javaReplacementToGoTemplate(`\$1`)
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
