package lwcshell

import (
	"encoding/xml"
	"os"
	"strings"
)

type CustomApplication struct {
	Name              string   `json:"name"`
	Label             string   `json:"label,omitempty"`
	NavItems          []string `json:"navItems,omitempty"`
	DefaultLandingTab string   `json:"defaultLandingTab,omitempty"`
	Console           bool     `json:"console,omitempty"`
	File              string   `json:"file,omitempty"`
}

type customApplicationXML struct {
	Label             string   `xml:"label"`
	NavType           string   `xml:"navType"`
	Tabs              []string `xml:"tabs"`
	DefaultLandingTab string   `xml:"defaultLandingTab"`
}

func LoadCustomApplication(path string) (CustomApplication, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CustomApplication{}, err
	}
	var raw customApplicationXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return CustomApplication{}, err
	}
	navItems := trimStringList(raw.Tabs)
	defaultLandingTab := strings.TrimSpace(raw.DefaultLandingTab)
	if defaultLandingTab == "" && len(navItems) > 0 {
		defaultLandingTab = navItems[0]
	}
	return CustomApplication{
		Name:              metadataName(path, ".app-meta.xml", ".app"),
		Label:             strings.TrimSpace(raw.Label),
		NavItems:          navItems,
		DefaultLandingTab: defaultLandingTab,
		Console:           strings.EqualFold(strings.TrimSpace(raw.NavType), "Console"),
		File:              path,
	}, nil
}
