package apextest

// The seed's observed before-vector anchors record identity independently of
// the order being tested, so an ordering regression cannot relabel the rows.
func normalizeU7Capture(value map[string]any) map[string]any {
	wrapped := map[string]any{"0-before": value["before"], "1-body": value}
	normalized := normalizeCaptureIdentities(wrapped).(map[string]any)
	return normalized["1-body"].(map[string]any)
}
