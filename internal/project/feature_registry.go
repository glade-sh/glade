package project

import (
	"strings"
)

// FeatureConsumer names a source file that reads a feature. The drift test
// opens File and requires the feature name in it, so a stale entry fails.
// Features read only through the embedded standard-field policy data cite that
// data file and are checked against storage.PolicyFeatureNames.
type FeatureConsumer struct {
	Component string // "storage", "vm", "resource", "project" or "storage-policy-data"
	File      string // path relative to the repository root
}

// ScratchFeatureDescriptor describes a scratch definition feature recognized by Glade.
type ScratchFeatureDescriptor struct {
	Name      string            // Canonical feature name (case-preserving)
	Aliases   []string          // Alternative spellings/aliases recognized by Glade
	HasArg    bool              // Whether feature accepts an argument suffix like :N
	Prefix    bool              // Whether feature matches as a prefix (e.g. HealthCloud)
	Consumers []FeatureConsumer // Real consumers in Glade codebase
}

var scratchFeatureRegistry = []ScratchFeatureDescriptor{
	{
		Name: "PersonAccounts",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
			{"storage", "internal/storage/standard_fields.go"},
		},
	},
	{
		Name: "MultiCurrency",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
			{"storage", "internal/storage/currency_default.go"},
			{"vm", "internal/vm/dispatch.go"},
			{"vm", "internal/vm/json_query_functions.go"},
			{"vm", "internal/vm/soql_runtime.go"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "Sites",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
		},
	},
	{
		Name:    "Communities",
		Aliases: []string{"FlowSites"},
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
			{"storage", "internal/storage/fixture_organization_groups.go"},
			{"resource", "internal/resource/networks.go"},
			{"vm", "internal/vm/class_lookup.go"},
			{"vm", "internal/vm/dispatch.go"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "StateAndCountryPicklist",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
		},
	},
	{
		Name: "ContactsToMultipleAccounts",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
		},
	},
	{
		Name:    "PlatformCache",
		Aliases: []string{"ProviderFreePlatformCache"},
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
		},
	},
	{
		Name: "EnableSetPasswordInApi",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_features.go"},
		},
	},
	{
		Name:   "AddCustomApps",
		HasArg: true,
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/fixture.go"},
		},
	},
	{
		Name: "AnalyticsAdminPerms",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_features.go"},
		},
	},
	{
		Name:   "HealthCloud",
		Prefix: true,
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_features.go"},
		},
	},
	{
		Name: "LightningExperience",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_features.go"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "Chatter",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_features.go"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "Quotes",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_object_availability.go"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "OrderManagement",
		Consumers: []FeatureConsumer{
			{"storage", "internal/storage/org_object_availability.go"},
		},
	},
	{
		Name: "B2BCommerce",
		Consumers: []FeatureConsumer{
			{"storage-policy-data", "internal/storage/standard_field_policy.json.gz"},
		},
	},
	{
		Name: "CommerceEnabled",
		Consumers: []FeatureConsumer{
			{"storage-policy-data", "internal/storage/standard_field_policy.json.gz"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "LiveAgent",
		Consumers: []FeatureConsumer{
			{"storage-policy-data", "internal/storage/standard_field_policy.json.gz"},
		},
	},
	{
		Name: "LiveAgentEnabled",
		Consumers: []FeatureConsumer{
			{"storage-policy-data", "internal/storage/standard_field_policy.json.gz"},
			{"project", "internal/project/project.go"},
		},
	},
	{
		Name: "ApexUserModeWithPermset",
		Consumers: []FeatureConsumer{
			{"vm", "internal/vm/execution_policy.go"},
		},
	},
}

// ReadScratchFeatures returns the list of all scratch definition feature names and aliases
// recognized and read by Glade.
func ReadScratchFeatures() []string {
	var out []string
	for _, desc := range scratchFeatureRegistry {
		out = append(out, desc.Name)
		out = append(out, desc.Aliases...)
	}
	return out
}

// ScratchFeatureDescriptors returns descriptors for all registered scratch features.
func ScratchFeatureDescriptors() []ScratchFeatureDescriptor {
	return append([]ScratchFeatureDescriptor(nil), scratchFeatureRegistry...)
}

// IsMappedScratchFeature reports whether Glade reads the given scratch
// definition feature. Matching is case-insensitive. AddCustomApps is read only
// as AddCustomApps:N with a numeric N. HealthCloud matches any suffix. Every
// other name must match exactly, so a suffixed name is unread.
func IsMappedScratchFeature(feature string) bool {
	lower := strings.ToLower(strings.TrimSpace(feature))
	if lower == "" {
		return false
	}
	for _, desc := range scratchFeatureRegistry {
		names := append([]string{desc.Name}, desc.Aliases...)
		for _, name := range names {
			name = strings.ToLower(name)
			switch {
			case desc.HasArg:
				arg, ok := strings.CutPrefix(lower, name+":")
				if ok && arg != "" && strings.Trim(arg, "0123456789") == "" {
					return true
				}
			case desc.Prefix:
				if strings.HasPrefix(lower, name) {
					return true
				}
			case lower == name:
				return true
			}
		}
	}
	return false
}
