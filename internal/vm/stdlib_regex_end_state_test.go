package vm

import (
	"testing"
	"unicode/utf8"
)

// Engine preservation checks, not additional Salesforce observations or Apex facets.
func TestRegexp2EndAssertionInstrumentationPreservesPublicGroups(t *testing.T) {
	for _, tc := range []struct {
		name        string
		pattern     string
		input       string
		requiresEnd bool
		groupCount  int
		groupOne    string
		rejectInput string
		matchText   string
	}{
		{"endAssertion", `a$`, "a", true, 0, "", "", ""},
		{"ordinary", `a`, "a", false, 0, "", "", ""},
		{"escapedDollar", `a\$`, "a$", false, 0, "", "", ""},
		{"classDollar", `a[$]`, "a$", false, 0, "", "", ""},
		{"negatedCaretClass", `[^^]$`, "a", true, 0, "", "", ""},
		{"controlEscapeClass", `[\c]$]`, "\x1d", false, 0, "", "(", ""},
		{"controlEscapeFollowedByAnchor", `\c]$`, "\x1d", true, 0, "", "(", ""},
		{"quotedDollar", `\Qa$\E`, "a$", false, 0, "", "", ""},
		{"parentheticalComment", `(?#$)a`, "a", false, 0, "", "", ""},
		{"extendedComment", "(?x)a # $ comment\n", "a", false, 0, "", "", ""},
		{"scopedExtendedComment", "(?x:a # $ comment\n)$", "a", true, 0, "", "", ""},
		{"scopedLiteralHash", `(?x)(?-x:a#)$`, "a#", true, 0, "", "", ""},
		{"abandonedEndAlternative", `a$X|a`, "a", false, 0, "", "", ""},
		{"interiorMultilineEnd", `(?m)a$`, "a\nb", false, 0, "", "", ""},
		{"eofMultilineEnd", `(?m)a$`, "a", true, 0, "", "", ""},
		{"namedCapture", `(?<head>a)$`, "a", true, 1, "a", "", ""},
		{"numericBackreference", `(a)\1$`, "aa", true, 1, "a", "", ""},
		{"namedBackreference", `(?<head>a)\k<head>$`, "aa", true, 1, "a", "", ""},
		{"octalMustNotBecomeBackreference", `(a)(b)(c)(d)(e)(f)(g)(h)(i)\10$`, "abcdefghi\b", true, 9, "a", "", ""},
		{"graphemeCaptureIsolation", `(\X)$`, "a\u0301", true, 1, "a\u0301", "", ""},
		{"defaultFinalLF", `(a)$`, "a\n", true, 1, "a", "", "a"},
		{"defaultFinalCRLF", `(a)$`, "a\r\n", true, 1, "a", "", "a"},
		{"multilineFinalLF", `(?m)a$`, "a\n", false, 0, "", "", "a"},
		{"scopedMultilineFinalLF", `(?m:a$)`, "a\n", false, 0, "", "", "a"},
		{"globalMultilineDisabled", `(?m)(?-m)a$`, "a\n", true, 0, "", "", "a"},
		{"scopedMultilineDisabled", `(?m)(?-m:a$)`, "a\n", true, 0, "", "", "a"},
		{"multilineScopeRestored", `(?m:a)$`, "a\n", true, 0, "", "", "a"},
		{"defaultScopeRestored", `(?m)(?-m:a)$`, "a\n", false, 0, "", "", "a"},
		{"atomicCRLFNoBetween", `a\r$`, "a\r", true, 0, "", "a\r\n", "a\r"},
		{"abandonedFinalLFAlternative", `a$X|a`, "a\n", false, 0, "", "", "a"},
		{"numericBackreferenceFinalCRLF", `(a)\1$`, "aa\r\n", true, 1, "a", "", "aa"},
		{"uppercaseMultiline", `(?M)a$`, "a\nb", false, 0, "", "", "a"},
		{"plusMultiline", `(?i+m)a$`, "a\nb", false, 0, "", "", "a"},
		{"reenabledMultiline", `(?m-m+m)a$`, "a\nb", false, 0, "", "", "a"},
		{"uppercaseExtendedComment", "(?X)a # $ comment\n", "a", false, 0, "", "", "a"},
		{"defaultFinalCR", `(a)$`, "a\r", true, 1, "a", "", "a"},
		{"defaultFinalNEL", `(a)$`, "a\u0085", true, 1, "a", "", "a"},
		{"defaultFinalLS", `(a)$`, "a\u2028", true, 1, "a", "", "a"},
		{"defaultFinalPS", `(a)$`, "a\u2029", true, 1, "a", "", "a"},
		{"multilineFinalCR", `(?m)a$`, "a\r", false, 0, "", "", "a"},
		{"multilineFinalNEL", `(?m)a$`, "a\u0085", false, 0, "", "", "a"},
		{"multilineFinalLS", `(?m)a$`, "a\u2028", false, 0, "", "", "a"},
		{"multilineFinalPS", `(?m)a$`, "a\u2029", false, 0, "", "", "a"},
		{"scopedMultilineDisabledLS", `(?m)(?-m:a$)`, "a\u2028", true, 0, "", "", "a"},
		{"multilineScopeRestoredPS", `(?m:a)$`, "a\u2029", true, 0, "", "", "a"},
		{"defaultScopeRestoredCR", `(?m)(?-m:a)$`, "a\r", false, 0, "", "", "a"},
		{"uppercaseZAtEOF", `a\Z`, "a", true, 0, "", "", "a"},
		{"uppercaseZMultilineNamedBackreferenceRuneOffset", `(?m)(?<head>é)\k<head>\Z`, "éé\u2028", true, 1, "é", "éé\u2028b", "éé"},
		{"octalBeforeUppercaseZ", `(a)(b)(c)(d)(e)(f)(g)(h)(i)\10\Z`, "abcdefghi\b", true, 9, "a", "", "abcdefghi\b"},
		{"escapedUppercaseZ", `a\\Z`, "a\\Z", false, 0, "", "", "a\\Z"},
		{"quotedUppercaseZ", `\Qa\Z\E`, "a\\Z", false, 0, "", "", "a\\Z"},
		{"classUppercaseZ", `[\\Z]`, "Z", false, 0, "", "", "Z"},
		{"parentheticalCommentUppercaseZ", `(?#\Z)a`, "a", false, 0, "", "", "a"},
		{"extendedCommentUppercaseZ", "(?x)a # \\Z comment\n", "a", false, 0, "", "", "a"},
		{"controlEscapeUppercaseZ", `\cZ`, "\x1a", false, 0, "", "", "\x1a"},
		{"abandonedUppercaseZAlternative", `a\ZX|a`, "a", false, 0, "", "", "a"},
		{"multilineAtomicCRLFBeforeCR", `(?m)a$`, "a\r\nb", false, 0, "", "", "a"},
		{"multilineAtomicCRLFNoBetween", `(?m)a\r$`, "a\r", true, 0, "", "a\r\n", "a\r"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("Pattern.compile", tc.pattern, tc.input)
			if err != nil {
				t.Fatal(err)
			}
			match, err := plan.findValidStartingAt(tc.input, 0)
			if err != nil {
				t.Fatal(err)
			}
			if match == nil {
				t.Fatal("instrumentation changed the expected match")
			}
			if got := plan.matchRequiresEnd(tc.input, match); got != tc.requiresEnd {
				t.Fatalf("requireEnd = %v, want %v", got, tc.requiresEnd)
			}
			if got := plan.publicGroupCount(); got != tc.groupCount {
				t.Fatalf("public group count = %d, want %d", got, tc.groupCount)
			}
			if tc.matchText != "" && (match.Index != 0 || match.String() != tc.matchText || match.Length != utf8.RuneCountInString(tc.matchText)) {
				t.Fatal("end assertion changed the zero-width match bounds")
			}
			if tc.rejectInput != "" {
				other, err := compileRegexp2PlanForInput("Pattern.compile", tc.pattern, tc.rejectInput)
				if err != nil {
					t.Fatal(err)
				}
				unexpected, err := other.findValidStartingAt(tc.rejectInput, 0)
				if err != nil || unexpected != nil {
					t.Fatal("instrumentation admitted an unexpected character")
				}
			}
			if tc.groupOne != "" {
				number, ok := plan.publicGroupNumber(1)
				if !ok || match.GroupByNumber(number) == nil || match.GroupByNumber(number).String() != tc.groupOne {
					t.Fatal("instrumentation changed the first public capture")
				}
			}
		})
	}
}
