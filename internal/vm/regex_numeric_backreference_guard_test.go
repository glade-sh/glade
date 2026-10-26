package vm

import (
	"fmt"
	"testing"
)

// Native reference rows cover every single-digit reference. The lexical guard
// must distinguish them from octal and escaped literals before Go parses them.
func TestLiteralSplitNumericReferenceGuardDigitAndEscapedRoutes(t *testing.T) {
	for digit := 1; digit <= 9; digit++ {
		source := fmt.Sprintf(`(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\%d`, digit)
		t.Run(fmt.Sprintf("numeric/%d", digit), func(t *testing.T) {
			if !regexContainsNumericBackreference(source) {
				t.Fatal("numeric reference was not guarded")
			}
		})
	}
	for _, source := range []string{`\010`, `\\10`, `[\10]`, `[1]0`, `[\\]10`} {
		t.Run("literal/"+source, func(t *testing.T) {
			if regexContainsNumericBackreference(source) {
				t.Fatal("literal was classified as a numeric reference")
			}
		})
	}
}
