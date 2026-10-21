package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// CD-076's static-final and private cross-owner twins retain their rejections.
// Supply the measured native text without widening constructor admission.
func capturedInheritedFinalDiagnostics(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	var model *semaTypeMemberView
	var sources *semaSources
	protectedStatics := make(map[string]bool)
	var out []diagnostic.Diagnostic
	for _, d := range diagnostics {
		if d.NativeMessage != "" || d.Range == nil || d.Code != "GLADESEMA010" && d.Code != "GLADESEMA027" {
			out = append(out, d)
			continue
		}
		var typ typesys.TypeSymbol
		for _, candidate := range index.Types {
			if candidate.File == d.File && candidate.Range.Start.Offset <= d.Range.Start.Offset && candidate.Range.End.Offset >= d.Range.End.Offset &&
				(typ.Name == "" || candidate.NestingDepth > typ.NestingDepth) {
				typ = candidate
			}
		}
		var constructor typesys.MemberSymbol
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationConstructor && len(member.Parameters) == 0 &&
				member.Range.Start.Offset <= d.Range.Start.Offset && member.Range.End.Offset >= d.Range.End.Offset {
				constructor = member
				break
			}
		}
		if constructor.Name == "" || typ.Dependency || typ.Namespace != "" || !typeUsesAPIVersionAtLeast(typ, 62) ||
			hasModifier(typ.Modifiers, vm.AnonymousClassModifier) || !hasModifier(typ.Modifiers, "public") {
			out = append(out, d)
			continue
		}
		if model == nil {
			model = buildSemaTypeMemberView(index)
			sources = newSemaSources(nil, nil)
		}
		source, ok := sources.rawForType(typ)
		if !ok || d.Range.Start.Offset < 0 || d.Range.End.Offset <= d.Range.Start.Offset || d.Range.End.Offset > len(source) ||
			!strings.HasPrefix(strings.TrimSpace(source[d.Range.End.Offset:]), "=") {
			out = append(out, d)
			continue
		}
		target := strings.TrimSpace(source[d.Range.Start.Offset:d.Range.End.Offset])
		fieldName := shortNestedTypeName(target)
		field, found := semaResolveField(model, typ.Name, fieldName, make(map[string]bool))
		if prefix, _, qualified := strings.Cut(target, "."); qualified && !strings.EqualFold(prefix, "this") && !strings.EqualFold(prefix, "super") {
			if at := strings.LastIndexByte(target, '.'); at > 0 {
				field, found = semaResolveField(model, resolveNestedTypeName(model, typ.Name, target[:at]), fieldName, make(map[string]bool))
			}
		}
		outer := typ.OwnerName
		if outer == "" {
			outer = strings.Split(field.owner, ".")[0]
		}
		base, _, baseOK := semaLookupTypeMembers(model, field.owner)
		if !found || !baseOK || base.dependency || base.platform || field.member.Kind != apexast.DeclarationField ||
			!hasModifier(field.member.Modifiers, "final") || !hasEitherModifier(field.member.Modifiers, "private", "protected") ||
			!capturedInheritedFinalType(model, field.owner, outer, field.member.Type) {
			out = append(out, d)
			continue
		}
		parent := resolveNestedTypeName(model, typ.Name, typ.SuperClass)
		directBase := hasModifier(base.modifiers, "virtual") && strings.EqualFold(parent, field.owner)
		outerStatic := typ.NestingDepth == 1 && strings.EqualFold(typ.OwnerName, field.owner)
		static := hasModifier(field.member.Modifiers, "static")
		if static && (directBase && typ.NestingDepth == 0 && base.nestingDepth == 0 || outerStatic) {
			d.NativeMessage = "Final members can only be assigned in their declaration, init blocks, or constructors: " + field.member.Name
			if hasModifier(field.member.Modifiers, "protected") && !protectedStatics[field.owner+"."+field.member.Name] {
				for _, owner := range index.Types {
					if strings.EqualFold(owner.Name, field.owner) {
						declaration := declarationContractDiagnostic(owner, field.member.Range, "protected fields cannot be static")
						declaration.NativeMessage = declaration.Message
						out = append(out, declaration)
						protectedStatics[field.owner+"."+field.member.Name] = true
						break
					}
				}
			}
		} else if !static && directBase && typ.NestingDepth == 0 && base.nestingDepth <= 1 &&
			hasModifier(field.member.Modifiers, "private") && !semaSameTypeFamily(typ.Name, field.owner) {
			d.NativeMessage = "Variable is not visible: " + field.owner + "." + field.member.Name
		}
		out = append(out, d)
	}
	return capturedInheritedStaticFinalWrites(index, out)
}

