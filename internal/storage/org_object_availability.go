package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

//go:generate python3 ../../scripts/generate_order_management_objects.py
//go:generate gofmt -w order_management_objects_generated.go b2bavailable_objects_generated.go b2bunavailable_objects_generated.go network_setting_objects_generated.go plain_developer_objects_generated.go

// Only feature transitions backed by native A/B rows belong here. Other
// catalog objects retain their existing policy until native captures establish
// their feature or setting trigger.
var standardObjectFeatures = []struct{ object, feature string }{
	{"AccountContactRelation", "ContactsToMultipleAccounts"},
	{"Knowledge__ka", "Knowledge"},
	{"Knowledge__kav", "Knowledge"},
	{"CurrencyType", "MultiCurrency"},
	{"DatedConversionRate", "MultiCurrency"},
	{"Territory2", "Territory2"},
	{"Territory2Model", "Territory2"},
	{"Territory2Type", "Territory2"},
	{"UserTerritory2Association", "Territory2"},
	{"ObjectTerritory2Association", "Territory2"},
	{"AccountTeamMember", "AccountTeams"},
	{"ForecastingItem", "Forecasts"},
	{"ForecastingQuota", "Forecasts"},
	{"ForecastingType", "Forecasts"},
	{"OpportunityTeamMember", "OpportunityTeam"},
	{"OpportunitySplit", "OpportunitySplits"},
	{"ServiceChannel", "OmniChannel"},
	{"Survey", "Surveys"},
	{"Quote", "Quotes"},
	{"QuoteLineItem", "Quotes"},
	{"QuoteDocument", "Quotes"},
	{"QuoteFeed", "Quotes"},
	{"QuoteHistory", "Quotes"},
	{"QuoteLineItemHistory", "Quotes"},
	{"QuoteShare", "Quotes"},
	{"WebStore", "OrderManagement"},
	{"GiftTransaction", "FundraisingEnabled"},
	{"GiftCommitment", "FundraisingEnabled"},
	{"GiftCommitmentSchedule", "FundraisingEnabled"},
	{"OutreachSourceCode", "FundraisingEnabled"},
	{"PaymentInstrument", "FundraisingEnabled"},
	{"DocumentGenerationProcess", "DocGen"},
	{"DocumentTemplateContentDoc", "DocGen"},
	{"DocumentTemplate", "DocGen"},
	{"DocumentTemplateSection", "DocGen"},
	{"DocumentTemplateToken", "DocGen"},
	{"RegulatoryCode", "PublicSectorAccess"},
	// Native captures (org_split_influence_source): the objects are absent in
	// a plain Developer org at API 62.0 and 67.0. OpportunitySplit and
	// OpportunitySplitType appear after the Opportunity Splits Setup wizard;
	// no deployable setting does that, so only org.features names it.
	// CampaignInfluenceModel appears with campaignSettings.enableCampaignInfluence2.
	{"OpportunitySplitType", "OpportunitySplits"},
	{"CampaignInfluenceModel", "CampaignInfluence2Enabled"},
	{"PickTicket", "OrderManagement"},
	{"PickTicketAssignment", "OrderManagement"},
	{"PickTicketFeed", "OrderManagement"},
	{"PickTicketProduct", "OrderManagement"},
	{"PickTicketProductFeed", "OrderManagement"},
	{"PickTicketShare", "OrderManagement"},
	{"UserLocationAssignment", "OrderManagement"},
	{"PaymentCredit", "OrderManagement"},
	{"PaymentCreditLinePayment", "OrderManagement"},
	{"PaymentCreditTransaction", "OrderManagement"},
	{"WebCartCredit", "OrderManagement"},
	{"PendingOrderSummary", "OrderManagement"},
}

// AllowsStandardObject checks the declared shape, not whether an object is in
// the catalog or physically hydrated. An unspecified project cannot establish
// feature absence. Captured provisioning alternatives share the constructor
// policy so compilation and runtime availability agree.
// HasCapturedB2BCommerceShape identifies the exact native-probed combination.
func (o *OrgState) HasCapturedB2BCommerceShape() bool {
	return o != nil && o.HasFeature("B2BCommerce") && o.HasFeature("Communities") && o.HasFeature("NetworksEnabled") && o.HasFeature("CommerceEnabled")
}

