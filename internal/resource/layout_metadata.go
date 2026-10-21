package resource

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type layoutMetadataXML struct {
	XMLName  xml.Name `xml:"Layout"`
	FullName string   `xml:"fullName"`
}

// LoadLayoutMetadata retains declared layout metadata for every Apex entry
// point. A supplied reader binds cached test orgs to their captured metadata.
func LoadLayoutMetadata(org *storage.OrgState, paths []string, readFile func(string) ([]byte, error)) error {
	if org == nil || len(paths) == 0 {
		return nil
	}
	if readFile == nil {
		readFile = os.ReadFile
	}
	layouts := append([]storage.LayoutMetadata(nil), org.Metadata.Layouts...)
	for _, path := range paths {
		data, err := readFile(path)
		if err != nil {
			return err
		}
		var metadata layoutMetadataXML
		if err := xml.Unmarshal(data, &metadata); err != nil {
			return err
		}
		name := strings.TrimSpace(metadata.FullName)
		if name == "" {
			name = filepath.Base(path)
			for _, suffix := range []string{".layout-meta.xml", ".layout"} {
				if strings.HasSuffix(strings.ToLower(name), suffix) {
					name = name[:len(name)-len(suffix)]
					break
				}
			}
			name = strings.TrimSpace(name)
		}
		if name == "" {
			return fmt.Errorf("layout metadata has no full name")
		}
		entry := storage.LayoutMetadata{FullName: name, XML: string(data)}
		replaced := false
		for i := range layouts {
			if layouts[i].FullName == name {
				layouts[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			layouts = append(layouts, entry)
		}
	}
	org.Metadata.Layouts = layouts
	return nil
}
