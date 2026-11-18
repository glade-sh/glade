package vm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlang"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	commonSObjectTypeNamesOnce        sync.Once
	commonSObjectTypeNameSetOnce      sync.Once
	commonSObjectTypeNameFoldOnce     sync.Once
	commonSObjectTypeNameFoldSet      map[string]struct{}
	generatedPlatformTypeIndexCache   typesys.PlatformViewCache[*generatedPlatformTypeIndex]
	generatedPlatformMethodIndexCache typesys.PlatformViewCache[*generatedPlatformMethodIndex]
	// generatedPlatformMethodIndexOverride replaces the method index in tests.
	generatedPlatformMethodIndexOverride map[string]map[string][]Method
)

var (
	systemNamespaceClassNamesOnce sync.Once
	systemNamespaceClassNames     map[string]string
)

// systemNamespaceClassName is the canonical spelling of a System-namespace
// class or interface other than a primitive, Object, SObject or a collection.
func systemNamespaceClassName(name string) (string, bool) {
	systemNamespaceClassNamesOnce.Do(func() {
		systemNamespaceClassNames = make(map[string]string)
		for _, typeName := range typesys.StandardSystemNamespaceTypeNames() {
			systemNamespaceClassNames[strings.ToLower(typeName)] = typeName
		}
		for _, excluded := range []string{"blob", "boolean", "date", "datetime", "decimal", "double", "id", "integer", "long", "object", "sobject", "string", "time", "list", "set", "map", "void"} {
			delete(systemNamespaceClassNames, excluded)
		}
		// Native NS352B/NS385D: the URL class reads System.Url in cast text.
		systemNamespaceClassNames["url"] = "Url"
	})
	canonical, ok := systemNamespaceClassNames[apexlang.LowerName(strings.TrimSpace(name))]
	return canonical, ok
}

func CommonSObjectTypeNames() []string {
	commonSObjectTypeNamesOnce.Do(func() {
		commonSObjectTypeNames = buildCommonSObjectTypeNames()
	})
	return commonSObjectTypeNames
}

// IsCommonSObjectTypeName reports whether name matches an entry of
// CommonSObjectTypeNames case-insensitively. It is the constant-time
// equivalent of folding name against every entry of that slice.
func IsCommonSObjectTypeName(name string) bool {
	commonSObjectTypeNameFoldOnce.Do(func() {
		names := CommonSObjectTypeNames()
		commonSObjectTypeNameFoldSet = make(map[string]struct{}, len(names))
		for _, objectName := range names {
			commonSObjectTypeNameFoldSet[strings.ToLower(objectName)] = struct{}{}
		}
	})
	_, ok := commonSObjectTypeNameFoldSet[apexlang.LowerName(name)]
	return ok
}

func commonSObjectTypeNameLookup() map[string]bool {
	commonSObjectTypeNameSetOnce.Do(func() {
		commonSObjectTypeNameSet = make(map[string]bool, len(standardSObjectPrefixes))
		for _, objectName := range standardSObjectPrefixes {
			if objectName = strings.TrimSpace(objectName); objectName != "" {
				commonSObjectTypeNameSet[strings.ToLower(objectName)] = true
			}
		}
	})
	return commonSObjectTypeNameSet
}

// generatedPlatformTypeIndex maps a lower-cased runtime name to the position
// of its symbol in the standard platform view. A type's fields and
// constructors are built on the first lookup, so a process holds only the
// platform types it touches. Positions follow the eager build: a later symbol
// with the same runtime name replaces an earlier one.
type generatedPlatformTypeIndex struct {
	view  []typesys.TypeSymbol
	slots map[string]*generatedPlatformTypeSlot
}

type generatedPlatformTypeSlot struct {
	position int
	once     sync.Once
	value    generatedPlatformType
}

// lookupGeneratedPlatformType returns the generated platform type for a
// lower-cased runtime name. The result is shared and must not be mutated.
func lookupGeneratedPlatformType(key string) (generatedPlatformType, bool) {
	index := generatedPlatformTypeIndexCache.Get(buildGeneratedPlatformTypeIndex)
	slot, ok := index.slots[key]
	if !ok {
		return generatedPlatformType{}, false
	}
	slot.once.Do(func() {
		typ := index.view[slot.position]
		slot.value = buildGeneratedPlatformType(typ, generatedPlatformRuntimeName(typ))
	})
	return slot.value, true
}

