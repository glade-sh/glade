package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// A release introduction date does not always gate source type references.
// C032 and N003 declare this type in anonymous and named Apex at API 62 and 67.
// Captured member exceptions are listed below; other members retain their
// generated availability ranges.
var observedPlatformSourceTypeAvailability = map[string]apexversion.Range{
	"database.paginationcursor": {Since: 62},
	// Messaging C019/R261/R262, bisected at API 64/65/66.
	"richmessaging.processformhandler": {Since: 65},
	"richmessaging.catalogsection":     {Since: 66},
}

// Observed visibility failures do not establish release introduction dates.
// commerce_extension_shapes/org.tsv:31-32,46-47,92-93,107-108 captures these
// declaration and anonymous rejections in a Commerce-enabled scratch org.
// Leave unprobed source versions and other names in the partial catalog alone.
var observedPlatformSourceTypeRejections = map[string]map[string]bool{
	"commercestorepricing.contractresolutionrequest":  {"62.0": true, "67.0": true},
	"commercestorepricing.contractresolutionresponse": {"62.0": true, "67.0": true},
}

// Messaging C019 compiles this callback at API 65 as well as 66/67.
var observedPlatformSourceMemberAvailability = map[string]apexversion.Range{
	"richmessaging.processformhandler.processformrequest": {Since: 65},
	// Both cursor counter getters are admitted at API 62 and 67.
	"system.limits.getapexcursors":      {Since: 62},
	"system.limits.getlimitapexcursors": {Since: 62},
	// credential_mock_visibility captures these references behind mock branches
	// in named and anonymous Apex at source APIs 62, 65, 66 and 67. Their release
	// introduction does not gate source visibility; hosted execution stays separate.
	"connectapi.namedcredentials.updatenamedcredential":    {Since: 62},
	"connectapi.namedcredentials.deletenamedcredential":    {Since: 62},
	"connectapi.namedcredentials.updateexternalcredential": {Since: 62},
	"connectapi.namedcredentials.deleteexternalcredential": {Since: 62},
}

var observedPlatformSourceExactAvailability = map[string]apexversion.Range{
	"apex:richmessaging.processformhandler.processformrequest(richmessaging.processformresponse)": {Since: 65},
	// stdlib_locale_crypto_c14 location Address parked (API 62.0): the authenticated-data overload ran natively although the docs date it to 64.
	"apex:system.crypto.encryptwithmanagediv(string,blob,blob,blob)":                                       {Since: 62},
	"apex:system.limits.getapexcursors()":                                                                  {Since: 62},
	"apex:system.limits.getlimitapexcursors()":                                                             {Since: 62},
	"apex:connectapi.namedcredentials.updatenamedcredential(string,connectapi.namedcredentialinput)":       {Since: 62},
	"apex:connectapi.namedcredentials.deletenamedcredential(string)":                                       {Since: 62},
	"apex:connectapi.namedcredentials.updateexternalcredential(string,connectapi.externalcredentialinput)": {Since: 62},
	"apex:connectapi.namedcredentials.deleteexternalcredential(string)":                                    {Since: 62},
}

func semaPlatformTypeUnavailable(version, typeName string) bool {
	return semaPlatformTypeUnavailableInModel(version, typeName, nil)
}

func semaPlatformTypeUnavailableInModel(version, typeName string, model *semaTypeMemberView) bool {
	for _, key := range semaPlatformTypeKeys(typeName) {
		if semaPlatformExplicitlyUnsupported("apex:" + key) {
			return true
		}
		if versions, ok := observedPlatformSourceTypeRejections[key]; ok {
			if resolved, err := apexversion.PreserveSource(version); err == nil && (versions[resolved] || resolved == "68.0" && semaCommercePricingDisabled(model)) {
				// A declared source or dependency type can have the same
				// qualified name. Only skip this platform observation; ordinary
				// source constructor and method checks still apply.
				sourceOwned := false
				if model != nil {
					members, _, found := semaLookupTypeMembers(model, typeName)
					sourceOwned = found && !members.platform && !members.sobject
				}
				if !sourceOwned {
					return true
				}
			}
		}
		if rng, ok := observedPlatformSourceTypeAvailability[key]; ok {
			return !semaVersionAllows(version, rng)
		}
		if rng, ok := generatedPlatformTypeAvailability[key]; ok {
			return !semaVersionAllows(version, rng)
		}
	}
	return false
}

