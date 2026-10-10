package resource

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func loadDataWeaveResources(contentPaths, metadataPaths []string, namespace, fallbackAPI string) ([]storage.DataWeaveResourceMetadata, error) {
	byName := make(map[string]*storage.DataWeaveResourceMetadata)
	resourceFor := func(name string) *storage.DataWeaveResourceMetadata {
		key := lookupKey(name)
		if existing := byName[key]; existing != nil {
			return existing
		}
		resource := &storage.DataWeaveResourceMetadata{Name: name, Namespace: strings.TrimSpace(namespace), APIVersion: fallbackAPI}
		byName[key] = resource
		return resource
	}
	for _, path := range contentPaths {
		name := trimKnownSuffix(filepath.Base(path), ".dwl")
		entry := resourceFor(name)
		if entry.ContentPath != "" && filepath.Clean(entry.ContentPath) != filepath.Clean(path) {
			return nil, fmt.Errorf("duplicate DataWeave source %s: %s and %s", name, entry.ContentPath, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load DataWeave source %s: %w", path, err)
		}
		entry.ContentPath = path
		entry.Content = string(data)
	}
	for _, path := range metadataPaths {
		name := trimKnownSuffix(filepath.Base(path), ".dwl-meta.xml")
		entry := resourceFor(name)
		if entry.MetadataPath != "" && filepath.Clean(entry.MetadataPath) != filepath.Clean(path) {
			return nil, fmt.Errorf("duplicate DataWeave metadata %s: %s and %s", name, entry.MetadataPath, path)
		}
		if entry.ContentPath != "" && filepath.Clean(entry.ContentPath+"-meta.xml") != filepath.Clean(path) {
			return nil, fmt.Errorf("DataWeave metadata %s is not the companion of %s", path, entry.ContentPath)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load DataWeave metadata %s: %w", path, err)
		}
		var metadata struct{ XMLName xml.Name }
		if err := xml.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("load DataWeave metadata %s: %w", path, err)
		}
		if metadata.XMLName.Local != "DataWeaveResource" {
			return nil, fmt.Errorf("invalid DataWeave metadata root in %s: %s", path, metadata.XMLName.Local)
		}
		entry.MetadataPath = path
		entry.APIVersion = project.EffectiveSourceAPIVersionFromMetadata(data, fallbackAPI)
	}
	out := make([]storage.DataWeaveResourceMetadata, 0, len(byName))
	for _, entry := range byName {
		out = append(out, *entry)
	}
	return out, nil
}
