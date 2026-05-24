package compat

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/glade-sh/glade/internal/capability"
)

type EvidenceReport struct {
	CatalogEntries    int                  `json:"catalogEntries"`
	Fixtures          int                  `json:"fixtures"`
	Evidence          int                  `json:"evidence"`
	Covered           []CoveredEvidence    `json:"covered,omitempty"`
	UnmatchedEvidence []FixtureEvidenceRef `json:"unmatchedEvidence,omitempty"`
	UngatedPromoted   []CatalogEvidenceGap `json:"ungatedPromoted,omitempty"`
	Summary           []EvidenceSummary    `json:"summary,omitempty"`
}

type CoveredEvidence struct {
	Symbol   string                   `json:"symbol"`
	Status   capability.Status        `json:"status"`
	Target   capability.SupportTarget `json:"target"`
	Area     string                   `json:"area"`
	Fixtures []string                 `json:"fixtures"`
}

type FixtureEvidenceRef struct {
	Fixture string `json:"fixture"`
	Symbol  string `json:"symbol"`
	Kind    string `json:"kind,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type CatalogEvidenceGap struct {
	Symbol string                   `json:"symbol"`
	Status capability.Status        `json:"status"`
	Target capability.SupportTarget `json:"target"`
	Area   string                   `json:"area"`
}

type EvidenceSummary struct {
	Area    string                   `json:"area"`
	Target  capability.SupportTarget `json:"target"`
	Status  capability.Status        `json:"status"`
	Entries int                      `json:"entries"`
	Covered int                      `json:"covered"`
	Ungated int                      `json:"ungated,omitempty"`
}

func BuildEvidenceReport(catalog capability.Catalog, fixtures []Fixture) EvidenceReport {
	entriesBySymbol := map[string]capability.CatalogEntry{}
	for _, entry := range catalog.Entries {
		entriesBySymbol[entry.Symbol] = entry
	}
	fixturesBySymbol := map[string]map[string]struct{}{}
	report := EvidenceReport{
		CatalogEntries: len(catalog.Entries),
		Fixtures:       len(fixtures),
	}
	for _, fixture := range fixtures {
		for _, evidence := range fixture.Evidence {
			report.Evidence++
			if _, ok := entriesBySymbol[evidence.Symbol]; !ok {
				report.UnmatchedEvidence = append(report.UnmatchedEvidence, FixtureEvidenceRef{
					Fixture: fixture.Name,
					Symbol:  evidence.Symbol,
					Kind:    evidence.Kind,
					Notes:   evidence.Notes,
				})
				continue
			}
			set := fixturesBySymbol[evidence.Symbol]
			if set == nil {
				set = map[string]struct{}{}
				fixturesBySymbol[evidence.Symbol] = set
			}
			set[fixture.Name] = struct{}{}
		}
	}
	for symbol, fixtures := range fixturesBySymbol {
		entry := entriesBySymbol[symbol]
		covered := CoveredEvidence{
			Symbol: entry.Symbol,
			Status: entry.Status,
			Target: entry.Target,
			Area:   entry.Area,
		}
		for fixture := range fixtures {
			covered.Fixtures = append(covered.Fixtures, fixture)
		}
		sort.Strings(covered.Fixtures)
		report.Covered = append(report.Covered, covered)
	}
	for _, entry := range catalog.Entries {
		if entry.Status != capability.StatusSupported && entry.Status != capability.StatusPartial {
			continue
		}
		if _, ok := fixturesBySymbol[entry.Symbol]; ok {
			continue
		}
		report.UngatedPromoted = append(report.UngatedPromoted, CatalogEvidenceGap{
			Symbol: entry.Symbol,
			Status: entry.Status,
			Target: entry.Target,
			Area:   entry.Area,
		})
	}
	sort.Slice(report.Covered, func(i, j int) bool {
		return report.Covered[i].Symbol < report.Covered[j].Symbol
	})
	sort.Slice(report.UnmatchedEvidence, func(i, j int) bool {
		if report.UnmatchedEvidence[i].Fixture == report.UnmatchedEvidence[j].Fixture {
			return report.UnmatchedEvidence[i].Symbol < report.UnmatchedEvidence[j].Symbol
		}
		return report.UnmatchedEvidence[i].Fixture < report.UnmatchedEvidence[j].Fixture
	})
	sort.Slice(report.UngatedPromoted, func(i, j int) bool {
		return report.UngatedPromoted[i].Symbol < report.UngatedPromoted[j].Symbol
	})
	report.Summary = summarizeEvidence(catalog.Entries, fixturesBySymbol)
	return report
}

func WriteEvidenceJSON(w io.Writer, report EvidenceReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func summarizeEvidence(entries []capability.CatalogEntry, fixturesBySymbol map[string]map[string]struct{}) []EvidenceSummary {
	type key struct {
		area   string
		target capability.SupportTarget
		status capability.Status
	}
	seen := map[key]*EvidenceSummary{}
	for _, entry := range entries {
		k := key{area: entry.Area, target: entry.Target, status: entry.Status}
		summary := seen[k]
		if summary == nil {
			summary = &EvidenceSummary{Area: entry.Area, Target: entry.Target, Status: entry.Status}
			seen[k] = summary
		}
		summary.Entries++
		if _, ok := fixturesBySymbol[entry.Symbol]; ok {
			summary.Covered++
		} else if entry.Status == capability.StatusSupported || entry.Status == capability.StatusPartial {
			summary.Ungated++
		}
	}
	out := make([]EvidenceSummary, 0, len(seen))
	for _, summary := range seen {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Status < out[j].Status
	})
	return out
}
