package project

import (
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// This projection contains exact object/field entries from the native describe
// FeatureFields in storage/standard_schema_generated.go and the captured gates
// in storage/standard_feature_fields.go. See docs/STANDARD_OBJECT_SCHEMA.md.
// Keeping only these entries avoids a project/storage import cycle. The catalog
// conformance test checks each entry against the runtime's feature profiles.
//
//go:embed org_setup_field_metadata.json
var setupFieldMetadataJSON []byte

var setupFieldMetadata = func() map[string]map[string][]string {
	var fields map[string]map[string][]string
	if err := json.Unmarshal(setupFieldMetadataJSON, &fields); err != nil {
		panic(err)
	}
	return fields
}()

func setupFeatureFieldMetadata(feature, object, field string) bool {
	for name, fields := range setupFieldMetadata[feature] {
		if !strings.EqualFold(name, object) {
			continue
		}
		for _, name := range fields {
			if strings.EqualFold(name, field) {
				return true
			}
		}
	}
	return false
}

// Salesforce exposes a Contact custom field on Account with the __pc suffix.
// Derive those names from the project's actual metadata; a suffix alone does
// not establish that a queried field exists. Keep the set local to each scan.
func setupCustomPersonAccountFields(p Project) map[string]bool {
	fields := map[string]bool{}
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if strings.HasSuffix(name, "__c") {
			fields[strings.TrimSuffix(name, "__c")+"__pc"] = true
		}
	}
	read := func(path string, into any) bool {
		info, err := os.Stat(path)
		if err != nil || info.Size() > suggestMaxFileBytes {
			return false
		}
		content, err := os.ReadFile(path)
		return err == nil && xml.Unmarshal(content, into) == nil
	}
	for _, path := range p.FieldFiles {
		parts := strings.Split(filepath.ToSlash(path), "/")
		contact := false
		for i := 0; i+2 < len(parts); i++ {
			if strings.EqualFold(parts[i], "objects") && strings.EqualFold(parts[i+1], "Contact") && strings.EqualFold(parts[i+2], "fields") {
				contact = true
				break
			}
		}
		if !contact {
			continue
		}
		var field struct {
			XMLName  xml.Name `xml:"CustomField"`
			FullName string   `xml:"fullName"`
		}
		if read(path, &field) {
			add(field.FullName)
		}
	}
	for _, path := range p.ObjectFiles {
		name := metadataBaseName(path, ".object-meta.xml")
		name = metadataBaseName(name, ".object")
		if !strings.EqualFold(name, "Contact") {
			continue
		}
		var object struct {
			XMLName xml.Name `xml:"CustomObject"`
			Fields  []struct {
				FullName string `xml:"fullName"`
			} `xml:"fields"`
		}
		if read(path, &object) {
			for _, field := range object.Fields {
				add(field.FullName)
			}
		}
	}
	return fields
}
