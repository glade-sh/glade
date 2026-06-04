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
			kind := evidenceKindFromSurfaceID(id)
			row := RowFromEvidence(SurfaceLedgerRow{
				SurfaceID: id,
				Product:   productFromID(id),
				Area:      AreaRuntime,
				Kind:      kind,
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

func evidenceKindFromSurfaceID(id string) string {
	if !strings.HasPrefix(id, "apex:") {
		return KindMethod
	}
	rest := strings.TrimPrefix(id, "apex:")
	if strings.Contains(rest, "(") {
		return KindMethod
	}
	if len(strings.Split(rest, ".")) <= 2 {
		return KindType
	}
	return KindProperty
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
	symbol = cleanIdentityPart(symbol)
	if symbol == "" || strings.HasPrefix(symbol, "apex:") || strings.HasPrefix(symbol, "rest:") || strings.HasPrefix(symbol, "tooling:") || strings.HasPrefix(symbol, "data-reference:") {
		return symbol
	}
	if isHumanBehaviorLabel(symbol) {
		return ""
	}
	parts := strings.Split(symbol, ".")
	if len(parts) == 2 {
		if canonicalApexNamespaceName(parts[0]) == "Database" && startsLowerASCII(parts[1]) {
			return ""
		}
		if isKnownApexNamespace(parts[0]) {
			return ApexTypeID(parts[0], parts[1])
		}
		return ApexMemberID("System", parts[0], parts[1], nil)
	}
	if len(parts) >= 3 {
		if isKnownApexNamespace(parts[0]) {
			ns := parts[0]
			typeName := strings.Join(parts[1:len(parts)-1], ".")
			memberName := parts[len(parts)-1]
			if isKnownZeroArgApexMethod(ns, typeName, memberName) {
				return ApexMemberID(ns, typeName, memberName, []string{})
			}
			return ApexMemberID(ns, typeName, memberName, nil)
		}
		ns := strings.Join(parts[:len(parts)-2], ".")
		return ApexMemberID(ns, parts[len(parts)-2], parts[len(parts)-1], nil)
	}
	return ApexTypeID("System", symbol)
}

func isHumanBehaviorLabel(symbol string) bool {
	return strings.ContainsAny(symbol, " \t\r\n/")
}

func startsLowerASCII(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return value[0] >= 'a' && value[0] <= 'z'
}

func isKnownApexNamespace(namespace string) bool {
	switch canonicalApexNamespaceName(namespace) {
	case "ConnectApi", "Database", "Schema", "System":
		return true
	default:
		return false
	}
}

func isKnownZeroArgApexMethod(namespace, typeName, memberName string) bool {
	if canonicalApexNamespaceName(namespace) != "Schema" {
		return false
	}
	if !strings.HasPrefix(cleanIdentityPart(typeName), "Describe") && typeName != "ChildRelationship" && typeName != "RecordTypeInfo" && typeName != "PicklistEntry" {
		return false
	}
	memberName = canonicalApexMemberName(memberName)
	return hasPrefixFold(memberName, "get") || hasPrefixFold(memberName, "is")
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func productFromID(id string) string {
	switch {
	case strings.HasPrefix(id, "apex:"):
		return ProductApex
	case strings.HasPrefix(id, "tooling:"):
		return ProductTooling
	case strings.HasPrefix(id, "data-reference:"):
		return ProductDataRef
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
