package sema

import (
	"reflect"
	"testing"
)

var sourceSyntaxTokenSamples = []string{
	"",
	" \t\n\f\r",
	"a",
	"_a9 b_ 9a a9",
	"Integer x = 1; // tail",
	"do;while(x)",
	"x\vy",       // \v is not RE2 whitespace, so it is a token
	"é ñ   z",    // non-ASCII letters and no-break space are single-character tokens
	"a\xffb\xc3", // invalid UTF-8 bytes are one-byte tokens
	"Kſ",         // Kelvin sign and long s are not ASCII identifier bytes
	"List<Account> accts = [SELECT Id FROM Account WHERE Name = 'x'];",
	"{ ( [ ] ) } ; , . ? : = == != <= >= && || ! ~ ^ % * / + - <<< >>>",
	"'quoted text'  \"double\"",
	"@IsTest\nstatic void m() {}",
}

// TestSourceSyntaxTokensMatchPattern pins the byte scanner to the pattern it
// replaced on hand-written edge cases and on every Apex source in the
// repository (plus GLADE_SEMA_EQUIV_DIRS).
func TestSourceSyntaxTokensMatchPattern(t *testing.T) {
	check := func(t *testing.T, name, code string) {
		t.Helper()
		want := sourceSyntaxTokenPattern.FindAllStringIndex(code, -1)
		got := sourceSyntaxTokens(code)
		if len(want) == 0 && len(got) == 0 {
			return
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("%s: tokens differ\nwant %v\ngot  %v", name, want, got)
		}
	}
	for i, code := range sourceSyntaxTokenSamples {
		check(t, "sample "+string(rune('A'+i)), code)
		check(t, "sample code "+string(rune('A'+i)), sourceSyntaxCodeText(code))
	}
	files := 0
	forEachRepositoryApexSource(t, func(path, source string) {
		files++
		check(t, path, source)
		check(t, path+" (code text)", sourceSyntaxCodeText(source))
	})
	t.Logf("compared %d files", files)
}
