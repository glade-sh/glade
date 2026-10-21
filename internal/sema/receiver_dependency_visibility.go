package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// Resolved subscriber access rules agree across the supported source API range.
func receiverDependencyRejectionAPI(typ typesys.TypeSymbol) bool {
	major, known := apexversion.Major(typ.EffectiveAPIVersion)
	return !typ.Dependency && typ.Namespace == "" && known && major >= 62 && major <= 68
}

// Index peers once, rather than scanning the project on every hot-path call.
// Qualified source package declarations remain distinct from local declarations.
func markReceiverDependencyPeers(members map[string]typeMembers, index typesys.Index) bool {
	namespaces := make(map[string]bool)
	for _, dep := range index.Dependencies {
		if dep.Status == "loaded" && dep.SourceRoot != "" && !semaDependencyBackedByArtifact(index, dep.Namespace) {
			namespaces[normalizeName(dep.Namespace)] = true
		}
	}
	installed := false
	for _, foreign := range members {
		if !foreign.dependency || !foreign.receiverDependencySource || foreign.namespace == "" || !namespaces[normalizeName(foreign.namespace)] || foreign.nestingDepth != 0 || foreign.kind != apexast.DeclarationClass {
			continue
		}
		installed = true
		prefix := foreign.namespace + "."
		if !strings.HasPrefix(strings.ToLower(foreign.name), strings.ToLower(prefix)) {
			continue
		}
		key := normalizeName(foreign.name[len(prefix):])
		local, ok := members[key]
		if !ok || local.dependency || local.namespace != "" || !local.receiverDependencySource || local.nestingDepth != 0 || local.kind != apexast.DeclarationClass {
			continue
		}
		duplicate := false
		for _, peer := range local.receiverDependencyPeers {
			if strings.EqualFold(peer, foreign.name) {
				duplicate = true
			}
		}
		if !duplicate {
			local.receiverDependencyPeers = append(local.receiverDependencyPeers, foreign.name)
		}
		members[key] = local
	}
	return installed
}

func receiverDependencyPeers(model *semaTypeMemberView, receiver string) []string {
	local, ok := model.lookupName(receiver)
	if !ok || local.dependency || local.namespace != "" || local.nestingDepth != 0 || !local.receiverDependencySource || model.state == nil || model.state.base == nil {
		return nil
	}
	// File overlays preserve their own methods; the immutable base supplies only
	// namespace identity peers, never another file's declarations.
	base, ok := model.state.base.lookup(normalizeName(local.name))
	if !ok {
		return nil
	}
	return base.receiverDependencyPeers
}

func receiverDependencyCallPeers(typ typesys.TypeSymbol, receiver, callee string, model *semaTypeMemberView) []string {
	if peers := receiverDependencyPeers(model, receiver); len(peers) != 0 {
		return peers
	}
	// R29 selects the actual qualified package base through super; its local
	// subclass, rather than the package receiver itself, supplies the collision.
	root, _, qualified := strings.Cut(callee, ".")
	if !qualified || !strings.EqualFold(root, "super") {
		return nil
	}
	local, ok := model.lookupName(typ.Name)
	if !ok || !strings.EqualFold(local.superClass, receiver) {
		return nil
	}
	return receiverDependencyPeers(model, typ.Name)
}

func receiverDependencyCompleteChain(model *semaTypeMemberView, receiver string) bool {
	seen := map[string]bool{}
	for receiver != "" {
		members, ok := model.lookupName(receiver)
		if !ok || !members.receiverDependencySource || members.platform || members.kind != apexast.DeclarationClass || len(members.interfaces) != 0 || seen[normalizeName(members.name)] {
			return false
		}
		seen[normalizeName(members.name)] = true
		receiver = members.superClass
	}
	return true
}

// R06/R15/R16/R21/R24/R26-R30/R32/R35: a fully resolved collision
// surface gives a definitive member/access answer, not dependency uncertainty.
func receiverDependencyCallDiagnostic(typ typesys.TypeSymbol, context typesys.MemberSymbol, receiver, method, callee, mode string, args []string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if !receiverDependencyRejectionAPI(typ) || semaArgTypesContainUnknown(args) {
		return diagnostic.Diagnostic{}, false
	}
	peers := receiverDependencyCallPeers(typ, receiver, callee, model)
	if len(peers) == 0 || !receiverDependencyCompleteChain(model, receiver) {
		return diagnostic.Diagnostic{}, false
	}
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiver, method), mode)
	if mode == "implicit" {
		candidates = resolveImplicitMemberMethods(model, receiver, method)
	}
	if len(candidates) == 0 {
		if mode != "instance" || len(args) != 0 {
			return diagnostic.Diagnostic{}, false
		}
		// R06/R24 reject borrowing a method from a same-named package type.
		// Unrelated unresolved calls retain their existing permissive handling.
		foreignMatch := false
		for _, peer := range peers {
			if _, ok, ambiguous := bestResolvedMemberByArgTypes(resolveMemberMethods(model, peer, method), args, model); ok && !ambiguous {
				foreignMatch = true
			}
		}
		if !foreignMatch {
			return diagnostic.Diagnostic{}, false
		}
		return receiverDependencyDiagnostic(typ, fmt.Sprintf("method %q is not declared by receiver type %q", method, receiver), start, end, source), true
	}
	// Captured inaccessible surfaces have one declaration. Preserve the
	// existing accessible-alternate handling for uncaptured overload sets.
	if len(candidates) != 1 {
		return diagnostic.Diagnostic{}, false
	}
	target, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, args, model)
	if !ok || ambiguous {
		return diagnostic.Diagnostic{}, false
	}
	owner, known := model.lookupName(target.owner)
	if !known || !owner.receiverDependencySource {
		return diagnostic.Diagnostic{}, false
	}
	// R26-R30: protected package members do not cross the subscriber boundary.
	if owner.dependency && owner.namespace != "" && owner.namespace != typ.Namespace && accessModifier(target.member.Modifiers) == "protected" {
		return receiverDependencyDiagnostic(typ, fmt.Sprintf("protected method %q is not visible across namespace %q", target.owner+"."+method, owner.namespace), start, end, source), true
	}
	if d, blocked := checkSemaMemberAccess(typ, context, callee, target, start, end, source, model); blocked {
		d.Code = "receiver_expression_access"
		return d, true
	}
	return diagnostic.Diagnostic{}, false
}

func receiverDependencyDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	// Distinct from the prefixes reserved for incomplete dependency diagnostics.
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "receiver_expression_access", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}
