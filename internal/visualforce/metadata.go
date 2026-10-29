package visualforce

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

func validateVisualforceMetadata(path, resource string) error {
	metaPath := path
	if !strings.HasSuffix(strings.ToLower(path), "-meta.xml") {
		metaPath += "-meta.xml"
	}
	data, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var metadata struct {
		XMLName xml.Name
		Fields  []struct {
			XMLName xml.Name
			Text    string `xml:",chardata"`
		} `xml:",any"`
	}
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("load Visualforce metadata %s: %w", metaPath, err)
	}
	seen := make(map[string]bool)
	for _, field := range metadata.Fields {
		name := field.XMLName.Local
		if seen[name] && name != "packageVersions" {
			return fmt.Errorf("Error parsing file: Element %s is duplicated at this location in type %s", name, resource)
		}
		seen[name] = true
		switch name {
		case "apiVersion":
			value := strings.TrimSpace(field.Text)
			// Native meta_{page,component}_version_{empty,nonnumeric}
			// reports the schema type before source API support policy.
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return fmt.Errorf("Error parsing file: '%s' is not valid for the type xsd:double", value)
			}
		case "availableInTouch", "confirmationTokenRequired":
			if resource != "ApexPage" {
				return fmt.Errorf("%s: unsupported %s metadata field %s", metaPath, resource, name)
			}
			switch strings.TrimSpace(field.Text) {
			case "true", "false", "0", "1":
			default:
				return fmt.Errorf("Error parsing file: '%s' is not valid for type xsd:boolean, should be '0', '1', 'true' or 'false'", strings.TrimSpace(field.Text))
			}
		case "fullName", "description", "label", "packageVersions":
			if name == "label" && resource == "ApexPage" && metadata.XMLName.Space == "http://soap.sforce.com/2006/04/metadata" && strings.TrimSpace(field.Text) == "" {
				return fmt.Errorf("Required fields are missing: Label")
			}
		default:
			element := name
			if field.XMLName.Space != "" {
				element = "{" + field.XMLName.Space + "}" + name
			}
			return fmt.Errorf("Error parsing file: Element %s invalid at this location in type %s", element, resource)
		}
	}
	// Salesforce metadata documents require a label (meta_missing_label).
	// Unnamespaced local partial metadata remains usable for API-version input.
	if resource == "ApexPage" && metadata.XMLName.Space == "http://soap.sforce.com/2006/04/metadata" && !seen["label"] {
		return fmt.Errorf("Required field is missing: label")
	}
	return nil
}

func visualforceConfirmationTokenRequired(path string) (bool, error) {
	metaPath := path
	if !strings.HasSuffix(strings.ToLower(metaPath), "-meta.xml") {
		metaPath += "-meta.xml"
	}
	data, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var metadata struct {
		Required string `xml:"confirmationTokenRequired"`
	}
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return false, err
	}
	value := strings.TrimSpace(metadata.Required)
	return value == "true" || value == "1", nil
}

// VisualforceAccessMetadataError retains the resource that owns a page/class
// access diagnostic, even when validation is requested through the VF loader.
type VisualforceAccessMetadataError struct {
	File    string
	Message string
}

func (e *VisualforceAccessMetadataError) Error() string { return e.Message }

// ValidateVisualforceAccessMetadata checks only the pageAccesses/classAccesses
// section. CRUD, FLS, permission assignments and query security stay with security and access modes.
func ValidateVisualforceAccessMetadata(p project.Project) error {
	if len(p.VisualforcePageFiles) == 0 {
		return nil
	}
	pages, classes := map[string]bool{}, map[string]bool{}
	type projectView struct {
		source    *project.Project
		namespace string
	}
	seen := map[projectView]bool{}
	addName := func(names map[string]bool, name, namespace string) {
		names[strings.ToLower(name)] = true
		if namespace != "" {
			// Access metadata uses the Metadata API spelling, not Apex's dot form.
			names[strings.ToLower(namespace+"__"+name)] = true
		}
	}
	var collectNames func(projectView)
	collectNames = func(view projectView) {
		current := view.source
		if current == nil || seen[view] {
			return
		}
		seen[view] = true
		for _, path := range current.VisualforcePageFiles {
			addName(pages, nameFromPath(path, ".page"), view.namespace)
		}
		for _, path := range current.ApexFiles {
			if strings.EqualFold(filepath.Ext(path), ".cls") {
				addName(classes, nameFromPath(path, ".cls"), view.namespace)
			}
		}
		for _, dep := range current.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				collectNames(projectView{source: dep.Project, namespace: dep.Namespace})
			}
		}
	}
	collectNames(projectView{source: &p})
	for _, path := range p.PermissionSetFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var metadata struct {
			Entries []struct {
				XMLName xml.Name
				Page    string  `xml:"apexPage"`
				Class   string  `xml:"apexClass"`
				Enabled *string `xml:"enabled"`
			} `xml:",any"`
		}
		if err := xml.Unmarshal(data, &metadata); err != nil {
			return err
		}
		for _, entry := range metadata.Entries {
			if entry.XMLName.Local != "pageAccesses" && entry.XMLName.Local != "classAccesses" {
				continue
			}
			if entry.Enabled != nil {
				value := strings.TrimSpace(*entry.Enabled)
				switch value {
				case "true", "false", "0", "1":
				default:
					return &VisualforceAccessMetadataError{File: path, Message: fmt.Sprintf("Error parsing file: '%s' is not valid for type xsd:boolean, should be '0', '1', 'true' or 'false'", value)}
				}
			}
			name, field, resource, names := strings.TrimSpace(entry.Page), "apexPage", "ApexPage", pages
			if entry.XMLName.Local == "classAccesses" {
				name, field, resource, names = strings.TrimSpace(entry.Class), "apexClass", "ApexClass", classes
			}
			if name != "" && !names[strings.ToLower(name)] {
				return &VisualforceAccessMetadataError{File: path, Message: fmt.Sprintf("In field: %s - no %s named %s found", field, resource, name)}
			}
		}
	}
	return nil
}
