package surfaceledger

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/compat"
)

func BuildEvidenceSnapshot(paths []string) ([]SurfaceLedgerRow, error) {
	var rows []SurfaceLedgerRow
	for _, path := range paths {
		if skip, err := shouldSkipNonFixtureEvidenceFile(path); err != nil {
			return nil, err
		} else if skip {
			continue
		}
		fixture, err := compat.LoadFile(path)
		if err != nil {
			return nil, err
		}
		for _, evidence := range fixture.Evidence {
			id := evidence.SurfaceID
			if id == "" {
				id = inferSurfaceIDFromSymbol(evidence.Symbol)
			}
			if id == "" {
				continue
			}
			row := RowFromEvidence(SurfaceLedgerRow{
				SurfaceID: id,
				Product:   productFromID(id),
				Area:      AreaRuntime,
				Kind:      KindMethod,
				Evidence:  EvidenceFixture,
				Sources:   []string{"fixture:" + fixture.Name},
				Notes:     evidence.Notes,
			})
			fillFromApexID(&row)
			rows = append(rows, row)
		}
	}
	sortRows(rows)
	return rows, nil
}

func shouldSkipNonFixtureEvidenceFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	_, hasEvidence := raw["evidence"]
	_, hasCommand := raw["command"]
	return !hasEvidence && !hasCommand, nil
}

func inferSurfaceIDFromSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" || strings.HasPrefix(symbol, "apex:") || strings.HasPrefix(symbol, "rest:") || strings.HasPrefix(symbol, "tooling:") {
		return symbol
	}
	parts := strings.Split(symbol, ".")
	if len(parts) == 2 {
		return ApexMemberID("System", parts[0], parts[1], nil)
	}
	if len(parts) >= 3 {
		ns := strings.Join(parts[:len(parts)-2], ".")
		return ApexMemberID(ns, parts[len(parts)-2], parts[len(parts)-1], nil)
	}
	return ApexTypeID("System", symbol)
}

func productFromID(id string) string {
	switch {
	case strings.HasPrefix(id, "apex:"):
		return ProductApex
	case strings.HasPrefix(id, "tooling:"):
		return ProductTooling
	case strings.HasPrefix(id, "rest:"):
		return ProductREST
	case strings.HasPrefix(id, "visualforce:"):
		return ProductVisualforce
	case strings.HasPrefix(id, "lwc:"):
		return ProductLWC
	case strings.HasPrefix(id, "aura:"):
		return ProductAura
	default:
		return ProductUnknown
	}
}
