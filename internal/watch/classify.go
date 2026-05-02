package watch

import (
	"path/filepath"
	"strings"
)

type FileKind string

const (
	FileKindIgnored     FileKind = "ignored"
	FileKindApexClass   FileKind = "apex_class"
	FileKindApexTrigger FileKind = "apex_trigger"
	FileKindObjectMeta  FileKind = "object_metadata"
	FileKindFieldMeta   FileKind = "field_metadata"
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
	base := filepath.Base(clean)
	lowerBase := strings.ToLower(base)

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
	default:
		return FileClassification{
			Path: clean,
			Kind: FileKindIgnored,
		}
	}
}

func trimSuffixFold(s, suffix string) string {
	if len(s) < len(suffix) {
		return s
	}
	return s[:len(s)-len(suffix)]
}