func (o *OrgState) AllowsStandardObject(objectName string) bool {
	if o != nil && o.HasFeature(project.PlainDeveloperObjectShape) && !PlainDeveloperObjectPresent(objectName, PlainDeveloperAnyRoute) {
		return false
	}
	if o.HasCapturedB2BCommerceShape() && !o.HasFeature(project.UnspecifiedStandardFieldShape) {
		name := strings.ToLower(objectName)
		if _, ok := b2bAvailableObjectLookup[name]; ok {
			return true
		}
		if name == "pendingordersummary" || name == "webcartcredit" {
			return true
		}
		if _, ok := b2bUnavailableObjectLookup[name]; ok && !o.HasFeature("OrderManagement") {
			return false
		}
	}
	if _, captured := orderManagementObjectLookup[strings.ToLower(objectName)]; captured {
		if o == nil {
			return false
		}
		if o.HasFeature(project.UnspecifiedStandardFieldShape) || o.HasFeature("OrderManagement") {
			return true
		}
		// Captured rows cover the order-management and exact B2B shapes only.
		// A shape the policy already admitted (for example B2BCommerce,
		// Communities and CommerceEnabled without the Networks setting) has no
		// native row, so it keeps the policy answer it had before.
		if policy := standardFieldPolicyFor(objectName); policy != nil &&
			(len(policy.RequiredFeatures) > 0 || len(policy.RequiredFeaturesAnyOf) > 0) {
			return StandardObjectConstructorAvailableForFeatures(objectName, o.Features)
		}
		return false
	}
	if _, captured := networkSettingObjectLookup[strings.ToLower(objectName)]; captured {
		if o == nil {
			return false
		}
		if stringsEqualFold(objectName, "Network") || stringsEqualFold(objectName, "NetworkMember") {
			return o.HasFeature("NetworksEnabled")
		}
		return o.HasFeature(project.UnspecifiedStandardFieldShape) || o.HasFeature("NetworksEnabled")
	}
	// The captured capture-revision-010093106582497266713559480549844075860237667873875753689875218990361047144977 shapes lack the CRM User license. Glade
	// has no licensed shape yet; keep unspecified projects as the fallback.
	if stringsEqualFold(objectName, "CampaignInfluence") {
		return o != nil && o.HasFeature(project.UnspecifiedStandardFieldShape)
	}
	for _, gate := range standardObjectFeatures {
		if stringsEqualFold(objectName, gate.object) {
			if o == nil {
				return false
			}
			if o.HasFeature(project.UnspecifiedStandardFieldShape) {
				return true
			}
			if (stringsEqualFold(objectName, "OpportunitySplit") || stringsEqualFold(objectName, "OpportunitySplitType")) && !o.HasFeature("OpportunityTeam") {
				return false
			}
			if (strings.HasPrefix(gate.object, "PickTicket") || gate.object == "UserLocationAssignment") && !o.HasFeature("OrderManagementEnabled") {
				return false
			}
			if policy := standardFieldPolicyFor(objectName); policy != nil &&
				(len(policy.RequiredFeatures) > 0 || len(policy.RequiredFeaturesAnyOf) > 0) {
				return StandardObjectConstructorAvailableForFeatures(objectName, o.Features)
			}
			if gate.feature == "FundraisingEnabled" && !o.HasFeature("Fundraising") {
				return false
			}
			// Knowledge objects follow the scratch setting (KnowledgeEnabled);
			// the license feature "Knowledge" alone yields no objects, and an
			// unspecified shape is ungated (captured shape rule).
			if gate.feature == "Knowledge" {
				return o.HasFeature("KnowledgeEnabled")
			}
			return o.HasFeature(gate.feature)
		}
	}
	return true
}

