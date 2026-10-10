package sema

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var (
	lifecycleUnknownLocal   = regexp.MustCompile(`declares local "[^"]+" with unknown type "([^"]+)"$`)
	lifecycleEnumLocal      = regexp.MustCompile(`initializes (.+) local "[^"]+" with (.+)$`)
	lifecycleConstructType  = regexp.MustCompile(`constructs non-instantiable (class|interface|enum) "([^"]+)"`)
	lifecycleDuplicateField = regexp.MustCompile(`declares duplicate (?:field|property|property/field|field/property) "([^"]+)"`)
	lifecycleNonVirtualBase = regexp.MustCompile(`cannot extend non-virtual, non-abstract class "([^"]+)"`)
	lifecycleGetterReturn   = regexp.MustCompile(`method "[^"]+\.get" has invalid return: returns (.+) from (.+) method`)
)

// NativeLifecycleDiagnostics supplies measured lifecycle diagnostics to ordinary
// analysis and anonymous requests. Measured constructor, member-type,
// conversion, valueOf and map-value failures also publish native text as Message;
// other diagnostics retain their local explanation for the editor.
func NativeLifecycleDiagnostics(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	out := append([]diagnostic.Diagnostic(nil), diagnostics...)
	var exceptionSources *semaSources
	lookup := func(name string) (typesys.TypeSymbol, bool) {
		for _, typ := range index.Types {
			if strings.EqualFold(typ.Name, name) {
				return typ, true
			}
		}
		return typesys.TypeSymbol{}, false
	}
	for i := range out {
		d := &out[i]
		if d.NativeMessage != "" {
			if d.Code == "GLADESEMA018" && strings.HasPrefix(d.NativeMessage, "Illegal assignment from ") && !valueCollectionAssignmentNativeText(d.NativeMessage) ||
				d.Code == "GLADESEMA002" && strings.HasPrefix(d.NativeMessage, "Invalid type: ") ||
				d.Code == "GLADESEMA023" && strings.HasPrefix(d.NativeMessage, "Method does not exist or incorrect signature: void valueOf(") && strings.HasSuffix(d.NativeMessage, " from the type String") ||
				d.Code == "GLADESEMA025" && strings.HasPrefix(d.NativeMessage, "Invalid value type ") && strings.HasSuffix(d.NativeMessage, " for Object") {
				d.Message = d.NativeMessage
			}
			continue
		}
		if d.Severity != diagnostic.Error {
			continue
		}
		// Preserve the parser's diagnosis
		// and expose the captured exception text at the compile boundary.
		if d.Code == "APEXPARSE001" {
			if exceptionSources == nil {
				exceptionSources = newSemaSources(nil, nil)
			}
			// A blocking parser error leaves no type symbols for its file.
			// The build still retains the raw source digest, so bind the
			// fallback read to that generation before refining its diagnosis.
			typ := typesys.TypeSymbol{File: d.File}
			for _, candidate := range index.Types {
				if candidate.File == d.File && candidate.HasSourceSnapshot() {
					typ = candidate
					break
				}
			}
			if source, ok := exceptionSources.rawForType(typ); ok {
				digest, bound := index.SourceDigest(d.File)
				if bound && sha256.Sum256([]byte(source)) == digest {
					if syntax := vm.ExceptionSyntaxError(source); syntax != nil && d.Range != nil {
						end := max(syntax.Offset, syntax.EndOffset)
						if d.Range.Start.Offset <= end && syntax.Offset <= d.Range.End.Offset {
							d.NativeMessage = exceptionSyntaxMessage(syntax.NativeMessage, hasModifier(typ.Modifiers, vm.AnonymousClassModifier))
							d.Range = semaRange(source, syntax.Offset, syntax.Offset+1)
						}
					}
				}
			}
			if d.NativeMessage != "" {
				continue
			}
		}
		var owner typesys.TypeSymbol
		for _, typ := range index.Types {
			if typ.File == d.File && d.Range != nil && typ.Range.Start.Offset <= d.Range.Start.Offset && typ.Range.End.Offset >= d.Range.Start.Offset && (owner.Name == "" || typ.NestingDepth > owner.NestingDepth) {
				owner = typ
			}
		}
		// Native inner-type depth errors precede reserved type names. The name
		// remains reserved in every legal declaration context (Q001-Q004).
		if d.Code == "APEXPARSE002" && strings.HasPrefix(d.Message, "Identifier name is reserved:") {
			for _, typ := range index.Types {
				depth := typ.NestingDepth
				if hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
					depth++
				}
				if typ.File == d.File && depth > 1 {
					d.NativeMessage = "Inner types are not allowed to have inner types"
					line := typ.Range.Start.Line
					d.NativeLine = &line
					break
				}
			}
		}
		switch d.Code {
		case "GLADESEMA006":
			// N010/N011 retain the spelling of an unknown qualified local
			// even when the text scanner reports it before IR diagnostics.
			if match := lifecycleUnknownLocal.FindStringSubmatch(d.Message); match != nil && strings.Contains(match[1], ".") {
				d.NativeMessage = "Invalid type: " + match[1]
			}
		case "GLADESEMA018":
			if match := lifecycleEnumLocal.FindStringSubmatch(d.Message); match != nil {
				native := valueCollectionAssignmentMessage(match[1], match[2], nil)
				if native != "" {
					// The named scanner runs before IR. Preserve its explanation
					// without rebuilding the type-member model for wording.
					for _, candidate := range index.Types {
						if !candidate.Dependency && candidate.NestingDepth == 0 && (strings.EqualFold(candidate.Name, match[1]) || strings.EqualFold(candidate.Name, match[2])) {
							native = ""
							break
						}
					}
					d.NativeMessage = native
				}
			}
			// Named C032/C033: the local scanner precedes the IR checker;
			// resolve the enum within its declaring context for native text.
			if match := lifecycleEnumLocal.FindStringSubmatch(d.Message); match != nil {
				parts := strings.Split(owner.Name, ".")
				for depth := len(parts); depth >= 0; depth-- {
					name := match[1]
					if depth > 0 {
						name = strings.Join(parts[:depth], ".") + "." + name
					}
					if target, ok := lookup(name); ok && target.Kind == apexast.DeclarationEnum && !target.Dependency {
						d.NativeMessage = "Illegal assignment from " + match[2] + " to " + target.Name
						d.Message = d.NativeMessage
						break
					}
				}
			}
		case "GLADESEMA015":
			if match := lifecycleConstructType.FindStringSubmatch(d.Message); match != nil {
				if match[1] == "class" {
					d.NativeMessage = "Abstract classes cannot be constructed: " + match[2]
				} else {
					d.NativeMessage = "Type cannot be constructed: " + match[2]
				}
			}
		case "GLADESEMA017":
			if match := lifecycleNonVirtualBase.FindStringSubmatch(d.Message); match != nil {
				parent := match[1]
				if owner.OwnerName != "" {
					if nested, ok := lookup(owner.OwnerName + "." + parent); ok {
						parent = nested.Name
					}
				}
				d.NativeMessage = "Non-virtual and non-abstract type cannot be extended: " + parent
			}
		case "GLADESEMA011":
			if strings.Contains(d.Message, "this(...)/super(...) must be the first statement") {
				callee := "this"
				if strings.Contains(d.Message, "invalid super(...)") {
					callee = "super"
				}
				d.NativeMessage = "Call to '" + callee + "()' must be the first statement in a constructor method"
			}
			if marker := "implicit super() requires an accessible no-argument constructor on "; strings.Contains(d.Message, marker) {
				_, parent, _ := strings.Cut(d.Message, marker)
				d.NativeMessage = "No default constructor available in super type: " + parent
			}
		case "GLADESEMA031":
			if match := lifecycleDuplicateField.FindStringSubmatch(d.Message); match != nil {
				d.NativeMessage = "Duplicate field: " + match[1]
			}
		case "GLADESEMA019":
			if match := lifecycleGetterReturn.FindStringSubmatch(d.Message); match != nil {
				d.NativeMessage = "Illegal conversion from " + match[1] + " to " + match[2]
			}
		case "GLADESEMA032":
			if strings.Contains(d.Message, "accessor visibility cannot be wider than the property") {
				for _, member := range owner.Members {
					if member.Kind == apexast.DeclarationProperty && accessModifier(member.Modifiers) == "private" {
						for _, accessor := range member.Accessors {
							if accessModifier(accessor.Modifiers) == "public" {
								d.NativeMessage = "Cannot declare public accessor on private property"
							}
						}
					}
				}
			}
			if strings.Contains(d.Message, "declares duplicate getter") {
				d.NativeMessage = "Missing '}' at 'get'"
				if hasModifier(owner.Modifiers, vm.AnonymousClassModifier) {
					d.NativeMessage = "Unexpected token 'class'."
				}
			}
			if strings.Contains(d.Message, "cannot declare a static initializer") {
				d.NativeMessage = "Inner types are not allowed to have static blocks"
			}
			if strings.Contains(d.Message, "nests deeper than one inner level") {
				d.NativeMessage = "Inner types are not allowed to have inner types"
			}
			if hasModifier(owner.Modifiers, vm.AnonymousClassModifier) {
				if d.Message == "classes are by default virtual" || d.Message == "Enclosing type for global fields in apex classes must be declared as global" {
					line := 1
					d.NativeLine = &line
				}
				if d.Message == "static can only be used on fields of a top level type" {
					for _, member := range owner.Members {
						if member.Kind == apexast.DeclarationProperty && hasModifier(member.Modifiers, "static") && d.Range != nil && member.Range == *d.Range {
							line := -1
							d.NativeLine = &line
						}
					}
				}
			}
		}
	}
	return nativeAnnotationDiagnostics(index, out)
}

