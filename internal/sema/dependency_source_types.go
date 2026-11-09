package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// dependencySourceVisibilityCode carries "Type is not visible" for a package
// type that exists in loaded dependency source but not for a subscriber.
const dependencySourceVisibilityCode = "GLADESEMA035"

// noteDependencySourceNamespace records a declared dependency whose complete
// package source Glade loaded. The loaded source is then the package's type
// inventory for a subscriber (native rows:
// testdata/conformance/namespace_subscriber_members/org.tsv). Artifact-backed
// and opaque dependencies stay open, as do the project's own namespaces.
func (a *Analyzer) noteDependencySourceNamespace(index typesys.Index, dep typesys.DependencyInfo) {
	if dep.Status != "loaded" || dep.SourceRoot == "" {
		return
	}
	namespace := normalizeName(dep.Namespace)
	if strings.EqualFold(dep.Namespace, a.namespace) || a.sourceNamespaces[namespace] || semaDependencyBackedByArtifact(index, dep.Namespace) {
		return
	}
	if a.depSourceNamespaces == nil {
		a.depSourceNamespaces = make(map[string]bool)
	}
	a.depSourceNamespaces[namespace] = true
}

// dependencySourceTypeRejection reports the native diagnostic for a type
// spelled through a source-backed dependency namespace when the package does
// not offer it to a subscriber. Captured subscriber rows (S02-S04, S13-S15,
// S17, S20): a type absent from the package, including one the subscriber
// itself declares under the same simple name, is "Invalid type"; a
// non-global nested type of a global outer is "Type is not visible".
// Public or private top-level package classes are not claimed: no row
// covers them.
func (a *Analyzer) dependencySourceTypeRejection(model *semaTypeMemberView, spelling string) (message, code string, rejected bool) {
	if a == nil || len(a.depSourceNamespaces) == 0 || model == nil {
		return "", "", false
	}
	spelling = strings.TrimSpace(spelling)
	parts := strings.Split(spelling, ".")
	if len(parts) < 2 || !a.depSourceNamespaces[normalizeName(parts[0])] {
		return "", "", false
	}
	for _, part := range parts {
		if part == "" || strings.ContainsAny(part, "<>[]() ,") {
			return "", "", false
		}
	}
	for _, part := range parts[1:] {
		// Custom objects, fields and metadata spell package members with
		// a double underscore; they are not Apex type paths.
		if strings.Contains(part, "__") {
			return "", "", false
		}
	}
	for end := 2; end <= len(parts); end++ {
		members, known := model.lookupName(strings.Join(parts[:end], "."))
		if !known || members.platform || !members.dependency || !strings.EqualFold(members.namespace, parts[0]) {
			// A package-qualified name never binds a subscriber class
			// (S15, S17). A type the loaded source does not declare stays
			// open: Glade has no signal that the source is the whole package.
			if end > 2 {
				// The outer is in the loaded source and the nested type is not:
				// a partial or older copy cannot prove absence.
				return "", "", false
			}
			if local, found := model.lookupName(strings.Join(parts[1:end], ".")); found && !local.dependency && !local.platform && !local.sobject {
				return "Invalid type: " + spelling, dependencySourceVisibilityCode, true
			}
			return "", "", false
		}
		global := hasModifier(members.modifiers, "global")
		if end == 2 {
			if !global {
				return "", "", false
			}
			continue
		}
		if !global {
			return "Type is not visible: " + spelling, dependencySourceVisibilityCode, true
		}
	}
	return "", "", false
}

func semaNativeDiagnosticText(d diagnostic.Diagnostic) string {
	if d.NativeMessage != "" {
		return d.NativeMessage
	}
	return d.Message
}

// dependencySourceTypeDiagnosticAuthoritative keeps a package-type rejection
// an error when a source-backed dependency would downgrade it. The loaded
// source is the package inventory for the spellings it covers: a spelling
// qualified by the namespace, and an unqualified spelling of a package type
// (native: "Invalid type: Outer.Inner" from a subscriber).
func dependencySourceTypeDiagnosticAuthoritative(index typesys.Index, d diagnostic.Diagnostic) bool {
	if d.Code != "GLADESEMA006" && d.Code != dependencySourceVisibilityCode {
		return false
	}
	// Only the diagnostics this file emits (code 035) stay errors when spelled
	// through a loaded-source namespace; older ones are still downgraded.
	text := semaNativeDiagnosticText(d)
	invalid := strings.HasPrefix(text, "Invalid type: ")
	spelling, found := strings.CutPrefix(text, "Invalid type: ")
	if !found {
		spelling, found = strings.CutPrefix(text, "Type is not visible: ")
	}
	if !found {
		return false
	}
	root, _, qualified := strings.Cut(strings.TrimSpace(spelling), ".")
	if !qualified {
		return false
	}
	for _, dep := range index.Dependencies {
		if d.Code == dependencySourceVisibilityCode && dep.Status == "loaded" && dep.SourceRoot != "" && strings.EqualFold(dep.Namespace, root) &&
			!strings.EqualFold(root, index.Project.Namespace) && !semaDependencyBackedByArtifact(index, dep.Namespace) {
			return true
		}
	}
	if !invalid || d.Code != "GLADESEMA006" {
		return false
	}
	packageType := false
	for _, typ := range index.Types {
		if typ.NestingDepth != 0 || !strings.EqualFold(typ.Name, root) {
			continue
		}
		if !typ.Dependency {
			return false
		}
		if typ.Namespace != "" && !typ.Artifact {
			packageType = true
		}
	}
	return packageType
}