// Commerce-enabled API 68 orgs expose the contract types only when contract
// pricing is enabled. An unspecified org shape retains the existing open-world
// behavior; it is not evidence that the setting is disabled.
func semaCommercePricingDisabled(model *semaTypeMemberView) bool {
	if model == nil || model.state == nil || model.state.base == nil {
		return false
	}
	commerce, pricing := false, false
	for _, feature := range model.state.base.orgShapeFeatures {
		if strings.EqualFold(feature, "CommerceEnabled") {
			commerce = true
		}
		if strings.EqualFold(feature, "CommerceContractPricing") {
			pricing = true
		}
	}
	return commerce && !pricing
}

func semaPlatformMemberUnavailable(version, receiver, member string) bool {
	member = normalizeName(member)
	for _, owner := range semaPlatformTypeKeys(receiver) {
		if rng, ok := observedPlatformSourceMemberAvailability[owner+"."+member]; ok {
			return !semaVersionAllows(version, rng)
		}
		if rng, ok := typesys.ConnectAPIMemberAvailability(owner, member); ok {
			return !semaVersionAllows(version, rng)
		}
		if rng, ok := generatedPlatformMemberAvailability[owner+"."+member]; ok {
			return !semaVersionAllows(version, rng)
		}
	}
	return false
}

// Captured ConnectApi member ranges belong to the declaring class, including
// inherited properties (capi2_assigned_outputs/org.tsv).
func semaConnectAPIInheritedFieldUnavailable(version, receiver, member string, model *semaTypeMemberView) bool {
	if len(receiver) < len("ConnectApi.") || !strings.EqualFold(receiver[:len("ConnectApi.")], "ConnectApi.") || model == nil {
		return false
	}
	seen := map[string]bool{}
	for current := receiver; current != ""; {
		key := strings.ToLower(current)
		if seen[key] {
			break
		}
		seen[key] = true
		if semaPlatformMemberUnavailable(version, current, member) {
			return true
		}
		members, _, ok := semaLookupTypeMembers(model, current)
		if !ok || !members.platform {
			break
		}
		current = members.superClass
	}
	return false
}

func semaPlatformFieldPathUnavailable(version, path string) bool {
	receiver, member, ok := strings.Cut(strings.TrimSpace(path), ".")
	if !ok {
		return false
	}
	if dot := strings.LastIndexByte(path, '.'); dot > 0 {
		receiver, member = path[:dot], path[dot+1:]
	}
	return semaPlatformMemberUnavailable(version, receiver, member)
}

func semaPlatformExactUnavailable(version, surfaceID string) bool {
	if semaPlatformExplicitlyUnsupported(surfaceID) {
		return true
	}
	if rng, ok := observedPlatformSourceExactAvailability[normalizeName(surfaceID)]; ok {
		return !semaVersionAllows(version, rng)
	}
	if rng, ok := typesys.ConnectAPIExactAvailability(normalizeName(surfaceID)); ok {
		return !semaVersionAllows(version, rng)
	}
	if rng, ok := generatedPlatformExactAvailability[normalizeName(surfaceID)]; ok {
		return !semaVersionAllows(version, rng)
	}
	return false
}

func semaPlatformExplicitlyUnsupported(surfaceID string) bool {
	surfaceID = normalizeName(surfaceID)
	for _, unsupported := range generatedUnsupportedPlatformSurfaceIDs {
		if surfaceID == unsupported {
			return true
		}
	}
	return false
}

func semaPlatformResolvedMemberUnavailable(version string, candidate resolvedMember) bool {
	params := make([]string, len(candidate.member.Parameters))
	for i, parameter := range candidate.member.Parameters {
		params[i] = strings.ReplaceAll(normalizeName(semaCanonicalPlatformAlias(parameter.Type)), " ", "")
	}
	member := normalizeName(candidate.member.Name) + "(" + strings.Join(params, ",") + ")"
	for _, owner := range semaPlatformTypeKeys(candidate.owner) {
		if semaPlatformExactUnavailable(version, "apex:"+owner+"."+member) {
			return true
		}
	}
	return false
}

