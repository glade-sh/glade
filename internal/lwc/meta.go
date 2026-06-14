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
		Properties           []Property            `xml:"property"`
		SupportedObjects     []string              `xml:"objects>object"`
		SupportedFormFactors []supportedFormFactor `xml:"supportedFormFactors>supportedFormFactor"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}

	c.Targets = splitCommaList(raw.Targets)
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
