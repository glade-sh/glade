package lwcshell

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

type flexiPageXML struct {
	MasterLabel string             `xml:"masterLabel"`
	Type        string             `xml:"type"`
	SObjectType string             `xml:"sobjectType"`
	Template    flexiPageTemplate  `xml:"template"`
	Regions     []flexiPageRegionX `xml:"flexiPageRegions"`
}

type flexiPageTemplate struct {
	Name string `xml:"name"`
}

type flexiPageRegionX struct {
	Name                      string                   `xml:"name"`
	Type                      string                   `xml:"type"`
	ItemInstances             []flexiPageItemInstance  `xml:"itemInstances"`
	LegacyComponentInstances  []flexiComponentInstance `xml:"componentInstances"`
	ComponentInstanceFallback []flexiComponentInstance `xml:"componentInstance"`
}

type flexiPageItemInstance struct {
	Component flexiComponentInstance `xml:"componentInstance"`
}

type flexiComponentInstance struct {
	ComponentName string                  `xml:"componentName"`
	Identifier    string                  `xml:"identifier"`
	Properties    []flexiComponentPropXML `xml:"componentInstanceProperties"`
	LegacyProps   []flexiComponentPropXML `xml:"properties"`
}

type flexiComponentPropXML struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

func LoadFlexiPage(path string) (FlexiPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FlexiPage{}, err
	}
	var raw flexiPageXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return FlexiPage{}, err
	}
	page := FlexiPage{
		Name:          metadataName(path, ".flexipage-meta.xml", ".flexipage"),
		Label:         strings.TrimSpace(raw.MasterLabel),
		Type:          strings.TrimSpace(raw.Type),
		ObjectAPIName: strings.TrimSpace(raw.SObjectType),
		Template:      strings.TrimSpace(raw.Template.Name),
		File:          path,
	}
	for _, rawRegion := range raw.Regions {
		region := PageRegion{
			Name: strings.TrimSpace(rawRegion.Name),
			Type: strings.TrimSpace(rawRegion.Type),
		}
		for _, item := range rawRegion.ItemInstances {
			if component := pageComponentFromXML(item.Component); component.ComponentName != "" {
				region.Components = append(region.Components, component)
			}
		}
		for _, rawComponent := range rawRegion.LegacyComponentInstances {
			if component := pageComponentFromXML(rawComponent); component.ComponentName != "" {
				region.Components = append(region.Components, component)
			}
		}
		for _, rawComponent := range rawRegion.ComponentInstanceFallback {
			if component := pageComponentFromXML(rawComponent); component.ComponentName != "" {
				region.Components = append(region.Components, component)
			}
		}
		page.Regions = append(page.Regions, region)
	}
	return page, nil
}

func pageComponentFromXML(raw flexiComponentInstance) PageComponent {
	component := PageComponent{
		ComponentName: strings.TrimSpace(raw.ComponentName),
		Identifier:    strings.TrimSpace(raw.Identifier),
		Properties:    make(map[string]string),
	}
	for _, prop := range raw.Properties {
		name := strings.TrimSpace(prop.Name)
		if name == "" {
			continue
		}
		component.Properties[name] = strings.TrimSpace(prop.Value)
	}
	for _, prop := range raw.LegacyProps {
		name := strings.TrimSpace(prop.Name)
		if name == "" {
			continue
		}
		component.Properties[name] = strings.TrimSpace(prop.Value)
	}
	if len(component.Properties) == 0 {
		component.Properties = nil
	}
	return component
}

func metadataName(path string, suffixes ...string) string {
	name := filepath.Base(path)
	for _, suffix := range suffixes {
		if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
			return name[:len(name)-len(suffix)]
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}
