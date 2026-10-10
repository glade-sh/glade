package lwc

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type ComponentMeta struct {
	APIVersion    string         `xml:"apiVersion"`
	MasterLabel   string         `xml:"masterLabel"`
	IsExposed     bool           `xml:"isExposed"`
	Targets       []string       `xml:"targets>target"`
	Capabilities  []string       `xml:"capabilities>capability"`
	TargetConfigs []TargetConfig `xml:"targetConfigs>targetConfig"`
	bundleName    string
}

// MetadataValidationError retains each deployment diagnostic in its original
// order. A single invalid target can produce more than one diagnostic.
type MetadataValidationError struct {
	Messages []string
}

func (e *MetadataValidationError) Error() string {
	return strings.Join(e.Messages, "\n")
}

func (m *ComponentMeta) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		APIVersion    string         `xml:"apiVersion"`
		MasterLabel   string         `xml:"masterLabel"`
		IsExposed     string         `xml:"isExposed"`
		Targets       []string       `xml:"targets>target"`
		Capabilities  []string       `xml:"capabilities>capability"`
		TargetConfigs []TargetConfig `xml:"targetConfigs>targetConfig"`
		Unknown       []struct {
			XMLName xml.Name
		} `xml:",any"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	for _, element := range raw.Unknown {
		switch element.XMLName.Local {
		case "description": // Valid metadata text unused by the local renderer.
		default:
			return fmt.Errorf("Error parsing file: LWC Metadata Xml Parser: Unhandled XML element: %s", element.XMLName.Local)
		}
	}
	// Metadata compilation does not reject non-boolean exposure text.
	isExposed, _ := strconv.ParseBool(strings.TrimSpace(raw.IsExposed))
	// An empty targets container is valid, but an empty target entry is not.
	// Preserve entries here so Validate can distinguish those two inputs.
	targets := make([]string, len(raw.Targets))
	for i, target := range raw.Targets {
		targets[i] = strings.TrimSpace(target)
	}
	*m = ComponentMeta{
		APIVersion:    strings.TrimSpace(raw.APIVersion),
		MasterLabel:   raw.MasterLabel,
		IsExposed:     isExposed,
		Targets:       targets,
		Capabilities:  trimStringList(raw.Capabilities),
		TargetConfigs: raw.TargetConfigs,
	}
	return nil
}

// Validate checks configuration references independently of builder exposure.
func (m ComponentMeta) Validate() error {
	if err := m.validateTargets(); err != nil {
		return err
	}
	if err := m.ValidateCapabilities(); err != nil {
		return err
	}
	return m.validateTargetConfigs()
}

// ValidateCompilerConfiguration checks metadata needed before compilation.
// Capability validation follows template compilation: a disabled dynamic
// component reports its compiler diagnostic before an invalid capability.
func (m ComponentMeta) ValidateCompilerConfiguration() error {
	if err := m.validateTargets(); err != nil {
		return err
	}
	return m.validateTargetConfigs()
}

func (m ComponentMeta) validateTargets() error {
	var messages []string
	seen := map[string]bool{}
	for _, target := range m.Targets {
		if target == "" {
			messages = append(messages, fmt.Sprintf("Lightning Component Bundle %s contains empty target", m.bundleName))
		} else if seen[target] {
			messages = append(messages, fmt.Sprintf("Lightning Component Bundle %s contains duplicate target %s", m.bundleName, target))
		}
		seen[target] = true
		if !validComponentTarget(target) {
			messages = append(messages, strings.TrimSpace(fmt.Sprintf("%s is not a valid TARGETS", target)))
		}
	}
	if len(messages) != 0 {
		return &MetadataValidationError{Messages: messages}
	}
	return nil
}

// ValidateCapabilities checks the bundle's declared capabilities after its
// source has compiled. Duplicate declarations retain the actual bundle name.
func (m ComponentMeta) ValidateCapabilities() error {
	seen := map[string]bool{}
	for _, capability := range m.Capabilities {
		if !validComponentCapability(capability) {
			return fmt.Errorf("%s is not a valid CAPABILITIES", capability)
		}
		if seen[capability] {
			return fmt.Errorf("Lightning Component Bundle %s contains duplicate capability %s", m.bundleName, capability)
		}
		seen[capability] = true
	}
	return nil
}

func (m ComponentMeta) validateTargetConfigs() error {
	for _, cfg := range m.TargetConfigs {
		for _, target := range cfg.Targets {
			if !slices.Contains(m.Targets, target) {
				return fmt.Errorf("targetConfig target %q is not specified in the targets section", target)
			}
			if target == "lightning__Tab" {
				if len(cfg.Properties) != 0 {
					return &MetadataValidationError{Messages: []string{"The 'property' tag isn't supported for lightning__Tab"}}
				}
				if len(cfg.SupportedFormFactors) != 0 {
					return &MetadataValidationError{Messages: []string{"The 'supportedFormFactors' tag isn't supported for lightning__Tab"}}
				}
			}
			if target == "lightning__RecordAction" && cfg.ActionType != "" && cfg.ActionType != "Action" && cfg.ActionType != "ScreenAction" {
				return &MetadataValidationError{Messages: []string{fmt.Sprintf("Invalid <actionType> tag value '%s'. Allowed values: Action, ScreenAction.", cfg.ActionType)}}
			}
		}
		for _, factor := range cfg.SupportedFormFactors {
			if factor == "" {
				return &MetadataValidationError{Messages: []string{"You must specify a type for formfactor tag"}}
			}
			if factor != "Large" && factor != "Small" {
				return &MetadataValidationError{Messages: []string{fmt.Sprintf("The formfactor type '%s' isn't valid.", factor)}}
			}
		}
		if slices.Contains(cfg.Targets, "lightning__RecordPage") {
			for _, factor := range cfg.SupportedFormFactors {
				if factor != "Large" && factor != "Small" {
					return fmt.Errorf("The formfactor type '%s' isn't valid.", factor)
				}
			}
		}
	}
	return m.validatePublicAPIProperties()
}

// Configuration enums: https://developer.salesforce.com/docs/platform/lwc/guide/reference-configuration-tags.html
func validComponentTarget(target string) bool {
	for _, known := range []string{"analytics__Dashboard",
		"lightningCommunity__Default", "lightningCommunity__Page", "lightningCommunity__Page_Layout", "lightningCommunity__Theme_Layout",
		"lightningSnapin__ChatHeader", "lightningSnapin__ChatMessage", "lightningSnapin__MessagingPreChat", "lightningSnapin__MessagingHeader", "lightningSnapin__Minimized", "lightningSnapin__PreChat",
		"lightningStatic__Email",
		"lightning__AgentforceInput", "lightning__AgentforceOutput", "lightning__AppPage", "lightning__ECSFSApp", "lightning__EnablementProgram",
		"lightning__FlowScreen", "lightning__GlobalAction", "lightning__HomePage", "lightning__Inbox", "lightning__PropertyEditor",
		"lightning__RecordAction", "lightning__RecordPage", "lightning__ServiceDocument", "lightning__Tab", "lightning__UrlAddressable", "lightning__UtilityBar", "lightning__VoiceExtension"} {
		if target == known {
			return true
		}
	}
	return false
}

func validComponentCapability(capability string) bool {
	switch capability {
	case "lightningCommunity__RelaxedCSP", "lightning__dynamicComponent", "lightning__ServerRenderable", "lightning__ServerRenderableWithHydration", "lightning__ServiceCloudVoiceToolkitApi":
		return true
	default:
		return false
	}
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
	meta.bundleName = strings.TrimSuffix(filepath.Base(path), ".js-meta.xml")
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
