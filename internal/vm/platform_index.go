package vm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	commonSObjectTypeNamesOnce       sync.Once
	generatedPlatformTypeIndexOnce   sync.Once
	generatedPlatformMethodIndexOnce sync.Once
)

func CommonSObjectTypeNames() []string {
	if commonSObjectTypeNames != nil {
		return commonSObjectTypeNames
	}
	commonSObjectTypeNamesOnce.Do(func() {
		commonSObjectTypeNames = buildCommonSObjectTypeNames()
	})
	return commonSObjectTypeNames
}

func generatedPlatformTypes() map[string]generatedPlatformType {
	if generatedPlatformTypeIndex != nil {
		return generatedPlatformTypeIndex
	}
	generatedPlatformTypeIndexOnce.Do(func() {
		generatedPlatformTypeIndex = buildGeneratedPlatformTypeIndex()
	})
	return generatedPlatformTypeIndex
}

func generatedPlatformMethods() map[string]map[string][]Method {
	if generatedPlatformMethodIndex != nil {
		return generatedPlatformMethodIndex
	}
	generatedPlatformMethodIndexOnce.Do(func() {
		generatedPlatformMethodIndex = buildGeneratedPlatformMethodIndex()
	})
	return generatedPlatformMethodIndex
}

func buildGeneratedPlatformTypeIndex() map[string]generatedPlatformType {
	out := make(map[string]generatedPlatformType)
	for _, typ := range typesys.StandardPlatformSymbolView() {
		name := generatedPlatformRuntimeName(typ)
		if name == "" {
			continue
		}
		generated := generatedPlatformType{
			Name:         name,
			Kind:         typ.Kind,
			SuperClass:   typ.SuperClass,
			Fields:       make(map[string]Field),
			StaticFields: make(map[string]Field),
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationField, apexast.DeclarationProperty:
				field := Field{
					Name:      member.Name,
					Type:      member.Type,
					Static:    methodHasModifier(member.Modifiers, "static"),
					Access:    "global",
					Modifiers: append([]string(nil), member.Modifiers...),
					Property:  member.Kind == apexast.DeclarationProperty,
				}
				if field.Static {
					generated.StaticFields[field.Name] = field
					generated.StaticFieldOrder = append(generated.StaticFieldOrder, field.Name)
				} else {
					generated.Fields[field.Name] = field
					generated.FieldOrder = append(generated.FieldOrder, field.Name)
				}
			case apexast.DeclarationConstructor:
				generated.Constructors = append(generated.Constructors, generatedPlatformRuntimeConstructor(name, member))
			}
		}
		out[strings.ToLower(name)] = generated
	}
	return out
}
func buildGeneratedPlatformMethodIndex() map[string]map[string][]Method {
	out := make(map[string]map[string][]Method)
	for _, typ := range typesys.StandardPlatformSymbolView() {
		className := generatedPlatformRuntimeName(typ)
		if className == "" {
			continue
		}
		classKey := strings.ToLower(className)
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			method := generatedPlatformRuntimeMethod(className, member)
			if method.Name == "" {
				continue
			}
			methodKey := strings.ToLower(member.Name)
			if out[classKey] == nil {
				out[classKey] = make(map[string][]Method)
			}
			out[classKey][methodKey] = append(out[classKey][methodKey], method)
		}
	}
	return out
}
func generatedPlatformRuntimeConstructor(className string, member typesys.MemberSymbol) Method {
	params := make([]Param, 0, len(member.Parameters))
	for i, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = "arg" + strconv.Itoa(i)
		}
		params = append(params, Param{Name: name, Type: param.Type})
	}
	return Method{
		Name:          className + ".<init>",
		ClassName:     className,
		ReturnType:    "void",
		Params:        params,
		IsConstructor: true,
		Access:        "global",
		Modifiers:     []string{"passive-generated"},
	}
}
func generatedPlatformRuntimeName(typ typesys.TypeSymbol) string {
	if typ.Namespace == "" || strings.Contains(typ.Name, ".") {
		return typ.Name
	}
	return typ.Namespace + "." + typ.Name
}
func generatedPlatformRuntimeMethod(className string, member typesys.MemberSymbol) Method {
	params := make([]Param, 0, len(member.Parameters))
	for i, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = "arg" + strconv.Itoa(i)
		}
		params = append(params, Param{Name: name, Type: param.Type})
	}
	modifiers := []string{"passive-generated"}
	if methodHasModifier(member.Modifiers, "static") {
		modifiers = append(modifiers, "static")
	}
	return Method{
		Name:       className + "." + member.Name,
		ClassName:  className,
		ReturnType: member.Type,
		Params:     params,
		IsStatic:   methodHasModifier(member.Modifiers, "static"),
		Access:     "global",
		Modifiers:  modifiers,
	}
}
func buildCommonSObjectTypeNames() []string {
	knownStandardObjects := storage.KnownStandardObjectNames()
	names := make([]string, 0, len(standardSObjectPrefixes)+len(knownStandardObjects))
	seen := make(map[string]bool, len(standardSObjectPrefixes)+len(knownStandardObjects))
	for _, name := range standardSObjectPrefixes {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for _, name := range knownStandardObjects {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	return names
}
func platformScalar(typeName, value string) Value {
	out := Object(typeName)
	out.Fields["value"] = String(value)
	return out
}
func platformScalarText(value Value, typeName string) (string, error) {
	if value.Kind != ValueObject || value.Type != typeName {
		return "", fmt.Errorf("expected %s value", typeName)
	}
	raw, ok := value.Fields["value"]
	if !ok || raw.Kind != ValueString {
		return "", fmt.Errorf("%s value is missing scalar text", typeName)
	}
	return raw.Text, nil
}
