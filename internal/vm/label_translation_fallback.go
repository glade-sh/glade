package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// labelEmptyTranslationScope identifies a source-loaded local label with a
// known empty translation and returns a local-only registry without its empty
// translations. The marker is absent in older registries whose values could
// already have lost nonbreaking characters, so those rows remain unchanged.
func labelEmptyTranslationScope(registry storage.MetadataRegistry, name string) (storage.MetadataRegistry, string, bool) {
	for _, translation := range registry.Labels {
		if translation.Namespace != "" || translation.Value != "" || !translation.TranslationSourceExact || !labelTranslationSource(translation.File) {
			continue
		}
		candidate := registry
		candidate.Labels = []storage.LabelMetadata{translation}
		if _, status := resource.ResolveLabel(candidate, "", "", name); status != resource.LabelLookupResolved {
			continue
		}
		var master storage.LabelMetadata
		found := false
		for _, label := range registry.Labels {
			if label.Namespace == "" && strings.EqualFold(label.Name, translation.Name) && labelMasterSource(label.File) {
				master = label
				found = true
				break
			}
		}
		if !found {
			continue
		}
		labels := make([]storage.LabelMetadata, 0, len(registry.Labels))
		for _, label := range registry.Labels {
			if label.Namespace != "" {
				continue
			}
			if strings.EqualFold(label.Name, translation.Name) && label.TranslationSourceExact && labelTranslationSource(label.File) && label.Value == "" {
				continue
			}
			labels = append(labels, label)
		}
		registry.Labels = labels
		return registry, master.Value, true
	}
	return registry, "", false
}

func labelTranslationSource(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".translation") || strings.HasSuffix(path, ".translation-meta.xml")
}

func labelMasterSource(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".labels") || strings.HasSuffix(path, ".labels-meta.xml")
}