func semaPlatformTypeKeys(typeName string) []string {
	base, _ := semaGenericBaseAndArgs(semaCanonicalPlatformAlias(typeName))
	base = normalizeName(base)
	keys := []string{base}
	if !strings.Contains(base, ".") {
		keys = append(keys, "system."+base)
	} else if strings.HasPrefix(base, "system.") {
		keys = append(keys, strings.TrimPrefix(base, "system."))
	}
	return keys
}

func semaVersionAllows(version string, rng apexversion.Range) bool {
	resolved, err := apexversion.PreserveSource(version)
	return err == nil && rng.Allows(resolved)
}

func (a *Analyzer) hasKnownAtVersion(name, version string) bool {
	if !a.hasKnown(name) {
		return false
	}
	if reference, ok := a.known[a.canonicalName(name)]; ok && reference.Kind != TypePlatform {
		return true
	}
	return !semaPlatformTypeUnavailable(version, name)
}

func sourceAPIVersionDiagnostics(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Dependency || !typ.HasSourceSnapshot() {
			continue
		}
		if _, err := apexversion.PreserveSource(typ.EffectiveAPIVersion); err != nil {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_VERSION", NativeMessage: "Invalid api version:" + typ.EffectiveAPIVersion, Message: fmt.Sprintf("%s %q: %v", typ.Kind, typ.Name, err), File: typ.File, Range: &typ.Range})
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.Dependency || !trigger.HasSourceSnapshot() {
			continue
		}
		if _, err := apexversion.PreserveSource(trigger.EffectiveAPIVersion); err != nil {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_VERSION", NativeMessage: "Invalid api version:" + trigger.EffectiveAPIVersion, Message: fmt.Sprintf("trigger %q: %v", trigger.Name, err), File: trigger.File, Range: &trigger.Range})
		}
	}
	return diagnostics
}

// semaObjectPermissionsViewAllFieldsHidden reports ObjectPermissions.
// PermissionsViewAllFields at source API 62.x. Native case289_followups ST1-ST4
// (anonymous, static) reject the member at 62.0 and compile it at 67.0; the
// describe map and dynamic routes expose it from 63 (OPV rows). The storage
// catalog keeps the field; only the source API gates it.
func semaObjectPermissionsViewAllFieldsHidden(version, object, field string) bool {
	if !strings.EqualFold(strings.TrimSpace(field), "PermissionsViewAllFields") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(object), "ObjectPermissions") && !strings.EqualFold(strings.TrimSpace(object), "Schema.ObjectPermissions") {
		return false
	}
	return apexversion.AtLeast(version, 62) && !apexversion.AtLeast(version, 63)
}

// semaConnectAPICallUnavailable applies the captured per-overload ConnectApi
// boundaries ("Method not added until", "Method was removed after") when every
// platform overload the arguments can select is outside the running version.
func semaConnectAPICallUnavailable(version, receiver, method string, argTypes []string, model *semaTypeMemberView) bool {
	if !strings.HasPrefix(normalizeName(receiver), "connectapi.") || model == nil {
		return false
	}
	matched := false
	for _, candidate := range resolveMemberMethods(model, receiver, method) {
		params := make([]string, len(candidate.member.Parameters))
		for i, parameter := range candidate.member.Parameters {
			params[i] = parameter.Type
		}
		if !semaArgsMatch(params, argTypes, model) {
			continue
		}
		matched = true
		keyParams := make([]string, len(params))
		for i, param := range params {
			keyParams[i] = strings.ReplaceAll(normalizeName(semaCanonicalPlatformAlias(param)), " ", "")
		}
		rng, ok := typesys.ConnectAPIExactAvailability("apex:" + normalizeName(receiver) + "." + normalizeName(method) + "(" + strings.Join(keyParams, ",") + ")")
		if !ok || semaVersionAllows(version, rng) {
			return false
		}
	}
	return matched
}
