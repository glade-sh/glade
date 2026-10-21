package schema

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// Custom object tabs expose recent-access timestamps independently of search.
func addCustomTabAccessFields(p project.Project, objects map[string]*Object) error {
	for _, path := range p.TabFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tab struct {
			CustomObject bool `xml:"customObject"`
		}
		if err := xml.Unmarshal(data, &tab); err != nil {
			// Tab XML validity is not this pass's concern; a malformed
			// unrelated tab must not block schema or page loading.
			continue
		}
		if !tab.CustomObject {
			continue
		}
		name := trimMetadataSuffix(trimMetadataSuffix(filepath.Base(path), ".tab-meta.xml"), ".tab")
		name = namespaceProjectObjectName(p.Namespace, remapProjectAPIName(p, name))
		if !strings.HasSuffix(strings.ToLower(name), "__c") {
			continue
		}
		for objectName, object := range objects {
			if !strings.EqualFold(objectName, name) || object.CustomSettingsType != "" {
				continue
			}
			for _, fieldName := range []string{"LastViewedDate", "LastReferencedDate"} {
				if fieldIndex(object.Fields, fieldName) < 0 {
					readOnly, nullable := false, true
					object.Fields = append(object.Fields, Field{
						Name: fieldName, Type: "DateTime", Nillable: &nullable, GeneratedRecentAccess: true,
						Createable: &readOnly, Updateable: &readOnly,
					})
				}
			}
		}
	}
	return nil
}
