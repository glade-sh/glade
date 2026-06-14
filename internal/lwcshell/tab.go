package lwcshell

import (
	"encoding/xml"
	"os"
	"strings"
)

type customTabXML struct {
	Label        string `xml:"label"`
	LWCComponent string `xml:"lwcComponent"`
	FlexiPage    string `xml:"flexiPage"`
	Page         string `xml:"page"`
	URL          string `xml:"url"`
	CustomObject string `xml:"customObject"`
	SObjectName  string `xml:"sobjectName"`
}

func LoadCustomTab(path string) (CustomTab, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CustomTab{}, err
	}
	var raw customTabXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return CustomTab{}, err
	}
	tab := CustomTab{
		Name:  metadataName(path, ".tab-meta.xml", ".tab"),
		Label: strings.TrimSpace(raw.Label),
		File:  path,
	}
	switch {
	case strings.TrimSpace(raw.LWCComponent) != "":
		tab.Type = TabTypeLWC
		tab.Target = strings.TrimSpace(raw.LWCComponent)
	case strings.TrimSpace(raw.FlexiPage) != "":
		tab.Type = TabTypeFlexiPage
		tab.Target = strings.TrimSpace(raw.FlexiPage)
	case strings.TrimSpace(raw.Page) != "":
		tab.Type = TabTypeVisualforce
		tab.Target = strings.TrimSpace(raw.Page)
	case strings.TrimSpace(raw.URL) != "":
		tab.Type = TabTypeWeb
		tab.Target = strings.TrimSpace(raw.URL)
	case strings.EqualFold(strings.TrimSpace(raw.CustomObject), "true") || strings.TrimSpace(raw.SObjectName) != "":
		tab.Type = TabTypeObject
		tab.Target = strings.TrimSpace(raw.SObjectName)
	default:
		tab.Type = TabTypeUnknown
	}
	return tab, nil
}
