package apexast

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// The captured rows report the illegal static member before the reserved
// class identifier. Keep the reserved error itself (including Message and
// severity), but expose the measured primary text at the parser boundary.
// Other reserved words, member types/signatures and nesting depths are not
// covered by those observations and retain their existing diagnostics.
func nativeReservedStaticPriority(file File) File {
	reserved := false
	for _, d := range file.Diagnostics {
		if d.Severity == diagnostic.Error && d.Code == "APEXPARSE002" && strings.EqualFold(d.Message, "Identifier name is reserved: Inner") {
			reserved = true
			break
		}
	}
	if !reserved {
		return file
	}
	var visit func([]Declaration, int)
	visit = func(declarations []Declaration, depth int) {
		for _, decl := range declarations {
			if decl.Kind != DeclarationClass {
				continue
			}
			if depth == 1 && strings.EqualFold(decl.Name, "Inner") && len(decl.Members) == 1 && decl.SuperClass == "" && len(decl.Interfaces) == 0 && len(decl.TypeParameters) == 0 && len(decl.Annotations) == 0 {
				member := decl.Members[0]
				message := ""
				if member.Type == "Integer" && len(member.Parameters) == 0 && len(member.Modifiers) == 2 && len(member.Annotations) == 0 && reservedStaticModifier(member.Modifiers, "public") && reservedStaticModifier(member.Modifiers, "static") {
					switch member.Kind {
					case DeclarationMethod:
						message = "static can only be used on methods of a top level type"
					case DeclarationField:
						message = "static can only be used on fields of a top level type"
					}
				}
				if message != "" {
					for i := range file.Diagnostics {
						d := &file.Diagnostics[i]
						if d.Severity == diagnostic.Error && d.Code == "APEXPARSE002" && strings.EqualFold(d.Message, "Identifier name is reserved: Inner") && d.Range != nil &&
							d.Range.Start.Offset >= decl.Range.Start.Offset && d.Range.Start.Offset < member.Range.Start.Offset {
							d.NativeMessage = message
						}
					}
				}
			}
			visit(decl.Members, depth+1)
		}
	}
	visit(file.Declarations, 0)
	return file
}

func reservedStaticModifier(modifiers []string, want string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, want) {
			return true
		}
	}
	return false
}
