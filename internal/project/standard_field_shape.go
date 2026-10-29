package project

import (
	"encoding/json"
	"os"

	"github.com/glade-sh/glade/internal/config"
)

// UnspecifiedStandardFieldShape is an internal overlay-policy token, not an org
// capability. It keeps absence in a partial source tree distinct from absence
// in an explicitly declared org. It travels with feature signatures so schema,
// semantic and persisted-runtime caches cannot mix the two interpretations.
const UnspecifiedStandardFieldShape = "__glade_unspecified_standard_field_shape"

// StandardFieldPolicyFeatures augments the effective project's positive feature
// evidence with its field-policy certainty. Only the consuming project's own
// declaration establishes its target shape. Dependency development definitions
// supply positive capabilities, never proof that the consumer lacks a field.
func StandardFieldPolicyFeatures(p Project) []string {
	features := EffectiveOrgShapeFeatures(p)
	if p.Root == "" || declaresStandardFieldShape(p.Root) {
		return features
	}
	return append(features, UnspecifiedStandardFieldShape)
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
