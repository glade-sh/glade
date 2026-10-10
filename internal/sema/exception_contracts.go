package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// C004-C008/C039, K001-K008 and D001-D003 observe these native rules.
// Resolve source ancestry before consulting built-ins, so source classes that
// share a platform spelling retain their own inheritance rules.
func checkExceptionDeclarations(index typesys.Index) []diagnostic.Diagnostic {
	return checkExceptionDeclarationsWithView(index, buildSemaTypeMemberView(index))
}

func checkExceptionDeclarationsWithView(index typesys.Index, model *semaTypeMemberView) []diagnostic.Diagnostic {
	sources := make(map[string]typesys.TypeSymbol, len(index.Types))
	for _, typ := range index.Types {
		key := semaTypeSymbolKey(typ)
		if previous, exists := sources[key]; !exists || (previous.Dependency && !typ.Dependency) {
			sources[key] = typ
		}
	}
	resolve := func(owner typesys.TypeSymbol, name string) (string, bool) {
		if name == "" {
			return name, false
		}
		// Dependency artifacts retain a separate namespace and may have the
		// same short name as a class in another package. Follow each base in
		// its own namespace rather than flattening those symbols by name.
		if strings.Contains(name, ".") {
			if _, ok := sources[normalizeName(name)]; ok {
				return name, false
			}
		}
		for scope := owner.Name; scope != ""; {
			if owner.Namespace != "" {
				candidate := owner.Namespace + "." + scope + "." + name
				if _, ok := sources[normalizeName(candidate)]; ok {
					return candidate, false
				}
			}
			if _, ok := sources[normalizeName(scope+"."+name)]; ok {
				return scope + "." + name, false
			}
			dot := strings.LastIndexByte(scope, '.')
			if dot < 0 {
				break
			}
			scope = scope[:dot]
		}
		if owner.Namespace != "" {
			candidate := owner.Namespace + "." + name
			if _, ok := sources[normalizeName(candidate)]; ok {
				return candidate, false
			}
		}
		if _, ok := sources[normalizeName(name)]; !ok {
			// An inner class can extend a type inherited by its enclosing
			// class. Use the same nested-type lookup as member/body analysis,
			// while retaining source and package precedence above.
			// C007-C010 in the inherited-inner named capture resolve the
			// private base but reject its visibility. Preserve ancestry so
			// this does not become an unrelated exception-base rejection.
			// Lexical lookup above still permits private sibling bases.
			inaccessible := func(source typesys.TypeSymbol) bool {
				return !hasExplicitDeclarationVisibility(source.Modifiers) || hasModifier(source.Modifiers, "private")
			}
			resolved := resolveNestedTypeName(model, semaTypeMembersName(owner), name)
			if source, ok := sources[normalizeName(resolved)]; ok {
				return resolved, inaccessible(source)
			}
			if owner.Namespace != "" {
				candidate := owner.Namespace + "." + resolved
				if source, ok := sources[normalizeName(candidate)]; ok {
					return candidate, inaccessible(source)
				}
			}
		}
		return name, false
	}
	var derivesException func(string, map[string]bool) bool
	derivesException = func(name string, seen map[string]bool) bool {
		key := normalizeName(name)
		if seen[key] {
			return false
		}
		seen[key] = true
		if source, ok := sources[key]; ok {
			parent, _ := resolve(source, source.SuperClass)
			return derivesException(parent, seen)
		}
		return strings.EqualFold(semaCanonicalPlatformAlias(name), "Exception") || semaStandardExceptionType(name)
	}
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || typ.Kind != apexast.DeclarationClass {
			continue
		}
		name := typ.Name
		parent, hidden := resolve(typ, typ.SuperClass)
		isException := derivesException(parent, make(map[string]bool))
		endsException := strings.HasSuffix(strings.ToLower(name), "exception")
		add := func(message string, where diagnostic.Range) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA030", Message: message, NativeMessage: message, File: typ.File, Range: &where})
		}
		// C007-C010 observe visibility only for exception ancestry.
		// Ordinary-class visibility belongs to its own captured contract.
		if hidden && isException {
			add("Type is not visible: "+parent, typ.Range)
			diagnostics[len(diagnostics)-1].Code = "GLADESEMA017"
		}
		if isException && !endsException {
			add("Classes extending Exception must have a name ending in Exception: "+name, typ.Range)
			diagnostics[len(diagnostics)-1].Message = fmt.Sprintf("custom exception class %q must end with Exception", typ.Name)
		}
		if endsException && !isException {
			base := parent
			if base == "" {
				base = name
			}
			add("Exception class must extend another Exception class: "+base, typ.Range)
		}
		if _, shadowed := sources[normalizeName(parent)]; !shadowed && strings.EqualFold(semaCanonicalPlatformAlias(parent), "LimitException") {
			add("Non-virtual and non-abstract type cannot be extended: System.LimitException", typ.Range)
		}
		if !isException {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationConstructor {
				continue
			}
			args := make([]string, len(member.Parameters))
			for i, param := range member.Parameters {
				args[i] = semaCanonicalPlatformAlias(param.Type)
			}
			for _, inherited := range inheritedExceptionConstructorSignatures(typ.Name) {
				if len(args) != len(inherited) {
					continue
				}
				matches := true
				for i := range args {
					matches = matches && strings.EqualFold(args[i], inherited[i])
				}
				if matches {
					add("System exception constructor already defined: void <init>("+strings.Join(args, ", ")+")", member.Range)
					break
				}
			}
		}
	}
	return diagnostics
}