// Preserve the member selected by inheritance checking. Reconstructing it from
// a diagnostic's name alone loses overload identity and declaration context.
func resolvedLifecycleMethod(model *semaTypeMemberView, owner string, member typesys.MemberSymbol) typesys.MemberSymbol {
	member.Parameters = append([]apexast.Parameter(nil), member.Parameters...)
	return semaNormalizeMemberTypes(model, owner, member)
}

func nativeLifecycleOverrideMessage(model *semaTypeMemberView, owner typesys.TypeSymbol, member typesys.MemberSymbol) string {
	member = resolvedLifecycleMethod(model, owner.Name, member)
	prefix := "@Override specified for non-overriding method: "
	parentName := resolveNestedTypeReference(model, owner.Name, owner.SuperClass)
	if parent, ok := model.lookup(normalizeName(parentName)); ok {
		for _, inherited := range parent.methods[normalizeName(member.Name)] {
			inherited = resolvedLifecycleMethod(model, parent.name, inherited)
			if memberSignatureKey(inherited) == memberSignatureKey(member) {
				prefix = "Non-virtual, non-abstract methods cannot be overridden: "
				break
			}
		}
	}
	return prefix + nativeLifecycleSignature(owner.Name, member)
}

func nativeLifecycleMissingMethodMessage(model *semaTypeMemberView, owner typesys.TypeSymbol, requirement methodRequirement) string {
	required := resolvedLifecycleMethod(model, requirement.owner, requirement.member)
	methodOwner := requirement.owner
	// Retain the namespace of the declaring platform abstract type.
	if declaringType, ok := model.lookupName(methodOwner); ok && requirement.sourceKind == "abstract" &&
		declaringType.platform && declaringType.namespace != "" && !strings.EqualFold(declaringType.namespace, "System") &&
		!strings.HasPrefix(strings.ToLower(methodOwner), strings.ToLower(declaringType.namespace)+".") {
		methodOwner = declaringType.namespace + "." + methodOwner
	}
	// The declaring EventBus interfaces retain their native namespace.
	if declaringType, ok := model.lookupName(methodOwner); ok && declaringType.platform && strings.EqualFold(declaringType.namespace, "eventbus") &&
		(strings.EqualFold(declaringType.name, "EventPublishSuccessCallback") || strings.EqualFold(declaringType.name, "eventbus.EventPublishSuccessCallback") ||
			strings.EqualFold(declaringType.name, "EventPublishFailureCallback") || strings.EqualFold(declaringType.name, "eventbus.EventPublishFailureCallback")) {
		callbackName := "EventPublishSuccessCallback"
		if strings.HasSuffix(strings.ToLower(declaringType.name), "eventpublishfailurecallback") {
			callbackName = "EventPublishFailureCallback"
		}
		methodOwner = "eventbus." + callbackName
		for i := range required.Parameters {
			for _, resultType := range []string{"SuccessResult", "FailureResult"} {
				if strings.EqualFold(required.Parameters[i].Type, resultType) || strings.EqualFold(required.Parameters[i].Type, "eventbus."+resultType) {
					required.Parameters[i].Type = "eventbus." + resultType
				}
			}
		}
	}
	signature := nativeLifecycleSignature(methodOwner, required)
	if requirement.sourceKind == "interface" {
		for _, implementation := range owner.Members {
			if implementation.Kind != apexast.DeclarationMethod {
				continue
			}
			implementation = resolvedLifecycleMethod(model, owner.Name, implementation)
			// Inheritance already treats System type aliases as the
			// same signature; retain its visibility rejection in native text.
			if sameSemaSignature(implementation, required) && declarationVisibilityRank(implementation.Modifiers) < declarationVisibilityRank([]string{"public"}) {
				return fmt.Sprintf("%s: Overriding implementations of global or public interface methods must be global or public: %s", owner.Name, signature)
			}
		}
	}
	kind := "method"
	if requirement.sourceKind == "abstract" {
		kind = "abstract method"
	}
	return fmt.Sprintf("Class %s must implement the %s: %s", owner.Name, kind, signature)
}

