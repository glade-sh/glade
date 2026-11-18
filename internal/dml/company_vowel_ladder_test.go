package dml

import (
	"strings"
	"testing"
)

// NATIVE company_name_fit/org.tsv T1_first and T2_last (custom CompanyName
// rule, API 62.0 and 67.0, anonymous and @IsTest, two repeats; all
// observations agree). A parenthesis candidate with a vowel suffix ("(East)")
// finds the dash stored name with a consonant suffix ("- West") on every
// first-word variant of "Plovdi Trade" (13 of 13, confidence 91) and on 11 of
// 12 last-word variants ("Plovdi Trad" is the one that does not; "Plovdi
// Tradex" reports 92). Neither the base length, the vowel or consonant class
// of a changed letter, nor the earlier syllable alignment candidate (C1,
// scores 89 and 90) follows these verdicts: the first-word ladder holds the
// native score at 91 while its letters change. No rule that fits the training
// rows and these panels is known, so Glade reports none of them. These are
// pinned misses; a rule that matches them must also keep the 0 false matches
// of the other company panels.
func TestCompanyVowelLadderPinnedMisses(t *testing.T) {
	panels := companyFitPanels(t, "T1_", "T2_")
	if len(panels) != 2 {
		t.Fatalf("%d panels, want 2", len(panels))
	}
	want := map[string]struct{ native, confidence91, glade int }{
		"T1_first": {13, 13, 0},
		"T2_last":  {11, 10, 0},
	}
	for name, panel := range panels {
		cross, at91, glade := 0, 0, 0
		for pair, score := range panel.native {
			if pair[0] == pair[1] {
				continue
			}
			cross++
			a, b := panel.values[pair[0]], panel.values[pair[1]]
			if !strings.HasSuffix(a, " (East)") || !strings.HasSuffix(b, " - West") {
				t.Errorf("%s: native match %q to %q is not a parenthesis candidate against a dash stored name", name, a, b)
			}
			if score == 91 {
				at91++
			}
			if matched, _ := fuzzyCompany(a, b, false); matched {
				glade++
			}
		}
		w := want[name]
		t.Logf("%s: %d native cross matches, %d at 91, Glade matches %d", name, cross, at91, glade)
		if cross != w.native || at91 != w.confidence91 || glade != w.glade {
			t.Errorf("%s: native %d at 91 %d Glade %d, want %d, %d, %d", name, cross, at91, glade, w.native, w.confidence91, w.glade)
		}
	}
}
