package project

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/config"
)

// UnspecifiedStandardFieldShape is an internal overlay-policy token, not an org
// capability. It keeps absence in a partial source tree distinct from absence
// in an explicitly declared org. It travels with feature signatures so schema,
// semantic and persisted-runtime caches cannot mix the two interpretations.
const UnspecifiedStandardFieldShape = "__glade_unspecified_standard_field_shape"

// Edition markers are internal schema identity, not Salesforce features.
const (
	StandardFieldEditionDeveloper  = "__glade_edition_developer"
	StandardFieldEditionEnterprise = "__glade_edition_enterprise"
	StandardFieldEditionOther      = "__glade_edition_other"
)

// PlainDeveloperObjectShape marks a consuming project whose every declaration
// is a captured plain Developer shape (plain_global_describe org P and org S).
// Only then does the native plain object inventory establish object absence.
const PlainDeveloperObjectShape = "__glade_plain_developer_objects"

// StandardFieldPolicyFeatures augments the effective project's positive feature
// evidence with its field-policy certainty. Only the consuming project's own
// declaration establishes its target shape. Dependency development definitions
// supply positive capabilities, never proof that the consumer lacks a field.
func StandardFieldPolicyFeatures(p Project) []string {
	features := EffectiveOrgShapeFeatures(p)
	if edition := standardFieldRootEdition(p.Root); edition != "" {
		features = append(features, edition)
	}
	if p.Root == "" || declaresStandardFieldShape(p.Root) {
		if plainDeveloperObjectShape(p, features) {
			features = append(features, PlainDeveloperObjectShape)
		}
		return features
	}
	return append(features, UnspecifiedStandardFieldShape)
}

// Only the consuming project's declaration can identify its edition. If the
// root contains conflicting declarations, preserve uncertainty.
func standardFieldRootEdition(root string) string {
	if root == "" {
		return ""
	}
	edition := ""
	for _, path := range scratchOrgConfigFiles(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var definition struct {
			Edition string `json:"edition"`
		}
		if json.Unmarshal(data, &definition) != nil || strings.TrimSpace(definition.Edition) == "" {
			continue
		}
		current := StandardFieldEditionOther
		switch strings.ToLower(strings.TrimSpace(definition.Edition)) {
		case "developer":
			current = StandardFieldEditionDeveloper
		case "enterprise":
			current = StandardFieldEditionEnterprise
		}
		if edition != "" && edition != current {
			return ""
		}
		edition = current
	}
	return edition
}

func declaresStandardFieldShape(root string) bool {
	if cfg, _, err := config.LoadNearest(root); err == nil && cfg.Org.Features != nil {
		return true
	}
	for _, path := range scratchOrgConfigFiles(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var definition struct {
			Edition  string              `json:"edition"`
			Features scratchFeatureNames `json:"features"`
			Settings map[string]any      `json:"settings"`
		}
		if json.Unmarshal(data, &definition) == nil && (definition.Edition != "" || definition.Features != nil || definition.Settings != nil) {
			return true
		}
	}
	return false
}

// plainDeveloperObjectShape reports that the consuming project declares only
// shapes whose native global describe was captured (plain_global_describe):
// Developer edition with at most the EnableSetPasswordInApi feature and the
// lightningExperienceSettings.enableS1DesktopEnabled setting. Any other
// feature, setting, deployed settings file, dependency or org config keeps the
// catalog fallback, because Glade cannot tell which objects it adds.
func plainDeveloperObjectShape(p Project, features []string) bool {
	if standardFieldRootEdition(p.Root) != StandardFieldEditionDeveloper {
		return false
	}
	for _, feature := range features {
		switch feature {
		case StandardFieldEditionDeveloper:
		default:
			// LightningExperience is the trigger-table name of the captured
			// lightningExperienceSettings.enableS1DesktopEnabled setting.
			if !strings.EqualFold(strings.TrimSpace(feature), "EnableSetPasswordInApi") && !strings.EqualFold(strings.TrimSpace(feature), "LightningExperience") {
				return false
			}
		}
	}
	if roots := effectiveOrgShapeRoots(p); len(roots) != 1 {
		return false
	}
	if cfg, _, err := config.LoadNearest(p.Root); err == nil && len(cfg.Org.Features) > 0 {
		return false
	}
	if _, ok := sourceSettingsDefinition(p.Root); ok {
		return false
	}
	found := false
	for _, path := range scratchOrgConfigFiles(p.Root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !plainDeveloperDefinition(data) {
			return false
		}
		found = true
	}
	return found
}

func plainDeveloperDefinition(data []byte) bool {
	var definition map[string]json.RawMessage
	if json.Unmarshal(data, &definition) != nil {
		return false
	}
	for key, raw := range definition {
		switch key {
		case "orgName", "description", "edition", "country", "language":
		case "features":
			var features scratchFeatureNames
			if json.Unmarshal(raw, &features) != nil {
				return false
			}
			for _, feature := range features {
				if !strings.EqualFold(strings.TrimSpace(feature), "EnableSetPasswordInApi") {
					return false
				}
			}
		case "settings":
			var settings map[string]map[string]any
			if json.Unmarshal(raw, &settings) != nil {
				return false
			}
			for group, values := range settings {
				if group != "lightningExperienceSettings" {
					return false
				}
				for name, value := range values {
					if name != "enableS1DesktopEnabled" || value != true {
						return false
					}
				}
			}
		default:
			return false
		}
	}
	return true
}
