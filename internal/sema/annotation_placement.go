package sema

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlang"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var annotationDuplicateLocal = regexp.MustCompile(`redeclares local variable "([^"]+)" in the same scope$`)

func annotationCatalogForTarget(typ typesys.TypeSymbol, target, name, typeName string, modifiers []string, annotations []apexast.Annotation) []diagnostic.Diagnostic {
	catalog := annotationCatalogDiagnostics(typ.File, typ.Range, annotations)
	if placement := annotationPlacementDiagnostics(typ, target, name, typeName, modifiers, annotations); len(placement) > 0 {
		for i := range catalog {
			catalog[i].NativeMessage = placement[0].NativeMessage
			catalog[i].NativeLine = placement[0].NativeLine
			catalog[i].Range = placement[0].Range
		}
	}
	return catalog
}

func annotationTarget(kind apexast.DeclarationKind) string {
	switch kind {
	case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
		return "classes"
	case apexast.DeclarationMethod:
		return "methods"
	case apexast.DeclarationConstructor:
		return "constructors"
	case apexast.DeclarationProperty:
		return "properties"
	default:
		return "fields"
	}
}

func nativeAnnotationDiagnostic(file string, r diagnostic.Range, message string) diagnostic.Diagnostic {
	d := annotationContractDiagnostic(file, r, message)
	d.NativeMessage = message
	return d
}

// Placement comes before signature/attribute contracts. These diagnostics are
// observations from owned annotation-placement cases.
func annotationPlacementDiagnostics(typ typesys.TypeSymbol, target string, name, typeName string, modifiers []string, annotations []apexast.Annotation) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	seen := map[string]bool{}
	for _, annotation := range annotations {
		spec, known := apexlang.LookupAnnotation(annotation.Name)
		if !known {
			continue // The catalog owns unknown annotations.
		}
		annotationName := spec.Name
		if strings.EqualFold(annotationName, "future") {
			annotationName = "Future"
		}
		message := ""
		key := strings.ToLower(annotation.Name)
		if seen[key] {
			message = "Duplicate modifier: " + annotationName
		}
		seen[key] = true
		allowed := false
		switch key {
		case "testvisible", "deprecated":
			allowed = target != "parameters"
		case "suppresswarnings":
			allowed = true
		case "jsonaccess":
			allowed = target == "classes"
		case "namespaceaccessible":
			allowed = target != "parameters"
		case "restresource":
			allowed = target == "classes" || target == "methods"
		case "istest":
			allowed = target == "classes" || target == "methods"
		case "auraenabled":
			allowed = target == "methods" || target == "fields" || target == "properties"
		case "invocablevariable":
			allowed = target == "fields"
		case "remoteaction":
			// The e_annotation_getter case accepts a property annotation; it does
			// not expose that property as a callable remoting method.
			allowed = target == "methods" || target == "properties"
		default:
			allowed = target == "methods"
		}
		if message == "" && !allowed {
			message = annotationName + " is not allowed on " + target
		}
		if message == "" {
			switch key {
			case "deprecated":
				if !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) && typ.Namespace == "" && !typ.Dependency && target != "parameters" {
					identifier := typ.Name
					switch target {
					case "fields", "properties":
						identifier = name
					case "methods", "constructors":
						for _, member := range typ.Members {
							if member.Name == name && member.Range.Start.Offset <= annotation.Range.Start.Offset && member.Range.End.Offset >= annotation.Range.End.Offset {
								identifier = nativeLifecycleSignature(typ.Name, member)
								if target == "constructors" {
									identifier = "void " + typ.Name + ".<init>()"
								}
								break
							}
						}
					}
					message = "Only managed identifiers can be marked deprecated: " + identifier
				}
			case "auraenabled":
				if target == "methods" && !hasModifier(modifiers, "static") && !strings.HasPrefix(strings.ToLower(name), "get") {
					message = "Non static AuraEnabled methods must be named with a prefix 'get'"
				} else if (target == "fields" || target == "properties") && !hasEitherModifier(modifiers, "global", "public") {
					message = "AuraEnabled " + target + " require at least one of the following global, public"
				} else if target == "fields" || target == "properties" {
					for _, argument := range annotation.Arguments {
						if strings.EqualFold(argument.Name, "cacheable") {
							message = "Annotation property, cacheable on AuraEnabled, is not allowed on " + target
							break
						}
					}
				}
			case "invocablemethod", "future":
				if !hasModifier(modifiers, "static") {
					message = annotationName + " methods must be declared as static"
				}
			case "remoteaction":
				// Instance and private remoting cases are checked here; protected declarations
				// receive their class-contract errors instead (e_annotation_protected).
				if target == "methods" && !hasModifier(modifiers, "static") {
					message = "RemoteAction methods must be declared as static"
				} else if target == "methods" && !hasModifier(modifiers, "protected") && !hasEitherModifier(modifiers, "public", "global") {
					message = "RemoteAction methods require at least one of the following global, public"
				}
			case "invocablevariable":
				switch {
				case !hasEitherModifier(modifiers, "global", "public"):
					message = "InvocableVariable fields require at least one of the following global, public"
				case hasModifier(modifiers, "static"):
					message = "InvocableVariable fields cannot be static"
				case hasModifier(modifiers, "final"):
					message = "InvocableVariable fields cannot be final"
				case strings.HasPrefix(strings.ToLower(strings.ReplaceAll(typeName, " ", "")), "set<"):
					message = "InvocableVariable fields do not support type of " + strings.ReplaceAll(typeName, " ", "")
				}
			case "restresource", "httpget", "httppost", "httpput", "httppatch", "httpdelete":
				if !hasModifier(modifiers, "global") {
					message = annotationName + " " + target + " must be declared as global"
				}
			case "istest", "testsetup":
				if typ.NestingDepth > 0 {
					if target == "classes" {
						message = "IsTest can only be used on a top level type"
					} else {
						message = annotationName + " can only be used on methods of a top level type"
					}
				} else if key == "istest" && target == "classes" && hasModifier(modifiers, "virtual") {
					// W5 matrix C001/C011/C012, APIs 62/64/67: the test
					// annotation rejects a virtual outer class before its body.
					message = "IsTest classes cannot be virtual"
				}
			case "readonly":
				if !readOnlyMethodAllowed(typ, name, modifiers, annotations) {
					message = "Only WebService, RemoteAction or Schedulable.execute(SchedulableContext) methods can be marked ReadOnly"
					// S012: the named static overload has its own modifier
					// rejection; anonymous inner-type static errors precede it.
					if target == "methods" && strings.EqualFold(name, "execute") && readOnlySchedulableType(typ) && hasModifier(modifiers, "static") && !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
						message = "ReadOnly methods require at least one of the following webService, RemoteAction, HttpGet, HttpPost, HttpPatch, HttpPut, HttpDelete"
					}
				}
			case "namespaceaccessible":
				if annotationAPIVersionAtLeast(typ, 50) {
					if target != "classes" && !hasEitherModifier(modifiers, "public", "protected") && !hasModifier(modifiers, "global") {
						message = "NamespaceAccessible " + target + " require at least one of the following public, protected"
					} else if target == "classes" && typ.NestingDepth > 0 && hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
						message = "Enclosing type for NamespaceAccessible classes in apex classes require at least one of the following NamespaceAccessible, global"
					} else if target != "classes" && !hasAnnotation(typ.Annotations, "NamespaceAccessible") && !hasModifier(typ.Modifiers, "global") {
						message = "Defining type for NamespaceAccessible " + target + " require at least one of the following NamespaceAccessible, global"
					}
				}
			}
		}
		if message == "" {
			message = annotationNativeArgumentMessage(annotation, target)
		}
		if message != "" {
			d := nativeAnnotationDiagnostic(typ.File, annotation.Range, message)
			if strings.HasPrefix(message, "Enclosing type for NamespaceAccessible") {
				line := 1
				d.NativeLine = &line
			}
			out = append(out, d)
		}
	}
	return out
}

func readOnlyMethodAllowed(typ typesys.TypeSymbol, name string, modifiers []string, annotations []apexast.Annotation) bool {
	if hasModifier(modifiers, "webservice") || hasAnnotation(annotations, "RemoteAction") || hasAnnotation(typ.Annotations, "RestResource") && hasAnyAnnotation(modifiers, "HttpGet", "HttpPost", "HttpPut", "HttpPatch", "HttpDelete") {
		return true
	}
	if !readOnlySchedulableType(typ) {
		return false
	}
	// S001-S013: bind the annotation to its exact overload, then require the
	// public/global instance interface signature. A valid sibling is insufficient.
	for _, member := range typ.Members {
		if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(member.Name, name) {
			continue
		}
		for _, annotation := range annotations {
			if !strings.EqualFold(annotation.Name, "ReadOnly") {
				continue
			}
			for _, declared := range member.Annotations {
				if strings.EqualFold(declared.Name, "ReadOnly") && declared.Range == annotation.Range {
					return readOnlySchedulableContext(member) && strings.EqualFold(member.Type, "void") && hasEitherModifier(member.Modifiers, "public", "global") && !hasModifier(member.Modifiers, "static")
				}
			}
		}
	}
	return false
}

func readOnlySchedulableType(typ typesys.TypeSymbol) bool {
	for _, iface := range typ.Interfaces {
		if strings.EqualFold(iface, "Schedulable") || strings.EqualFold(iface, "System.Schedulable") {
			return true
		}
	}
	return false
}

func readOnlySchedulableContext(member typesys.MemberSymbol) bool {
	return member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, "execute") && len(member.Parameters) == 1 && (strings.EqualFold(member.Parameters[0].Type, "SchedulableContext") || strings.EqualFold(member.Parameters[0].Type, "System.SchedulableContext"))
}

func annotationNativeArgumentMessage(annotation apexast.Annotation, target string) string {
	if strings.EqualFold(annotation.Name, "JsonAccess") && target == "classes" {
		if len(annotation.Arguments) == 0 {
			return "At least one JSON serialization control parameter must be specified"
		}
		for _, argument := range annotation.Arguments {
			if !strings.EqualFold(argument.Name, "serializable") && !strings.EqualFold(argument.Name, "deserializable") {
				continue
			}
			value, ok := apexStringLiteralValue(argument.Value)
			if !ok {
				return "Invalid value for property " + argument.Name + " expected type String"
			}
			switch strings.ToLower(value) {
			case "always", "never", "samenamespace", "samepackage":
			default:
				return fmt.Sprintf("Annotation property, %s on JsonAccess, unknown value: %s", argument.Name, value)
			}
		}
	}
	if strings.EqualFold(annotation.Name, "AuraEnabled") {
		for _, argument := range annotation.Arguments {
			if !strings.EqualFold(argument.Name, "cacheable") && !strings.EqualFold(argument.Name, "scope") {
				return "No such property, " + argument.Name + ", defined on this annotation: AuraEnabled"
			}
		}
	}
	return ""
}

// Exposure errors can precede the implicit anonymous type's static restriction
// (C136, C162-C192, C227-C228). Keep this limited to annotated/webservice
// declarations; ordinary lifecycle declarations retain their existing checks.
func anonymousExposureDiagnostics(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	makeError := func(r diagnostic.Range, message string, enclosing bool) []diagnostic.Diagnostic {
		d := nativeAnnotationDiagnostic(typ.File, r, message)
		if enclosing {
			line := 1
			d.NativeLine = &line
		}
		return []diagnostic.Diagnostic{d}
	}
	if hasModifier(typ.Modifiers, "webservice") {
		return makeError(typ.Range, "webService is not allowed on classes", false)
	}
	for _, member := range typ.Members {
		if hasModifier(member.Modifiers, "webservice") {
			target := annotationTarget(member.Kind)
			if target == "constructors" {
				return makeError(member.Range, "webService is not allowed on constructors", false)
			}
			if target == "methods" {
				for _, parameter := range member.Parameters {
					if !validSOAPType(parameter.Type) {
						return makeError(member.Range, "webService methods do not support parameter type of "+strings.ReplaceAll(parameter.Type, " ", ""), false)
					}
				}
			}
			if !hasModifier(typ.Modifiers, "global") {
				return makeError(member.Range, "Defining type for webService "+target+" must be declared as global", false)
			}
			return makeError(member.Range, "Enclosing type for global "+target+" in apex classes must be declared as global", true)
		}
		if hasModifier(member.Modifiers, "global") && len(member.Annotations) > 0 {
			target := annotationTarget(member.Kind)
			if !hasModifier(typ.Modifiers, "global") {
				return makeError(member.Range, "Defining type for global "+target+" must be declared as global", false)
			}
			return makeError(member.Range, "Enclosing type for global "+target+" in apex classes must be declared as global", true)
		}
		for _, annotation := range member.Annotations {
			if restVerb(annotation.Name) != "" && !hasModifier(member.Modifiers, "global") {
				return annotationPlacementDiagnostics(typ, annotationTarget(member.Kind), member.Name, member.Type, member.Modifiers, []apexast.Annotation{annotation})
			}
		}
	}
	if hasModifier(typ.Modifiers, "global") && len(typ.Annotations) > 0 {
		return makeError(typ.Range, "Enclosing type for global classes in apex classes must be declared as global", true)
	}
	return nil
}

func namedAnnotationExposureDiagnostics(index typesys.Index, typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	if typ.NestingDepth == 0 || hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
		return nil
	}
	for _, parent := range index.Types {
		// Enclosing declarations share a source file and namespace. A loaded
		// dependency may declare the same name with different exposure.
		if parent.File != typ.File || !strings.EqualFold(parent.Namespace, typ.Namespace) || !strings.EqualFold(parent.Name, typ.OwnerName) {
			continue
		}
		if !hasModifier(parent.Modifiers, "global") {
			if diagnostics := anonymousExposureDiagnostics(typ); len(diagnostics) > 0 {
				for i := range diagnostics {
					diagnostics[i].NativeLine = nil
				}
				return diagnostics
			}
		}
		if hasAnnotation(typ.Annotations, "NamespaceAccessible") && !hasAnnotation(parent.Annotations, "NamespaceAccessible") && !hasModifier(parent.Modifiers, "global") {
			return []diagnostic.Diagnostic{nativeAnnotationDiagnostic(typ.File, typ.Range, "Enclosing type for NamespaceAccessible classes in apex classes require at least one of the following NamespaceAccessible, global")}
		}
	}
	return nil
}

// Preserve declaration-error precedence for annotation exposure in both source
// contexts. Unannotated lifecycle declarations keep their existing messages.
func nativeAnnotationDiagnostics(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	// Preserve the resolved interface diagnostic before ReadOnly
	// admission errors when the annotated implementation is private/non-void.
	schedulableImplementationError := func(d diagnostic.Diagnostic) bool {
		if d.Code != "GLADESEMA017" || d.Range == nil || !strings.HasSuffix(d.NativeMessage, ": void System.Schedulable.execute(System.SchedulableContext)") {
			return false
		}
		for _, typ := range index.Types {
			if typ.File != d.File || typ.Range != *d.Range || !readOnlySchedulableType(typ) {
				continue
			}
			for _, member := range typ.Members {
				if readOnlySchedulableContext(member) && hasAnnotation(member.Annotations, "ReadOnly") {
					return true
				}
			}
		}
		return false
	}
	owned := func(d diagnostic.Diagnostic) bool {
		if d.Range == nil {
			return false
		}
		if schedulableImplementationError(d) {
			return true
		}
		var owner typesys.TypeSymbol
		for _, typ := range index.Types {
			if typ.File != d.File || typ.Range.Start.Offset > d.Range.Start.Offset || typ.Range.End.Offset < d.Range.Start.Offset {
				continue
			}
			if owner.Name == "" || typ.NestingDepth > owner.NestingDepth {
				owner = typ
			}
		}
		if len(owner.Annotations) > 0 || hasModifier(owner.Modifiers, "webservice") {
			return true
		}
		for _, member := range owner.Members {
			// Field ranges begin at the declarator, after its annotations.
			// C223's Deprecated diagnostic still belongs to the annotated
			// field and must participate in declaration-error precedence.
			for _, annotation := range member.Annotations {
				if annotation.Range.Start.Offset <= d.Range.Start.Offset && annotation.Range.End.Offset >= d.Range.Start.Offset {
					return true
				}
			}
			if member.Range.Start.Offset <= d.Range.Start.Offset && member.Range.End.Offset >= d.Range.Start.Offset && (len(member.Annotations) > 0 || hasModifier(member.Modifiers, "webservice")) {
				return true
			}
		}
		return false
	}
	for i := range diagnostics {
		d := &diagnostics[i]
		if !owned(*d) || d.NativeMessage != "" {
			continue
		}
		if strings.HasPrefix(d.Message, "inner type ") && strings.Contains(d.Message, "cannot declare a static method") {
			d.NativeMessage = "static can only be used on methods of a top level type"
		}
		if strings.HasPrefix(d.Message, "global method ") && strings.Contains(d.Message, "requires a global enclosing type") {
			d.NativeMessage = "Defining type for global methods must be declared as global"
		}
		// Named C098/C112/C212-C213/C223 reject the observation local's
		// shadowing before inspecting declaration annotations.
		if match := annotationDuplicateLocal.FindStringSubmatch(d.Message); match != nil {
			d.NativeMessage = "Duplicate variable: " + match[1]
		}
	}
	priority := func(d diagnostic.Diagnostic) int {
		if !owned(d) {
			return 1
		}
		message := d.NativeMessage
		if message == "" {
			message = d.Message
		}
		// Named C243 rejects the literal before duplicate-local analysis.
		if message == "Illegal integer" {
			return -1
		}
		if schedulableImplementationError(d) {
			return 0
		}
		if strings.HasPrefix(message, "Defining type for global ") || strings.HasPrefix(message, "Enclosing type for global ") || strings.HasPrefix(message, "Defining type for webService ") || strings.HasPrefix(message, "webService methods do not support parameter type") || strings.HasPrefix(message, "webService is not allowed") {
			return 0
		}
		if strings.HasPrefix(message, "static can only") {
			return 1
		}
		if strings.HasPrefix(message, "Duplicate variable:") {
			return 0
		}
		return 2
	}
	// Reorder only annotation-owned slots in each file. Other declarations and
	// other files retain their diagnostic order.
	files := map[string][]int{}
	for i, d := range diagnostics {
		if owned(d) {
			files[d.File] = append(files[d.File], i)
		}
	}
	for _, slots := range files {
		group := make([]diagnostic.Diagnostic, len(slots))
		for i, slot := range slots {
			group[i] = diagnostics[slot]
		}
		sort.SliceStable(group, func(i, j int) bool { return priority(group[i]) < priority(group[j]) })
		for i, slot := range slots {
			diagnostics[slot] = group[i]
		}
	}
	return diagnostics
}

func localAnnotationDiagnostics(typ typesys.TypeSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	if !strings.Contains(body, "@") && !strings.Contains(strings.ToLower(body), "webservice") {
		return nil
	}
	ast := apexast.ParseSourceAST(typ.File, body)
	var out []diagnostic.Diagnostic
	var visit func(apexast.ASTNode)
	visit = func(node apexast.ASTNode) {
		if node.Kind == "annotation" {
			start, end := node.Range.Start.Offset, node.Range.End.Offset
			if start < 0 || end <= start || end > len(body) {
				return
			}
			text := strings.TrimSpace(strings.TrimPrefix(body[start:end], "@"))
			name, _, _ := strings.Cut(text, "(")
			if spec, known := apexlang.LookupAnnotation(strings.TrimSpace(name)); known && !strings.EqualFold(spec.Name, "TestVisible") && !strings.EqualFold(spec.Name, "SuppressWarnings") {
				name = spec.Name
				if strings.EqualFold(name, "future") {
					name = "Future"
				}
				d := nativeAnnotationDiagnostic(typ.File, *semaRange(source, bodyOffset+start, bodyOffset+end), name+" is not allowed on locals")
				out = append(out, d)
			}
		}
		if node.Kind == "modifiers" && node.Range.Start.Offset >= 0 && node.Range.End.Offset <= len(body) && strings.EqualFold(strings.TrimSpace(body[node.Range.Start.Offset:node.Range.End.Offset]), "webservice") {
			out = append(out, nativeAnnotationDiagnostic(typ.File, *semaRange(source, bodyOffset+node.Range.Start.Offset, bodyOffset+node.Range.End.Offset), "webService is not allowed on locals"))
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, node := range ast.Nodes {
		visit(node)
	}
	return out
}
