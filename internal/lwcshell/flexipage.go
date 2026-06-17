package lwcshell

import (
	"encoding/json"
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
	Visibility    flexiVisibilityRuleXML  `xml:"visibilityRule"`
}

type flexiComponentPropXML struct {
	Name      string            `xml:"name"`
	Value     string            `xml:"value"`
	ValueList flexiValueListXML `xml:"valueList"`
}

type flexiValueListXML struct {
	Items []flexiValueListItemXML `xml:"valueListItems"`
}

type flexiValueListItemXML struct {
	Value string `xml:"value"`
}

type flexiVisibilityRuleXML struct {
	BooleanFilter string                        `xml:"booleanFilter"`
	Criteria      []flexiVisibilityCriterionXML `xml:"criteria"`
}

type flexiVisibilityCriterionXML struct {
	LeftValue  string `xml:"leftValue"`
	Operator   string `xml:"operator"`
	RightValue string `xml:"rightValue"`
}

type VisibilityRule struct {
	BooleanFilter string                `json:"booleanFilter,omitempty"`
	Criteria      []VisibilityCriterion `json:"criteria,omitempty"`
}

type VisibilityCriterion struct {
	LeftValue  string `json:"leftValue,omitempty"`
	Operator   string `json:"operator,omitempty"`
	RightValue string `json:"rightValue,omitempty"`
}

const visibilityRuleProperty = "__glade.visibilityRule"

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
		addFlexiComponentProperty(component.Properties, prop)
	}
	for _, prop := range raw.LegacyProps {
		addFlexiComponentProperty(component.Properties, prop)
	}
	if rule, ok := visibilityRuleFromXML(raw.Visibility); ok {
		if data, err := json.Marshal(rule); err == nil {
			component.Properties[visibilityRuleProperty] = string(data)
		}
	}
	if len(component.Properties) == 0 {
		component.Properties = nil
	}
	return component
}

func addFlexiComponentProperty(out map[string]string, prop flexiComponentPropXML) {
	name := strings.TrimSpace(prop.Name)
	if name == "" {
		return
	}
	if values := prop.ValueList.Strings(); len(values) > 0 {
		data, err := json.Marshal(values)
		if err == nil {
			out[name] = string(data)
		}
		return
	}
	out[name] = strings.TrimSpace(prop.Value)
}

func (v flexiValueListXML) Strings() []string {
	out := make([]string, 0, len(v.Items))
	for _, item := range v.Items {
		value := strings.TrimSpace(item.Value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func ComponentStringListProperty(component PageComponent, name string) ([]string, bool) {
	raw := strings.TrimSpace(component.Properties[name])
	if raw == "" {
		return nil, false
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, false
	}
	return trimStringList(values), true
}

func ComponentVisibilityRule(component PageComponent) (VisibilityRule, bool) {
	raw := strings.TrimSpace(component.Properties[visibilityRuleProperty])
	if raw == "" {
		return VisibilityRule{}, false
	}
	var rule VisibilityRule
	if err := json.Unmarshal([]byte(raw), &rule); err != nil {
		return VisibilityRule{}, false
	}
	if rule.BooleanFilter == "" && len(rule.Criteria) == 0 {
		return VisibilityRule{}, false
	}
	return rule, true
}

func visibilityRuleFromXML(raw flexiVisibilityRuleXML) (VisibilityRule, bool) {
	rule := VisibilityRule{BooleanFilter: strings.TrimSpace(raw.BooleanFilter)}
	for _, criterion := range raw.Criteria {
		item := VisibilityCriterion{
			LeftValue:  strings.TrimSpace(criterion.LeftValue),
			Operator:   strings.TrimSpace(criterion.Operator),
			RightValue: strings.TrimSpace(criterion.RightValue),
		}
		if item.LeftValue == "" && item.Operator == "" && item.RightValue == "" {
			continue
		}
		rule.Criteria = append(rule.Criteria, item)
	}
	if rule.BooleanFilter == "" && len(rule.Criteria) == 0 {
		return VisibilityRule{}, false
	}
	return rule, true
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
