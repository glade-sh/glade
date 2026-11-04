package apextest

// Anchor the parent before visiting child rows, which can be absent under sharing.
func normalizeQ3HiddenCapture(value map[string]any) map[string]any {
	wrapped := map[string]any{"0-parent": value["expectedParentId"], "1-body": value}
	return normalizeCaptureIdentities(wrapped).(map[string]any)["1-body"].(map[string]any)
}
