package watch

import (
	"path/filepath"
	"strings"
)

type FileKind string

const (
	FileKindIgnored               FileKind = "ignored"
	FileKindApexClass             FileKind = "apex_class"
	FileKindApexTrigger           FileKind = "apex_trigger"
	FileKindObjectMeta            FileKind = "object_metadata"
	FileKindFieldMeta             FileKind = "field_metadata"
	FileKindLightningWebComponent FileKind = "lwc_bundle"
	FileKindAuraBundle            FileKind = "aura_bundle"
	FileKindVisualforcePage       FileKind = "visualforce_page"
	FileKindVisualforceComponent  FileKind = "visualforce_component"
	FileKindStaticResource        FileKind = "static_resource"
)

type FileClassification struct {
	Path       string   `json:"path"`
	Kind       FileKind `json:"kind"`
	Name       string   `json:"name,omitempty"`
	ObjectName string   `json:"objectName,omitempty"`
	Watchable  bool     `json:"watchable"`
}

func ClassifyPath(path string) FileClassification {
	clean := filepath.Clean(path)
	slash := filepath.ToSlash(clean)
	base := filepath.Base(clean)
	lowerBase := strings.ToLower(base)
	lowerSlash := strings.ToLower(slash)

	switch {
	case strings.HasSuffix(lowerBase, ".cls"):
		return FileClassification{
			Path:      clean,
			Kind:      FileKindApexClass,
			Name:      trimSuffixFold(base, ".cls"),
			Watchable: true,
		}
	case strings.HasSuffix(lowerBase, ".trigger"):
		return FileClassification{
			Path:      clean,
			Kind:      FileKindApexTrigger,
			Name:      trimSuffixFold(base, ".trigger"),
			Watchable: true,
		}
	case strings.HasSuffix(lowerBase, ".object-meta.xml"):
		return FileClassification{
			Path:       clean,
			Kind:       FileKindObjectMeta,
			Name:       trimSuffixFold(base, ".object-meta.xml"),
			ObjectName: trimSuffixFold(base, ".object-meta.xml"),
			Watchable:  true,
		}
	case strings.HasSuffix(lowerBase, ".field-meta.xml"):
		objectName := ""
		parent := filepath.Base(filepath.Dir(clean))
		if parent == "fields" {
			objectName = filepath.Base(filepath.Dir(filepath.Dir(clean)))
		}
		return FileClassification{
			Path:       clean,
			Kind:       FileKindFieldMeta,
			Name:       trimSuffixFold(base, ".field-meta.xml"),
			ObjectName: objectName,
			Watchable:  true,
		}
	}

	if bundleName, ok := pathSegmentAfter(slash, lowerSlash, "lwc"); ok {
		return FileClassification{
			Path:      clean,
			Kind:      FileKindLightningWebComponent,
			Name:      bundleName,
			Watchable: true,
		}
	}
	if bundleName, ok := pathSegmentAfter(slash, lowerSlash, "aura"); ok {
		return FileClassification{
			Path:      clean,
			Kind:      FileKindAuraBundle,
			Name:      bundleName,
			Watchable: true,
		}
	}

	switch {
	case strings.HasSuffix(lowerBase, ".page"):
		return FileClassification{
			Path:      clean,
			Kind:      FileKindVisualforcePage,
			Name:      trimSuffixFold(base, ".page"),
			Watchable: true,
		}
	case strings.HasSuffix(lowerBase, ".component"):
		return FileClassification{
			Path:      clean,
			Kind:      FileKindVisualforceComponent,
			Name:      trimSuffixFold(base, ".component"),
			Watchable: true,
		}
	case strings.HasSuffix(lowerBase, ".app"):
		return FileClassification{
			Path:      clean,
			Kind:      FileKindAuraBundle,
			Name:      trimSuffixFold(base, ".app"),
			Watchable: true,
		}
	}

	if resourceName, ok := staticResourceName(slash, lowerSlash); ok {
		return FileClassification{
			Path:      clean,
			Kind:      FileKindStaticResource,
			Name:      resourceName,
			Watchable: true,
		}
	}

	return FileClassification{
		Path: clean,
		Kind: FileKindIgnored,
	}
}

func pathSegmentAfter(slashPath, lowerSlashPath, segment string) (string, bool) {
	parts := strings.Split(slashPath, "/")
	lowerParts := strings.Split(lowerSlashPath, "/")
	for i := 0; i < len(lowerParts)-1; i++ {
		if lowerParts[i] == segment && parts[i+1] != "" {
			return parts[i+1], true
		}
	}
	return "", false
}

func staticResourceName(slashPath, lowerSlashPath string) (string, bool) {
	name, ok := pathSegmentAfter(slashPath, lowerSlashPath, "staticresources")
	if !ok {
		return "", false
	}
	name = trimSuffixFold(name, ".resource-meta.xml")
	name = trimSuffixFold(name, ".resource")
	return name, name != ""
}

func trimSuffixFold(s, suffix string) string {
	if len(s) < len(suffix) {
		return s
	}
	if !strings.EqualFold(s[len(s)-len(suffix):], suffix) {
		return s
	}
	return s[:len(s)-len(suffix)]
}
