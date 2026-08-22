package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaPlatformTypeUnavailable(version, typeName string) bool {
	for _, key := range semaPlatformTypeKeys(typeName) {
		if semaPlatformExplicitlyUnsupported("apex:" + key) {
			return true
		}
		if rng, ok := generatedPlatformTypeAvailability[key]; ok {
			return !semaVersionAllows(version, rng)
		}
	}
	return false
}

func semaPlatformMemberUnavailable(version, receiver, member string) bool {
	member = normalizeName(member)
	for _, owner := range semaPlatformTypeKeys(receiver) {
		if rng, ok := generatedPlatformMemberAvailability[owner+"."+member]; ok {
			return !semaVersionAllows(version, rng)
		}
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
	resolved, err := apexversion.ResolveSource(version)
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
		if _, err := apexversion.ResolveSource(typ.EffectiveAPIVersion); err != nil {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_VERSION", Message: fmt.Sprintf("%s %q: %v", typ.Kind, typ.Name, err), File: typ.File, Range: &typ.Range})
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.Dependency || !trigger.HasSourceSnapshot() {
			continue
		}
		if _, err := apexversion.ResolveSource(trigger.EffectiveAPIVersion); err != nil {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_VERSION", Message: fmt.Sprintf("trigger %q: %v", trigger.Name, err), File: trigger.File, Range: &trigger.Range})
		}
	}
	return diagnostics
}
