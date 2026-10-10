package resource

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type messageChannelXML struct {
	XMLName     xml.Name `xml:"LightningMessageChannel"`
	MasterLabel *string  `xml:"masterLabel"`
	IsExposed   *string  `xml:"isExposed"`
	Description string   `xml:"description"`
	Fields      []struct {
		Name        *string `xml:"fieldName"`
		Description string  `xml:"description"`
	} `xml:"lightningMessageFields"`
	Unknown []struct {
		XMLName xml.Name
	} `xml:",any"`
}

func loadMessageChannel(path, namespace string) (storage.MessageChannelMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return storage.MessageChannelMetadata{}, err
	}
	var definition messageChannelXML
	if err := xml.Unmarshal(content, &definition); err != nil {
		return storage.MessageChannelMetadata{}, fmt.Errorf("Error parsing file: %w", err)
	}
	if len(definition.Unknown) != 0 {
		name := definition.Unknown[0].XMLName
		return storage.MessageChannelMetadata{}, fmt.Errorf("Error parsing file: Element {%s}%s invalid at this location in type LightningMessageChannel", name.Space, name.Local)
	}
	if definition.MasterLabel == nil {
		return storage.MessageChannelMetadata{}, fmt.Errorf("Required field is missing: masterLabel")
	}
	if *definition.MasterLabel == "" {
		return storage.MessageChannelMetadata{}, fmt.Errorf("Required fields are missing: [MasterLabel]")
	}
	channel := storage.MessageChannelMetadata{
		Name:        trimKnownSuffix(filepath.Base(path), ".messageChannel-meta.xml"),
		Namespace:   namespace,
		MasterLabel: *definition.MasterLabel,
		Description: definition.Description,
		File:        path,
	}
	if definition.IsExposed != nil {
		switch strings.TrimSpace(*definition.IsExposed) {
		case "true", "1":
			channel.IsExposed = true
		case "false", "0":
		default:
			return storage.MessageChannelMetadata{}, fmt.Errorf("Error parsing file: '%s' is not valid for type xsd:boolean, should be '0', '1', 'true' or 'false'", *definition.IsExposed)
		}
	}
	seen := make(map[string]bool, len(definition.Fields))
	for _, field := range definition.Fields {
		if field.Name == nil || *field.Name == "" {
			return storage.MessageChannelMetadata{}, fmt.Errorf("Required field is missing: fieldName")
		}
		if seen[*field.Name] {
			return storage.MessageChannelMetadata{}, fmt.Errorf("You can't have multiple lightning message fields with the same fieldName")
		}
		seen[*field.Name] = true
		channel.Fields = append(channel.Fields, storage.MessageChannelField{Name: *field.Name, Description: field.Description})
	}
	return channel, nil
}