// CD-076 STATIC and PROTECTED_STATIC_BASE contain one class-qualified static
// final write in a sibling or direct-base constructor. Check that assignment
// even when earlier body analysis produced no diagnostic (for example x=7).
func capturedInheritedStaticFinalWrites(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	var model *semaTypeMemberView
	var sources *semaSources
	appendUnique := func(d diagnostic.Diagnostic) {
		for _, existing := range diagnostics {
			sameRange := existing.Range == nil && d.Range == nil || existing.Range != nil && d.Range != nil && *existing.Range == *d.Range
			if existing.File == d.File && existing.NativeMessage == d.NativeMessage && sameRange {
				return
			}
		}
		diagnostics = append(diagnostics, d)
	}
	for _, typ := range index.Types {
		if typ.Dependency || typ.Namespace != "" || typ.Kind != apexast.DeclarationClass || !hasModifier(typ.Modifiers, "public") ||
			hasModifier(typ.Modifiers, "abstract") || hasModifier(typ.Modifiers, "virtual") ||
			hasModifier(typ.Modifiers, vm.AnonymousClassModifier) || typ.NestingDepth > 1 || !typeUsesAPIVersionAtLeast(typ, 62) {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationConstructor || len(member.Parameters) != 0 || !hasModifier(member.Modifiers, "public") {
				continue
			}
			if model == nil {
				model = buildSemaTypeMemberView(index)
				sources = newSemaSources(nil, nil)
			}
			source, ok := sources.rawForType(typ)
			if !ok {
				continue
			}
			body, offset, ok := semaBodyFromRange(source, member.BodyRange)
			if !ok {
				continue
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: typ.EffectiveAPIVersion})
			if err != nil || len(program.Instructions) != 1 || program.Instructions[0].Op != ir.OpAssign {
				continue
			}
			inst := program.Instructions[0]
			at := strings.LastIndexByte(inst.Name, '.')
			if at <= 0 || strings.EqualFold(inst.Name[:at], "this") || strings.EqualFold(inst.Name[:at], "super") {
				continue
			}
			ownerName := resolveNestedTypeName(model, typ.Name, inst.Name[:at])
			field, found := semaResolveField(model, ownerName, inst.Name[at+1:], make(map[string]bool))
			owner, _, ownerOK := semaLookupTypeMembers(model, field.owner)
			outer := typ.OwnerName
			if outer == "" {
				outer = field.owner
			}
			if !found || !ownerOK || !strings.EqualFold(ownerName, field.owner) || owner.dependency || owner.platform || owner.namespace != "" ||
				field.member.Kind != apexast.DeclarationField || !hasModifier(field.member.Modifiers, "static") ||
				!hasModifier(field.member.Modifiers, "final") || !hasEitherModifier(field.member.Modifiers, "private", "protected") ||
				!capturedInheritedFinalType(model, field.owner, outer, field.member.Type) {
				continue
			}
			direct := typ.NestingDepth == 0 && owner.nestingDepth == 0 && hasModifier(owner.modifiers, "virtual") &&
				strings.EqualFold(resolveNestedTypeName(model, typ.Name, typ.SuperClass), field.owner)
			sibling := typ.NestingDepth == 1 && strings.EqualFold(typ.OwnerName, field.owner)
			if sibling {
				base, _, found := semaLookupTypeMembers(model, resolveNestedTypeName(model, typ.Name, typ.SuperClass))
				sibling = found && base.nestingDepth == 1 && hasModifier(base.modifiers, "virtual") &&
					strings.EqualFold(base.name, typ.OwnerName+"."+shortNestedTypeName(base.name))
			}
			if !direct && !sibling {
				continue
			}
			var declaring typesys.TypeSymbol
			for _, candidate := range index.Types {
				if strings.EqualFold(candidate.Name, field.owner) {
					declaring = candidate
					break
				}
			}
			ownerSource, ok := sources.rawForType(declaring)
			if !ok || !semaStaticFinalFieldHasDeclarationInitializer(field.member, ownerSource) {
				continue
			}
			if hasModifier(field.member.Modifiers, "protected") {
				d := declarationContractDiagnostic(declaring, field.member.Range, "protected fields cannot be static")
				d.NativeMessage = d.Message
				appendUnique(d)
			}
			message := "Final members can only be assigned in their declaration, init blocks, or constructors: " + field.member.Name
			appendUnique(diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA027", Message: message,
				NativeMessage: message, File: typ.File, Range: semaRange(source, offset+inst.Pos, offset+inst.Pos+len(inst.Name))})
		}
	}
	return diagnostics
}
