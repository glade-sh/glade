package resource

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// LocalLabelTranslationScope derives captured unnamespaced label behavior from
// exact source translation text. Old registries without source text are left
// unchanged. Generic resource lookups retain their existing Value behavior.
func LocalLabelTranslationScope(registry storage.MetadataRegistry, name string) (storage.MetadataRegistry, string, bool) {
	for _, translation := range registry.Labels {
		if translation.Namespace != "" || translation.TranslationSourceRaw == nil || !labelTranslationFile(translation.File) || !strings.EqualFold(strings.TrimSpace(translation.Name), strings.TrimSpace(name)) {
			continue
		}
		var master storage.LabelMetadata
		found := false
		for _, label := range registry.Labels {
			if label.Namespace == "" && strings.EqualFold(label.Name, translation.Name) && labelMasterFile(label.File) {
				master = label
				found = true
				break
			}
		}
		if !found {
			continue
		}
		affected := false
		for _, label := range registry.Labels {
			if label.Namespace == "" && strings.EqualFold(label.Name, translation.Name) && label.TranslationSourceRaw != nil && labelTranslationFile(label.File) {
				value := capturedTranslationValue(*label.TranslationSourceRaw)
				if value == "" || value != label.Value {
					affected = true
					break
				}
			}
		}
		if !affected {
			return registry, "", false
		}
		labels := make([]storage.LabelMetadata, 0, len(registry.Labels))
		for _, label := range registry.Labels {
			if label.Namespace != "" || !strings.EqualFold(strings.TrimSpace(label.Name), strings.TrimSpace(translation.Name)) {
				continue
			}
			if strings.EqualFold(label.Name, translation.Name) && label.TranslationSourceRaw != nil && labelTranslationFile(label.File) {
				label.Value = capturedTranslationValue(*label.TranslationSourceRaw)
				if label.Value == "" {
					continue
				}
			}
			labels = append(labels, label)
		}
		registry.Labels = labels
		return registry, master.Value, true
	}
	return registry, "", false
}

func labelTranslationFile(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".translation") || strings.HasSuffix(path, ".translation-meta.xml")
}

func labelMasterFile(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".labels") || strings.HasSuffix(path, ".labels-meta.xml")
}

// Captures include every XML-valid Unicode White_Space codepoint. Only the
// observed blank codepoints are treated as blank; nonblank text is not trimmed.
func capturedTranslationValue(value string) string {
	for _, r := range value {
		if !(r == '\t' || r == '\n' || r == '\r' || r == ' ' || r == '\u1680' ||
			(r >= '\u2000' && r <= '\u2006') || (r >= '\u2008' && r <= '\u200a') ||
			r == '\u2028' || r == '\u2029' || r == '\u205f' || r == '\u3000') {
			return value
		}
	}
	return ""
}
