package lwc

import (
	"encoding/xml"
	"os"
	"strings"
)

type ComponentMeta struct {
	APIVersion    string         `xml:"apiVersion"`
	MasterLabel   string         `xml:"masterLabel"`
	IsExposed     bool           `xml:"isExposed"`
	Targets       []string       `xml:"targets>target"`
	TargetConfigs []TargetConfig `xml:"targetConfigs>targetConfig"`
}

type TargetConfig struct {
	Targets              []string
	ActionType           string
	Properties           []Property
	SupportedObjects     []string
	SupportedFormFactors []string
}

type Property struct {
	Name        string `xml:"name,attr"`
	Type        string `xml:"type,attr"`
	Label       string `xml:"label,attr"`
	Description string `xml:"description,attr"`
	Default     string `xml:"default,attr"`
	Required    bool   `xml:"required,attr"`
	Placeholder string `xml:"placeholder,attr"`
	DataSource  string `xml:"datasource,attr"`
	Min         string `xml:"min,attr"`
	Max         string `xml:"max,attr"`
	Role        string `xml:"role,attr"`
}

func ParseComponentMeta(path string) (ComponentMeta, error) {
	if path == "" {
		return ComponentMeta{}, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ComponentMeta{}, err
	}
	var meta ComponentMeta
	if err := xml.Unmarshal(data, &meta); err != nil {
		return ComponentMeta{}, err
	}
	return meta, nil
}

func (c *TargetConfig) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Targets              string                `xml:"targets,attr"`
		ActionType           string                `xml:"actionType"`
		Properties           []Property            `xml:"property"`
		SupportedObjects     []string              `xml:"objects>object"`
		SupportedFormFactors []supportedFormFactor `xml:"supportedFormFactors>supportedFormFactor"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}

	c.Targets = splitCommaList(raw.Targets)
	c.ActionType = strings.TrimSpace(raw.ActionType)
	c.Properties = raw.Properties
	c.SupportedObjects = trimStringList(raw.SupportedObjects)
	c.SupportedFormFactors = make([]string, 0, len(raw.SupportedFormFactors))
	for _, factor := range raw.SupportedFormFactors {
		if factor.Type == "" {
			continue
		}
		c.SupportedFormFactors = append(c.SupportedFormFactors, strings.TrimSpace(factor.Type))
	}
	return nil
}

func (m ComponentMeta) SupportsTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return true
	}
	for _, value := range m.Targets {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	for _, cfg := range m.TargetConfigs {
		for _, value := range cfg.Targets {
			if strings.EqualFold(strings.TrimSpace(value), target) {
				return true
			}
		}
	}
	return false
}

func (m ComponentMeta) TargetConfigFor(target string) TargetConfig {
	for _, cfg := range m.TargetConfigs {
		if target == "" || containsEqualFold(cfg.Targets, target) {
			return cfg
		}
	}
	return TargetConfig{}
}

type supportedFormFactor struct {
	Type string `xml:"type,attr"`
}

func splitCommaList(in string) []string {
	if in == "" {
		return nil
	}
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func trimStringList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func containsEqualFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}