func nativeLifecycleSignature(owner string, member typesys.MemberSymbol) string {
	args := make([]string, len(member.Parameters))
	for i, p := range member.Parameters {
		args[i] = p.Type
	}
	return fmt.Sprintf("%s %s.%s(%s)", member.Type, owner, member.Name, strings.Join(args, ","))
}

func nativeIRConstructorDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, display, target string, argTypes []string, namedCount int, ambiguous, platform bool, start, end int, source string) diagnostic.Diagnostic {
	detail := fmt.Sprintf("no matching %s constructor with %d argument(s)", display, len(argTypes)+namedCount)
	if ambiguous {
		detail = fmt.Sprintf("ambiguous %s constructor with %d argument(s)", display, len(argTypes)+namedCount)
	}
	d := constructorDiagnostic(typ, member, "new "+display, detail, start, end, source)
	if platform || namedCount != 0 {
		return d
	}
	args := append([]string(nil), argTypes...)
	for i, arg := range args {
		if strings.EqualFold(arg, "null") {
			args[i] = "NULL"
		}
	}
	if ambiguous {
		d.NativeMessage = "Ambiguous method signature: void <init>(" + strings.Join(args, ",") + ")"
	} else {
		d.NativeMessage = "Constructor not defined: [" + target + "].<Constructor>(" + strings.Join(args, ",") + ")"
	}
	return d
}
