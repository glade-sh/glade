package surfaceledger

func AssignPriorities(rows []SurfaceLedgerRow) {
	for i := range rows {
		rows[i].Priority = Priority(rows[i])
		if rows[i].Owner == "" {
			rows[i].Owner = OwnerFor(rows[i])
		}
	}
}

func Priority(row SurfaceLedgerRow) int {
	switch row.GapClass {
	case GapMissingShape:
		if row.Area == AreaRuntime {
			return 1
		}
		return 6
	case GapSignatureChanged:
		if row.Area == AreaRuntime {
			return 2
		}
		return 6
	case GapMissingBehavior:
		if row.Area == AreaRuntime {
			return 3
		}
		return 7
	case GapPassiveServiceRisk:
		return 4
	case GapMissingEvidence:
		return 5
	default:
		if row.Product == ProductREST || row.Product == ProductTooling {
			return 7
		}
		if row.Area == AreaUI {
			return 8
		}
		return 9
	}
}

func OwnerFor(row SurfaceLedgerRow) string {
	if row.Product == ProductREST || row.Product == ProductTooling || row.Area == AreaServer {
		return "server"
	}
	if row.Area == AreaUI {
		return "ui"
	}
	if containsASCIIFold(row.SurfaceID, "schema") || containsASCIIFold(row.SurfaceID, "database") {
		return "data-runtime"
	}
	return "runtime"
}
