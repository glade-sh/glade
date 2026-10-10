package lwc

import (
	"fmt"
	"slices"
	"strconv"
)

// validatePublicAPIProperties covers AppPage and RecordPage property configuration.
// Other targets (notably Flow's richer type system) retain their own rules.
// Native metadata rejection cases for unknown types, invalid scalars, duplicates and
// the valid String/Integer/Boolean controls back these checks at API 59/67.
func (m ComponentMeta) validatePublicAPIProperties() error {
	for _, cfg := range m.TargetConfigs {
		if !slices.Contains(cfg.Targets, "lightning__AppPage") {
			// RecordPage controls back the property type restriction.
			// Default values and duplicate properties keep their target rules.
			if slices.Contains(cfg.Targets, "lightning__RecordPage") {
				for _, prop := range cfg.Properties {
					if !slices.Contains([]string{"String", "Integer", "Boolean"}, prop.Type) {
						return fmt.Errorf("You specified an invalid type for '%s'", prop.Name)
					}
				}
			}
			continue
		}
		seen := make(map[string]bool, len(cfg.Properties))
		for _, prop := range cfg.Properties {
			if seen[prop.Name] {
				return fmt.Errorf("More than one property has the same name: '%s'", prop.Name)
			}
			seen[prop.Name] = true
			switch prop.Type {
			case "String":
			case "Integer":
				if prop.Default != "" && !publicAPIIntegerDefault(prop.Default) {
					return fmt.Errorf("The default value you specified '%s' is not a valid integer.", prop.Default)
				}
			case "Boolean":
				if prop.Default != "" {
					if _, err := strconv.ParseBool(prop.Default); err != nil {
						return fmt.Errorf("The default value you specified '%s' is invalid. It must be a Boolean.", prop.Default)
					}
				}
			default:
				return fmt.Errorf("You specified an invalid type for '%s'", prop.Name)
			}
		}
	}
	return nil
}

func publicAPIIntegerDefault(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
