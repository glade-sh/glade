package vm

import "testing"

// PENDING-ROWS: only the language-only Polish key is a candidate alias. The
// published full pl_PL key and unknown-key fallback remain controls.
func TestPolishLanguageOnlyWeekStartCandidate(t *testing.T) {
	if got := salesforceLocaleFirstWeekday("pl"); got != salesforceLocaleFirstWeekday("pl_PL") {
		t.Fatalf("language-only candidate=%v full-key=%v", got, salesforceLocaleFirstWeekday("pl_PL"))
	}
	if got := salesforceLocaleFirstWeekday("unknown_key"); got != salesforceLocaleFirstWeekday("en_US") {
		t.Fatal("unknown locale fallback changed")
	}
}
