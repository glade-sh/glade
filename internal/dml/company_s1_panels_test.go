package dml

import (
	"math"
	"testing"
)

func TestCompanyS1PanelCounts(t *testing.T) {
	for name, panel := range companyFitPanels(t, "s1_") {
		hits, exact, extra := 0, 0, 0
		for i := range panel.values {
			for j := range panel.values {
				got, score := fuzzyCompany(panel.values[i], panel.values[j], false)
				native, matched := panel.native[[2]int{i, j}]
				switch {
				case got && matched:
					hits++
					if math.Round(score) == native {
						exact++
					}
				case got:
					extra++
				}
			}
		}
		t.Logf("%s: native %d, matched %d, exact confidence %d, not native %d", name, len(panel.native), hits, exact, extra)
	}
}
