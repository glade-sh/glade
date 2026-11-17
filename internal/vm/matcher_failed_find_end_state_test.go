package vm

import "testing"

// Failed-find end state. Native rows: regex_java_divergence d14, d14x and
// pattern_matcher R140 (org.tsv under internal/apextest/testdata).
func TestExecMatcherFailedFindEndState(t *testing.T) {
	for _, tc := range []struct {
		name, pattern, text, prelude, want string
	}{
		// R140 family: a Begin root (^ or \A) has no Start wrapper, so a failed
		// find reads only what its one attempt read.
		{"caretRoot", "^a", "b", "", "false hitEnd=false requireEnd=false"},
		{"beginRoot", "\\\\Aa", "b", "", "false hitEnd=false requireEnd=false"},
		{"beginRootAfterFlags", "(?i)\\\\Aa", "b", "", "false hitEnd=false requireEnd=false"},
		{"startWrapped", "a", "b", "", "false hitEnd=true requireEnd=false"},
		{"lastMatchStaysWrapped", "\\\\Ga", "b", "", "false hitEnd=true requireEnd=false"},
		// d14: an end read in a failed attempt leaves requireEnd set.
		{"negLookaheadAtEnd", "a(?!)", "a", "", "false hitEnd=true requireEnd=true"},
		{"nonBoundaryAtEnd", "a\\\\B", "a", "", "false hitEnd=true requireEnd=true"},
		{"negLookaheadMid", "a(?!b)x", "ab", "", "false hitEnd=true requireEnd=false"},
		// d14x: the Start guard keeps the end read out of reach.
		{"startGuard", "\\\\bz", "ab", "", "false hitEnd=true requireEnd=false"},
		{"secondFind", "a\\\\B", "aXa", "m.find();", "false hitEnd=true requireEnd=true"},
		{"thirdFind", "a\\\\B", "aXa", "m.find(); m.find();", "false hitEnd=true requireEnd=true"},
		{"emptyEndRefind", "$", "a", "m.find();", "false hitEnd=true requireEnd=true"},
		// d14x: a negative lookahead body that reads the end counts.
		{"negBodyDollar", "a(?!b$)c", "ab", "", "false hitEnd=true requireEnd=true"},
		{"negBodyFirstSuccess", "a(?!b|b$)", "ab", "", "false hitEnd=true requireEnd=false"},
		{"negBodyDollarFirst", "a(?!b$|b)", "ab", "", "false hitEnd=true requireEnd=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "Matcher m = Pattern.compile('" + tc.pattern + "').matcher('" + tc.text + "'); " + tc.prelude +
				" Boolean f = m.find(); String got = f + ' hitEnd=' + m.hitEnd() + ' requireEnd=' + m.requireEnd();\n" +
				"System.assertEquals('" + tc.want + "', got);\n"
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// d10_m_lone_hi, d10_m_lone_lo and d10_m_prop_Cs: Java matches a lone surrogate
// as a code unit of its own.
func TestExecPatternMatchesLoneSurrogateInput(t *testing.T) {
	source := `System.assertEquals(true, Pattern.matches('[\\uD83D]', '\uD83D'));
System.assertEquals(true, Pattern.matches('\\uDE00', '\uDE00'));
System.assertEquals(true, Pattern.matches('\\p{Cs}', '\uD83D'));
System.assertEquals(true, Pattern.matches('.', '\uD83D'));
System.assertEquals(false, Pattern.matches('\\p{L}', '\uD83D'));
System.assertEquals(true, Pattern.matches('a.*', 'a\uD83Db'));
System.assertEquals(false, Pattern.matches('b', 'a\uD83Db'));
`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Java keeps `last` after a failed search, so the next find() searches again
// from it with the current pattern (d14x_usepattern_*, d14x_caret_third_find).
func TestExecMatcherFindAfterFailureSearchesAgain(t *testing.T) {
	source := `Matcher m = Pattern.compile('a\\B').matcher('a'); m.find();
m.usePattern(Pattern.compile('q'));
System.assertEquals(false, m.find());
System.assertEquals(true, m.hitEnd());
System.assertEquals(false, m.requireEnd());
Matcher n = Pattern.compile('a\\B').matcher('a'); n.find();
n.usePattern(Pattern.compile('a'));
System.assertEquals(true, n.find());
Matcher c = Pattern.compile('^').matcher('ab'); c.find(); c.find();
System.assertEquals(true, c.find());
Matcher j = Pattern.compile('a\\B').matcher('a'); j.find();
String json = JSON.serialize(j);
System.assertEquals(false, json.contains('requireEndText'));
System.assertEquals(true, j.requireEnd());
`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

// d10_m_lit_lone_hi and the Matcher route over a lone surrogate.
func TestExecMatcherMatchesLoneSurrogateInput(t *testing.T) {
	source := `System.assertEquals(true, Pattern.matches('\uD83D', '\uD83D'));
System.assertEquals(true, Pattern.compile('\\uD83D').matcher('\uD83D').matches());
System.assertEquals(true, Pattern.compile('\uD83D').matcher('\uD83D').matches());
Matcher g = Pattern.compile('(.)').matcher('\uD83D');
System.assertEquals(true, g.matches());
System.assertEquals(1, g.end(1));
Matcher k = Pattern.compile('(a)(\uD83D)').matcher('a\uD83D');
System.assertEquals(true, k.matches());
System.assertEquals(2, k.end(2));
System.assertEquals(false, Pattern.compile('b').matcher('a\uD83Db').matches());
System.assertEquals(true, Pattern.compile('a').matcher('a\uD83Db').lookingAt());
`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