func refreshOrgFeatureObjects(org *OrgState) {
	// The plain Developer inventory is a union of the captured API and route
	// lists. Preserve non-catalog project objects when applying this profile.
	if org.HasFeature(project.PlainDeveloperObjectShape) && (org.eagerStandardSpread || org.lazyStandardSpread || org.StandardSpread != "") {
		for name, object := range org.Objects {
			if plainDeveloperPrunable(name) {
				// Only remove untouched catalog entries, retaining project schema
				// and data that were installed before the shape was applied.
				if len(object.Records) != 0 || !isUnmodifiedSpreadObject(org, name, object) {
					continue
				}
				DeleteObject(org, name)
				if org.eagerStandardSpread {
					org.StandardSpreadPruned = true
				}
			}
		}
	}
	if org.HasCapturedB2BCommerceShape() && !org.HasFeature(project.UnspecifiedStandardFieldShape) {
		for _, names := range [][]string{b2bAvailableObjectNames, b2bUnavailableObjectNames} {
			for _, name := range names {
				if !org.AllowsStandardObject(name) {
					DeleteObject(org, name)
				} else if _, exists := org.Objects[name]; !exists {
					EnsureStandardObject(org, name)
				}
			}
		}
	}
	for _, name := range orderManagementObjectNames {
		if !org.AllowsStandardObject(name) {
			DeleteObject(org, name)
		} else if _, exists := org.Objects[name]; !exists {
			EnsureStandardObject(org, name)
		}
	}
	for _, name := range networkSettingObjectNames {
		if !org.AllowsStandardObject(name) {
			DeleteObject(org, name)
		} else if _, exists := org.Objects[name]; !exists {
			EnsureStandardObject(org, name)
		}
	}
	if !org.AllowsStandardObject("CampaignInfluence") {
		DeleteObject(org, "CampaignInfluence")
	}
	for _, gate := range standardObjectFeatures {
		if !org.AllowsStandardObject(gate.object) {
			DeleteObject(org, gate.object)
		} else if _, exists := org.Objects[gate.object]; !exists {
			EnsureStandardObject(org, gate.object)
		}
	}
}

func plainDeveloperPrunable(name string) bool {
	switch strings.ToLower(name) {
	case "aifeatureextractor", "airesearchpromptresult", "aiscoringstatusevent":
		return true
	}
	return false
}

func isUnmodifiedSpreadObject(org *OrgState, name string, object ObjectState) bool {
	if org.spreadExplicit[name] {
		return false
	}
	// Definitions change copy-on-write, so a definition that still holds every
	// value and collection the spread installed is the catalog build itself.
	if installed, ok := org.spreadInstalled[name]; ok {
		return spreadDefinitionFingerprint(object.Definition) == installed.fingerprint
	}
	baseline := NewOrgState()
	ensureStandardObjectWithProfile(&baseline, name, nil)
	return reflect.DeepEqual(object.Definition, baseline.Objects[name].Definition)
}

// Plain Developer route masks over plainDeveloperObjectLookup bits.
const (
	PlainDeveloperAnonymous62 uint8 = 1
	PlainDeveloperAnonymous67 uint8 = 2
	PlainDeveloperTestOnly62  uint8 = 4
	PlainDeveloperTestOnly67  uint8 = 8
	PlainDeveloperAnyRoute          = PlainDeveloperAnonymous62 | PlainDeveloperAnonymous67 | PlainDeveloperTestOnly62 | PlainDeveloperTestOnly67
)

// PlainDeveloperObjectPresent reports whether a standard object is in the
// native plain Developer global describe for any route in mask
// (plain_global_describe). Names that are neither catalog objects nor derived
// standard entities (custom and project objects) are always present here.
func PlainDeveloperObjectPresent(objectName string, mask uint8) bool {
	name := strings.ToLower(strings.TrimSpace(objectName))
	if bits, listed := plainDeveloperObjectLookup[name]; listed {
		return bits&mask != 0
	}
	if _, known := ResolveKnownStandardObjectName(name); known {
		return false
	}
	// Derived standard entities outside the catalog follow the same list.
	if !strings.Contains(name, "__") {
		for _, suffix := range []string{"changeevent", "share", "feed", "history"} {
			if strings.HasSuffix(name, suffix) {
				return false
			}
		}
	}
	return true
}

// installedSpreadDefinition is a shallow copy of a definition. A map shares
// its entries with the copy, so the entry counts are kept separately.
type installedSpreadDefinition struct {
	definition       ObjectDefinition
	fields, metadata int
	fingerprint      [32]byte
}

