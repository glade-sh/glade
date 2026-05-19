package probe

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/open-aer/oaer/internal/capability"
)

const (
	ProbeVersion    = "2026-05-06"
	ManifestVersion = "2026-05-06.2"
	SeedVersion     = "2026-05-06.1"
)

type ProbeIsolation string

const (
	ProbeIsolationPure           ProbeIsolation = "pure"
	ProbeIsolationStateful       ProbeIsolation = "stateful"
	ProbeIsolationLimitSensitive ProbeIsolation = "limit_sensitive"
)

// ProbeSpec is the runner manifest for one framework-shape probe.
type ProbeSpec struct {
	ID              string         `json:"id"`
	Category        string         `json:"category"`
	Isolation       ProbeIsolation `json:"isolation"`
	CanBatch        bool           `json:"canBatch"`
	Tier            string         `json:"tier"`
	SeedProfile     string         `json:"seedProfile"`
	RequiresFeature []string       `json:"requiresFeature,omitempty"`
}

func defaultProbeSpecs() []ProbeSpec {
	ids := []string{
		"stdlib.string.format-null",
		"stdlib.string.join-empty",
		"stdlib.string.containsIgnoreCase-null",
		"stdlib.string.valueOf-null",
		"stdlib.string.isBlank-whitespace",
		"stdlib.datetime.valueOf-null",
		"stdlib.datetime.leapYear",
		"stdlib.datetime.format-timezone",
		"stdlib.datetime.valueOf-invalid",
		"stdlib.datetime.yearZero",
		"stdlib.math.divide-scale",
		"stdlib.math.mod-negative",
		"stdlib.math.round-halfUp",
		"stdlib.math.decimalValueOf-null",
		"stdlib.math.log10",
		"soql.select-all",
		"soql.aggregate-count",
		"soql.where-like",
		"soql.order-desc",
		"soql.dynamic",
		"dml.insert-trigger",
		"dml.update-return",
		"dml.delete-return",
		"dml.undelete",
		"dml.insert-fail-duplicate",
		"limits.soql-before-after",
		"limits.dml-rows",
		"limits.heap-size",
		"limits.limit-queries",
		"limits.cpu-time",
		"collections.list-contains-null",
		"collections.map-null-key",
		"collections.set-contains-null",
		"collections.list-indexof-null",
		"collections.map-remove-null",
		"async.future-stub",
		"async.queueable-stub",
		"async.batchable-stub",
		"async.schedulable-stub",
		"platform-event.publish",
		"platform-event.describe",
		"metadata.custom-metadata-query",
		"metadata.custom-setting-query",
		"metadata.custom-metadata-describe",
		"email.single-message",
		"email.limits",
		"schema.global-describe-size",
		"schema.object-describe-fields",
		"schema.picklist-describe",
		"schema.record-type-describe",
		"security.user-info",
		"security.profile-name",
		"security.crud-check",
		"security.fls-check",
		"integration.http-request",
		"integration.json-serialize-complex",
		"integration.json-deserialize-untyped",
		"integration.encoding-util",
		"integration.json-serialize-sobject",
		"integration.url-encoding",
		"bulkdml.partial-success",
		"bulkdml.errors-shape",
		"bulkdml.id-assignment",
		"sobject.clone",
		"sobject.get-put",
		"sobject.getSObjectType",
		"id.validation",
		"collections.list-sort",
		"collections.map-keyset",
		"collections.set-equality",
		"datetime.now",
		"datetime.today",
		"datetime.valueOfGmt",
		"string.escapeSingleQuotes",
		"string.split",
		"string.trim",
		"soql.count-distinct",
		"soql.group-by",
		"soql.subquery",
		"system.assert-pass",
		"system.assertEquals",
		"math.abs-negative",
		"math.max",
		"math.pow",
		"math.min",
		"shape.exception-type-message",
		"shape.soql-row-attributes",
		"shape.json-sobject-attributes",
		"shape.describe-field-type-token",
	}
	specs := make([]ProbeSpec, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		spec := classifyProbe(id)
		specs = append(specs, spec)
		seen[spec.ID] = struct{}{}
	}
	for _, generated := range loadGeneratedStubContractProbeSpecs() {
		if _, ok := seen[generated.ID]; ok {
			continue
		}
		specs = append(specs, generated)
		seen[generated.ID] = struct{}{}
	}
	return specs
}

func defaultProbeIDs() []string {
	specs := defaultProbeSpecs()
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		ids = append(ids, spec.ID)
	}
	return ids
}

