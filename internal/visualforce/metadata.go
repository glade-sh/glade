package visualforce

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
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