func recordInstalledSpreadDefinition(definition ObjectDefinition) installedSpreadDefinition {
	return installedSpreadDefinition{definition: definition, fields: len(definition.Fields), metadata: len(definition.Metadata), fingerprint: spreadDefinitionFingerprint(definition)}
}

// sameDefinitionInstance reports whether a definition still holds the
// recorded scalar values, and the recorded map and slice instances at their
// recorded lengths.
func sameDefinitionInstance(a ObjectDefinition, recorded installedSpreadDefinition) bool {
	b := recorded.definition
	return spreadDefinitionFingerprint(a) == recorded.fingerprint && a.APIName == b.APIName && a.Label == b.Label && a.PluralLabel == b.PluralLabel &&
		a.KeyPrefix == b.KeyPrefix && a.SharingModel == b.SharingModel && a.EnableSearch == b.EnableSearch &&
		a.ExternalSharingModel == b.ExternalSharingModel &&
		len(a.Fields) == recorded.fields && sameMapInstance(a.Fields, b.Fields) &&
		len(a.Metadata) == recorded.metadata && sameMapInstance(a.Metadata, b.Metadata) &&
		sameSliceInstance(a.OwnerSharingRules, b.OwnerSharingRules) && sameSliceInstance(a.Relations, b.Relations) &&
		sameSliceInstance(a.RecordTypes, b.RecordTypes) && sameSliceInstance(a.ValidationRules, b.ValidationRules) &&
		sameSliceInstance(a.AutoResponseRules, b.AutoResponseRules) && sameSliceInstance(a.AssignmentRules, b.AssignmentRules) &&
		sameSliceInstance(a.DuplicateRules, b.DuplicateRules) && sameSliceInstance(a.WorkflowRules, b.WorkflowRules) &&
		sameSliceInstance(a.FlowRules, b.FlowRules) && sameSliceInstance(a.Indexes, b.Indexes) &&
		sameSliceInstance(a.CriteriaSharingRules, b.CriteriaSharingRules)
}

func sameMapInstance[M ~map[K]V, K comparable, V any](a, b M) bool {
	return reflect.ValueOf(a).UnsafePointer() == reflect.ValueOf(b).UnsafePointer()
}

func sameSliceInstance[T any](a, b []T) bool {
	if len(a) != len(b) || cap(a) != cap(b) || (a == nil) != (b == nil) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// spreadDefinitionFingerprint snapshots values, including nested collection entries.
func spreadDefinitionFingerprint(d ObjectDefinition) [32]byte {
	h := sha256.New()
	hashSpreadValue(h, reflect.ValueOf(d))
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// hashSpreadValue includes nil markers and all fields rather than JSON's
// omitempty projection. Maps are ordered so the fingerprint is stable.
func hashSpreadValue(h hash.Hash, v reflect.Value) {
	h.Write([]byte{byte(v.Kind())})
	writeNumber := func(n uint64) { var b [8]byte; binary.LittleEndian.PutUint64(b[:], n); h.Write(b[:]) }
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			h.Write([]byte{0})
			return
		}
		h.Write([]byte{1})
		hashSpreadValue(h, v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			hashSpreadValue(h, v.Field(i))
		}
	case reflect.Map:
		if v.IsNil() {
			h.Write([]byte{0})
			return
		}
		h.Write([]byte{1})
		writeNumber(uint64(v.Len()))
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, k := range keys {
			hashSpreadValue(h, k)
			hashSpreadValue(h, v.MapIndex(k))
		}
	case reflect.Slice:
		if v.IsNil() {
			h.Write([]byte{0})
			return
		}
		h.Write([]byte{1})
		writeNumber(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			hashSpreadValue(h, v.Index(i))
		}
	case reflect.Array:
		writeNumber(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			hashSpreadValue(h, v.Index(i))
		}
	case reflect.String:
		value := v.String()
		writeNumber(uint64(len(value)))
		h.Write([]byte(value))
	case reflect.Bool:
		if v.Bool() {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		writeNumber(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		writeNumber(v.Uint())
	case reflect.Float32, reflect.Float64:
		writeNumber(math.Float64bits(v.Float()))
	default:
		panic("unsupported spread definition field: " + v.Kind().String())
	}
}