func probeSpecByID(id string) ProbeSpec {
	return classifyProbe(id)
}

func classifyProbe(id string) ProbeSpec {
	spec := ProbeSpec{
		ID:          id,
		Category:    categoryForProbe(id),
		Isolation:   ProbeIsolationPure,
		CanBatch:    true,
		Tier:        "full",
		SeedProfile: "base",
	}
	switch {
	case strings.HasPrefix(id, "dml."), strings.HasPrefix(id, "bulkdml."), id == "platform-event.publish":
		spec.Isolation = ProbeIsolationStateful
		spec.CanBatch = false
	case strings.HasPrefix(id, "limits."), id == "email.limits":
		spec.Isolation = ProbeIsolationLimitSensitive
		spec.CanBatch = false
	}
	if strings.HasPrefix(id, "soql.") || strings.HasPrefix(id, "dml.") || strings.HasPrefix(id, "bulkdml.") || id == "limits.dml-rows" {
		spec.SeedProfile = "probe-test-object"
	}
	if strings.Contains(id, "currency") || id == "stdlib.math.divide-scale" {
		spec.RequiresFeature = []string{"MultiCurrency"}
	}
	switch id {
	case "stdlib.string.format-null", "soql.select-all", "dml.insert-trigger", "schema.picklist-describe", "integration.json-deserialize-untyped", "shape.exception-type-message", "shape.json-sobject-attributes":
		spec.Tier = "smoke"
	}
	return spec
}

func categoryForProbe(id string) string {
	switch {
	case strings.HasPrefix(id, "stdlib."), strings.HasPrefix(id, "datetime."), strings.HasPrefix(id, "string."), strings.HasPrefix(id, "math."), strings.HasPrefix(id, "system."):
		return "Stdlib & System"
	case strings.HasPrefix(id, "soql."):
		return "Data Runtime"
	case strings.HasPrefix(id, "dml."), strings.HasPrefix(id, "bulkdml."):
		return "DML & Triggers"
	case strings.HasPrefix(id, "limits."):
		return "Limits & System"
	case strings.HasPrefix(id, "collections."):
		return "Collections & Language"
	case strings.HasPrefix(id, "async."):
		return "Async Apex"
	case strings.HasPrefix(id, "platform-event."):
		return "Platform Events"
	case strings.HasPrefix(id, "metadata."):
		return "Metadata"
	case strings.HasPrefix(id, "email."):
		return "Email & Messaging"
	case strings.HasPrefix(id, "schema."):
		return "Schema Describe"
	case strings.HasPrefix(id, "security."):
		return "Security & Sharing"
	case strings.HasPrefix(id, "integration."):
		return "Integration"
	case strings.HasPrefix(id, "sobject."), id == "id.validation":
		return "SObject & Type"
	case strings.HasPrefix(id, "shape."):
		return "Framework Shape"
	default:
		return "Uncategorized"
	}
}

func probeIDsForTier(tier string) []string {
	specs := defaultProbeSpecs()
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		if tier == "" || tier == "full" || probeTierMatches(tier, spec.Tier) {
			ids = append(ids, spec.ID)
		}
	}
	return ids
}

// ProbeIDsForTier returns probe IDs for the named tier.
// Tiers include built-in and generated stub-contract tiers.
func ProbeIDsForTier(tier string) []string {
	return probeIDsForTier(strings.ToLower(strings.TrimSpace(tier)))
}

func probeTierMatches(requested, actual string) bool {
	switch requested {
	case "core":
		return actual == "core" || actual == "smoke"
	default:
		return requested == actual
	}
}

func loadGeneratedStubContractProbeSpecs() []ProbeSpec {
	const manifestPath = "docs/generated/STUB_CONTRACT_PROBE_MANIFEST.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var manifest []capability.StubContractProbeSpec
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	out := make([]ProbeSpec, 0, len(manifest))
	for _, spec := range manifest {
		isolation := ProbeIsolationPure
		if spec.Mode == capability.StubContractLocalOnly {
			isolation = ProbeIsolationStateful
		}
		out = append(out, ProbeSpec{
			ID:              spec.ID,
			Category:        "Stub Contracts",
			Isolation:       isolation,
			CanBatch:        spec.CanBatch,
			Tier:            normalizeGeneratedTier(spec.Tier),
			SeedProfile:     "base",
			RequiresFeature: nil,
		})
	}
	return out
}

func normalizeGeneratedTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "smoke":
		return "smoke"
	case "core":
		return "core"
	case "local":
		return "local"
	default:
		return "full"
	}
}