func exceptionTypeMembers(name string, model *semaTypeMemberView) (typeMembers, bool) {
	members, _, ok := semaLookupTypeMembers(model, name)
	if !ok {
		return typeMembers{}, false
	}
	// R019/R024/R029/R034/R039/R044/R049: generated standard exceptions
	// need not carry an explicit superclass. Source names retain their own
	// ancestry, and explicit System aliases use the ordinary qualified lookup.
	if members.platform && (strings.EqualFold(semaCanonicalPlatformAlias(name), "Exception") || semaStandardExceptionType(name)) {
		return members, true
	}
	return members, semaTypeMatches(model, name, "Exception", make(map[string]bool))
}

func exceptionDiagnosticType(name string, model *semaTypeMemberView) string {
	if target, ok := exceptionTypeMembers(name, model); ok && target.platform && !strings.EqualFold(semaCanonicalPlatformAlias(name), "Exception") {
		if !strings.Contains(name, ".") {
			return "System." + name
		}
	}
	return name
}

func exceptionSyntaxMessage(message string, anonymous bool) string {
	if !anonymous {
		if token, ok := strings.CutPrefix(message, "Missing '<EOF>' at "); ok {
			return "Expecting '}' but was: " + token
		}
	}
	return message
}

func exceptionConstructorDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, target string, args []string, model *semaTypeMemberView, start, end int, source string) (diagnostic.Diagnostic, bool) {
	constructors, found := exceptionTypeMembers(target, model)
	if !found {
		return diagnostic.Diagnostic{}, false
	}
	message := ""
	if constructors.platform {
		if strings.EqualFold(semaCanonicalPlatformAlias(target), "Exception") {
			message = "Type cannot be constructed: Exception"
		} else if _, matched, ambiguous := bestMemberByArgTypes(constructors.constructors, args, model); ambiguous {
			message = "Ambiguous method signature: void <init>(" + exceptionDiagnosticArgs(args, model) + ")"
		} else if !matched {
			message = "Constructor not defined: [" + exceptionDiagnosticType(target, model) + "].<Constructor>(" + exceptionDiagnosticArgs(args, model) + ")"
		}
	} else if len(args) == 1 && strings.EqualFold(args[0], "null") {
		// R004/R009: String and Exception inherited overloads both accept
		// untyped null. A cast supplies a type and avoids this ambiguity.
		message = "Ambiguous method signature: void <init>(NULL)"
	}
	if message == "" {
		return diagnostic.Diagnostic{}, false
	}
	detail := fmt.Sprintf("no matching %s constructor with %d argument(s)", target, len(args))
	if strings.HasPrefix(message, "Ambiguous method signature:") {
		detail = fmt.Sprintf("ambiguous %s constructor with %d argument(s)", target, len(args))
	}
	d := constructorDiagnostic(typ, member, "new "+target, detail, start, end, source)
	d.NativeMessage = message
	return d, true
}

func exceptionDiagnosticArgs(args []string, model *semaTypeMemberView) string {
	names := make([]string, len(args))
	for i, arg := range args {
		if strings.EqualFold(arg, "null") {
			names[i] = "NULL"
		} else {
			names[i] = exceptionDiagnosticType(arg, model)
		}
	}
	return strings.Join(names, ", ")
}

func exceptionCallDiagnostic(typ typesys.TypeSymbol, receiver, method string, args []string, model *semaTypeMemberView, start, end int, source string) (diagnostic.Diagnostic, bool) {
	if !semaTypeMatches(model, receiver, "Exception", make(map[string]bool)) {
		return diagnostic.Diagnostic{}, false
	}
	// A source method has its own contract, including overrides (C011).
	candidates := resolveMemberMethods(model, receiver, method)
	if len(candidates) > 0 && !semaResolvedMembersAllPlatformBacked(model, candidates) {
		return diagnostic.Diagnostic{}, false
	}
	var signatures [][]string
	switch strings.ToLower(method) {
	case "getmessage", "gettypename", "getlinenumber", "getstacktracestring", "getcause":
		signatures = [][]string{{}}
	case "setmessage":
		signatures = [][]string{{"String"}}
	case "setcause":
		// C042: initCause exists; setCause does not.
	default:
		return diagnostic.Diagnostic{}, false
	}
	if semaArgsMatchAny(signatures, args, model) {
		return diagnostic.Diagnostic{}, false
	}
	message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, exceptionDiagnosticArgs(args, model), receiver)
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_CALL", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, end)}, true
}
