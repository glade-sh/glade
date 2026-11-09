package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// anonymous_local_methods_c4 (API 62.0 and 67.0): methods declared at the top
// level of an anonymous block belong to type anon, are static whatever their
// modifiers, and see the block's top-level variables. The anonymous body and
// each method body resolve unqualified calls against them.
const anonymousLocalTypeName = "anon"

// installAnonymousLocalMethods registers the local methods as members of the
// anonymous type in model and reports duplicate signatures.
func installAnonymousLocalMethods(model *semaTypeMemberView, typ typesys.TypeSymbol, program ir.Program, source string) []diagnostic.Diagnostic {
	if len(program.LocalMethods) == 0 || model == nil {
		return nil
	}
	members := typeMembers{name: typ.Name, kind: apexast.DeclarationClass, methods: map[string][]typesys.MemberSymbol{}}
	seen := map[string]bool{}
	var diagnostics []diagnostic.Diagnostic
	for _, local := range program.LocalMethods {
		symbol := anonymousLocalMemberSymbol(local, source)
		var types []string
		for _, param := range local.Params {
			types = append(types, strings.ToLower(param.Type))
		}
		signature := strings.ToLower(local.Name) + "(" + strings.Join(types, ",") + ")"
		if seen[signature] {
			var shown []string
			for _, param := range local.Params {
				shown = append(shown, param.Type)
			}
			message := fmt.Sprintf("Method already defined: %s %s %s.%s(%s) from the type %s", local.Name, local.ReturnType, anonymousLocalTypeName, local.Name, strings.Join(shown, ", "), anonymousLocalTypeName)
			rng := semaRange(source, local.NamePos, local.NamePos+len(local.Name))
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_ANONYMOUS_METHOD", Message: message, NativeMessage: message, Range: rng})
			continue
		}
		seen[signature] = true
		key := normalizeName(local.Name)
		members.methods[key] = append(members.methods[key], symbol)
	}
	if model.current == nil {
		model.current = map[string]typeMembers{}
	}
	model.current[model.canonicalName(typ.Name)] = members
	model.changedHydration()
	return diagnostics
}

func anonymousLocalMemberSymbol(local ir.LocalMethod, source string) typesys.MemberSymbol {
	params := make([]apexast.Parameter, 0, len(local.Params))
	for _, param := range local.Params {
		params = append(params, apexast.Parameter{Name: param.Name, Type: param.Type})
	}
	bodyRange := semaRange(source, local.BodyStart, local.BodyEnd)
	return typesys.MemberSymbol{
		Kind:       apexast.DeclarationMethod,
		Name:       local.Name,
		Type:       local.ReturnType,
		Modifiers:  []string{"static"},
		Parameters: params,
		HasBody:    true,
		BodyRange:  bodyRange,
		Range:      *semaRange(source, local.Pos, local.BodyEnd),
	}
}

// anonymousTopLevelScope is the variable scope a local method body sees: the
// block's top-level declarations, wherever they appear in the block.
func anonymousTopLevelScope(program ir.Program) map[string]string {
	scope := map[string]string{}
	for _, instruction := range program.Instructions {
		if instruction.Op == ir.OpDeclare && instruction.Name != "" {
			scope[instruction.Name] = instruction.Type
		}
	}
	return scope
}

// anonymousThisDiagnostic reports this inside the anonymous block, a static
// context (anonymous_local_methods_c4 this_call: Ln 2, Col 26).
func anonymousThisDiagnostic(program ir.Program, source string) []diagnostic.Diagnostic {
	if len(program.LocalMethods) == 0 {
		return nil
	}
	if offset := anonymousThisOffset(source); offset >= 0 {
		return []diagnostic.Diagnostic{anonymousStaticThis(source, offset)}
	}
	return nil
}

// anonymousThisOffset finds the first this keyword outside string literals
// and comments, or -1.
func anonymousThisOffset(source string) int {
	for i := 0; i < len(source); i++ {
		switch {
		case source[i] == '\'':
			for i++; i < len(source) && source[i] != '\''; i++ {
				if source[i] == '\\' {
					i++
				}
			}
		case strings.HasPrefix(source[i:], "//"):
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case strings.HasPrefix(source[i:], "/*"):
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				return -1
			}
			i += end + 3
		case isIdentifierByte(source[i]):
			start := i
			for i < len(source) && isIdentifierByte(source[i]) {
				i++
			}
			if strings.EqualFold(source[start:i], "this") && (start == 0 || source[start-1] != '.') {
				return start
			}
			i--
		}
	}
	return -1
}

func anonymousStaticThis(source string, offset int) diagnostic.Diagnostic {
	message := "This cannot be referenced in a static context"
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_ANONYMOUS_METHOD", Message: message, NativeMessage: message, Range: semaRange(source, offset, offset+4)}
}

// anonymousLocalMethodNativeDiagnostics gives a local method's missing return
// its native text and position (anonymous_local_methods_c4 missing_return:
// "Missing return statement required return type: Integer", Ln 1, Col 9).
func anonymousLocalMethodNativeDiagnostics(diagnostics []diagnostic.Diagnostic, local ir.LocalMethod, source string) []diagnostic.Diagnostic {
	// The body checker may share its slice with a memo; edit a copy.
	diagnostics = append([]diagnostic.Diagnostic(nil), diagnostics...)
	for i := range diagnostics {
		if diagnostics[i].Code == "GLADESEMA019" && strings.HasSuffix(diagnostics[i].Message, "on all paths") {
			diagnostics[i].NativeMessage = "Missing return statement required return type: " + local.ReturnType
			diagnostics[i].Range = semaRange(source, local.NamePos, local.NamePos+len(local.Name))
		}
	}
	return diagnostics
}

// anonymousVoidAssignmentNativeDiagnostics gives a top-level declaration
// initialized from a void local method its native text
// (anonymous_local_methods_c4 void_value: "Illegal assignment from void to Object").
func anonymousVoidAssignmentNativeDiagnostics(diagnostics []diagnostic.Diagnostic, outer map[string]string, program ir.Program, source string) []diagnostic.Diagnostic {
	diagnostics = append([]diagnostic.Diagnostic(nil), diagnostics...)
	for i := range diagnostics {
		item := &diagnostics[i]
		if item.Code != "GLADESEMA018" || !strings.HasSuffix(item.Message, " with void") {
			continue
		}
		_, rest, ok := strings.Cut(item.Message, " initializes ")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(rest, " with void")
		if declared, ok := outer[name]; ok {
			item.NativeMessage = "Illegal assignment from void to " + declared
			// Native points at the declared name (void_value: Ln 2, Col 8).
			for _, instruction := range program.Instructions {
				if instruction.Op != ir.OpDeclare || instruction.Name != name || instruction.Pos < 0 || instruction.Pos >= len(source) {
					continue
				}
				if offset := anonymousWordOffset(source, instruction.Pos, name); offset >= 0 {
					item.Range = semaRange(source, offset, offset+len(name))
				}
				break
			}
		}
	}
	return diagnostics
}

func anonymousWordOffset(source string, from int, word string) int {
	for i := from; i+len(word) <= len(source); i++ {
		if strings.EqualFold(source[i:i+len(word)], word) && (i == 0 || !isIdentifierByte(source[i-1])) && (i+len(word) == len(source) || !isIdentifierByte(source[i+len(word)])) {
			return i
		}
	}
	return -1
}
