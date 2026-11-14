package lsp

import "strings"

func filterCompletionCatalog(catalog []CompletionItem, prefix string) ([]CompletionItem, bool) {
	prefix = strings.ToLower(prefix)
	items := make([]CompletionItem, 0, min(len(catalog), 2000))
	incomplete := prefix != ""
	for _, item := range catalog {
		if prefix != "" && !completionSubsequence(strings.ToLower(item.Label), prefix) {
			continue
		}
		if len(items) == 2000 {
			incomplete = true
			break
		}
		items = append(items, item)
	}
	return items, incomplete
}

func completionSubsequence(label, prefix string) bool {
	remaining := []rune(prefix)
	for _, r := range label {
		if len(remaining) > 0 && r == remaining[0] {
			remaining = remaining[1:]
		}
	}
	return len(remaining) == 0
}