// generatedPlatformMethodIndex maps a lower-cased runtime class name to the
// positions of every symbol that declares it. Methods are built per class on
// first lookup.
type generatedPlatformMethodIndex struct {
	view    []typesys.TypeSymbol
	classes map[string]*generatedPlatformMethodSlot
}

type generatedPlatformMethodSlot struct {
	positions []int
	once      sync.Once
	methods   map[string][]Method
}

// lookupGeneratedPlatformMethods returns the generated platform methods of a
// class by lower-cased method name, or nil when the class has none.
func lookupGeneratedPlatformMethods(classKey string) map[string][]Method {
	if override := generatedPlatformMethodIndexOverride; override != nil {
		return override[classKey]
	}
	index := generatedPlatformMethodIndexCache.Get(buildGeneratedPlatformMethodIndex)
	slot, ok := index.classes[classKey]
	if !ok {
		return nil
	}
	slot.once.Do(func() {
		var out map[string][]Method
		for _, position := range slot.positions {
			typ := index.view[position]
			className := generatedPlatformRuntimeName(typ)
			for _, member := range typ.Members {
				if member.Kind != apexast.DeclarationMethod {
					continue
				}
				method := generatedPlatformRuntimeMethod(className, member)
				if method.Name == "" {
					continue
				}
				if out == nil {
					out = make(map[string][]Method)
				}
				methodKey := strings.ToLower(member.Name)
				out[methodKey] = append(out[methodKey], method)
			}
		}
		slot.methods = out
	})
	return slot.methods
}

func warmGeneratedPlatformRuntimeIndexes() {
	_ = CommonSObjectTypeNames()
	_ = generatedPlatformTypeIndexCache.Get(buildGeneratedPlatformTypeIndex)
	_ = generatedPlatformMethodIndexCache.Get(buildGeneratedPlatformMethodIndex)
}

// PrewarmPlatformIndexes builds the shared platform indexes. The type and method
// indexes are independent once the symbol view exists, so they are built
// concurrently. Each index is still built exactly once behind its sync.Once.
// It allocates on every call, so newVM keeps using the serial warm.
func PrewarmPlatformIndexes() {
	_ = typesys.StandardPlatformSymbolView()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = generatedPlatformMethodIndexCache.Get(buildGeneratedPlatformMethodIndex)
	}()
	go func() {
		defer wg.Done()
		_ = CommonSObjectTypeNames()
	}()
	_ = generatedPlatformTypeIndexCache.Get(buildGeneratedPlatformTypeIndex)
	wg.Wait()
}

func buildGeneratedPlatformTypeIndex(view []typesys.TypeSymbol) *generatedPlatformTypeIndex {
	index := &generatedPlatformTypeIndex{view: view, slots: make(map[string]*generatedPlatformTypeSlot)}
	for position, typ := range view {
		name := generatedPlatformRuntimeName(typ)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if slot, ok := index.slots[key]; ok {
			slot.position = position
			continue
		}
		index.slots[key] = &generatedPlatformTypeSlot{position: position}
	}
	return index
}

func buildGeneratedPlatformType(typ typesys.TypeSymbol, name string) generatedPlatformType {
	generated := generatedPlatformType{
		Name:         name,
		Kind:         typ.Kind,
		SuperClass:   typ.SuperClass,
		IsAbstract:   methodHasModifier(typ.Modifiers, "abstract"),
		EnumHashBase: typ.EnumHashBase,
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
	return generated
}

func buildGeneratedPlatformMethodIndex(view []typesys.TypeSymbol) *generatedPlatformMethodIndex {
	index := &generatedPlatformMethodIndex{view: view, classes: make(map[string]*generatedPlatformMethodSlot)}
	for position, typ := range view {
		className := generatedPlatformRuntimeName(typ)
		if className == "" {
			continue
		}
		classKey := strings.ToLower(className)
		slot := index.classes[classKey]
		if slot == nil {
			slot = &generatedPlatformMethodSlot{}
			index.classes[classKey] = slot
		}
		slot.positions = append(slot.positions, position)
	}
	return index
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
	// Time values from storage and JSON may carry the wire suffix Z. Keep
	// the same clock representation as constructors for equality and ordering.
	if typeName == "Time" {
		if normalized, err := parseTimeText(value); err == nil {
			value = normalized
		}
	}
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
