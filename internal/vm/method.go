package vm

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

type Param struct {
	Name string
	Type string
}

type Method struct {
	Name          string
	ReturnType    string
	Params        []Param
	Program       ir.Program
	ClassName     string
	IsStatic      bool
	IsConstructor bool
	Access        string
	Modifiers     []string
	File          string
	Line          int
	Column        int
	Unsupported   string
	Dependency    bool
}

func (vm *VM) RegisterMethod(method Method) error {
	if method.Name == "" {
		return fmt.Errorf("method name is required")
	}
	if vm.Methods == nil {
		vm.Methods = make(map[string]Method)
	}
	if vm.MethodOverloads == nil {
		vm.MethodOverloads = make(map[string][]Method)
	}
	if vm.MethodFolded == nil {
		vm.MethodFolded = make(map[string][]Method)
	}
	vm.Methods[method.Name] = method
	vm.MethodOverloads[method.Name] = append(vm.MethodOverloads[method.Name], method)
	foldedName := strings.ToLower(method.Name)
	vm.MethodFolded[foldedName] = append(vm.MethodFolded[foldedName], method)
	vm.methodCandidates = nil
	vm.methodResolveCache = nil
	return nil
}

func (vm *VM) unregisterMethod(method Method) {
	if method.Name == "" {
		return
	}
	remove := func(methods []Method) []Method {
		out := methods[:0]
		for _, candidate := range methods {
			if sameRegisteredMethod(candidate, method) {
				continue
			}
			out = append(out, candidate)
		}
		return out
	}
	if methods := remove(vm.MethodOverloads[method.Name]); len(methods) > 0 {
		vm.MethodOverloads[method.Name] = methods
		vm.Methods[method.Name] = methods[len(methods)-1]
	} else {
		delete(vm.MethodOverloads, method.Name)
		delete(vm.Methods, method.Name)
	}
	foldedName := strings.ToLower(method.Name)
	if methods := remove(vm.MethodFolded[foldedName]); len(methods) > 0 {
		vm.MethodFolded[foldedName] = methods
	} else {
		delete(vm.MethodFolded, foldedName)
	}
	vm.methodCandidates = nil
	vm.methodResolveCache = nil
}

func sameRegisteredMethod(left, right Method) bool {
	return strings.EqualFold(left.Name, right.Name) &&
		strings.EqualFold(left.ClassName, right.ClassName) &&
		left.IsStatic == right.IsStatic &&
		left.Dependency == right.Dependency &&
		sameMethodFile(left.File, right.File) &&
		methodSignature(left) == methodSignature(right)
}

type Field struct {
	Name         string
	Type         string
	Static       bool
	Value        Value
	InitialValue Value
	Access       string
	Modifiers    []string
	Property     bool
	Getter       *Method
	Setter       *Method
	File         string
	Dependency   bool
	StorageName  string
}

type Class struct {
	Name                 string
	Namespace            string
	SuperClass           string
	Interfaces           []string
	Fields               map[string]Field
	StaticFields         map[string]Field
	FieldOrder           []string
	StaticFieldOrder     []string
	Methods              map[string]Method
	Constructors         []Method
	StaticInitializers   []Method
	InstanceInitializers []Method
	EnumValues           []string
	Access               string
	Modifiers            []string
	IsAbstract           bool
	IsInterface          bool
	IsTest               bool
	Dependency           bool
}

func (vm *VM) RegisterClass(class Class) error {
	if class.Name == "" {
		return fmt.Errorf("class name is required")
	}
	if class.Fields == nil {
		class.Fields = make(map[string]Field)
	}
	if class.StaticFields == nil {
		class.StaticFields = make(map[string]Field)
	}
	class.FieldOrder = orderedFieldNames(class.Fields, class.FieldOrder)
	class.StaticFieldOrder = orderedFieldNames(class.StaticFields, class.StaticFieldOrder)
	for _, name := range class.StaticFieldOrder {
		field := class.StaticFields[name]
		if field.InitialValue.Kind == "" {
			field.InitialValue = defaultStaticFieldValue(class.Name, name, field.Type, field.Value)
		}
		field.Value = defaultStaticFieldValue(class.Name, name, field.Type, field.InitialValue)
		class.StaticFields[name] = field
	}
	if class.Methods == nil {
		class.Methods = make(map[string]Method)
	}
	stampClassProvenance(&class)
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	vm.methodResolveCache = nil
	vm.uniqueNestedTypeCache = nil
	existingClass, mergeWithExisting := vm.duplicateClassForMerge(class)
	if mergeWithExisting {
		vm.unregisterClassMethods(existingClass)
		class = mergeDuplicateClass(existingClass, class)
	}
	for name, method := range class.Methods {
		if method.Name == "" {
			method.Name = class.Name + "." + name
		}
		if method.ClassName == "" {
			method.ClassName = class.Name
		}
		class.Methods[name] = method
		if err := vm.RegisterMethod(method); err != nil {
			return err
		}
		if class.Namespace != "" && !strings.HasPrefix(method.Name, class.Namespace+".") {
			alias := method
			alias.Name = class.Namespace + "." + method.Name
			if class.Dependency && alias.ClassName != "" && !strings.HasPrefix(alias.ClassName, class.Namespace+".") {
				alias.ClassName = class.Namespace + "." + alias.ClassName
			}
			if err := vm.RegisterMethod(alias); err != nil {
				return err
			}
		}
	}
	for i := range class.Constructors {
		if class.Constructors[i].Name == "" {
			class.Constructors[i].Name = class.Name + ".<init>"
		}
		if class.Constructors[i].ClassName == "" {
			class.Constructors[i].ClassName = class.Name
		}
		class.Constructors[i].IsConstructor = true
	}
	vm.storeClassAliases(class)
	if vm.staticInitState == nil {
		vm.staticInitState = make(map[string]staticInitState)
	}
	vm.staticInitState[class.Name] = staticInitUninitialized
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		vm.staticInitState[class.Namespace+"."+class.Name] = staticInitUninitialized
	}
	return nil
}

func (vm *VM) unregisterClassMethods(class Class) {
	for _, method := range class.Methods {
		vm.unregisterMethod(method)
		if class.Namespace != "" && !strings.HasPrefix(method.Name, class.Namespace+".") {
			alias := method
			alias.Name = class.Namespace + "." + method.Name
			if class.Dependency && alias.ClassName != "" && !strings.HasPrefix(alias.ClassName, class.Namespace+".") {
				alias.ClassName = class.Namespace + "." + alias.ClassName
			}
			vm.unregisterMethod(alias)
		}
	}
}

func (vm *VM) duplicateClassForMerge(class Class) (Class, bool) {
	existing, ok := vm.Classes[class.Name]
	if !ok || existing.Name == "" {
		return Class{}, false
	}
	if !strings.EqualFold(existing.Name, class.Name) || !strings.EqualFold(existing.Namespace, class.Namespace) {
		return Class{}, false
	}
	return existing, true
}

func stampClassProvenance(class *Class) {
	if class == nil || !class.Dependency {
		return
	}
	for name, method := range class.Methods {
		method.Dependency = true
		class.Methods[name] = method
	}
	for name, field := range class.Fields {
		field.Dependency = true
		stampFieldAccessors(&field)
		class.Fields[name] = field
	}
	for name, field := range class.StaticFields {
		field.Dependency = true
		stampFieldAccessors(&field)
		class.StaticFields[name] = field
	}
	for i := range class.Constructors {
		class.Constructors[i].Dependency = true
	}
	for i := range class.StaticInitializers {
		class.StaticInitializers[i].Dependency = true
	}
	for i := range class.InstanceInitializers {
		class.InstanceInitializers[i].Dependency = true
	}
}

func stampFieldAccessors(field *Field) {
	if field == nil {
		return
	}
	if field.Getter != nil {
		field.Getter.Dependency = true
	}
	if field.Setter != nil {
		field.Setter.Dependency = true
	}
}

func mergeDuplicateClass(existing, incoming Class) Class {
	merged := existing
	preferIncoming := !incoming.Dependency || existing.Dependency
	if preferIncoming && incoming.Namespace != "" {
		merged.Namespace = incoming.Namespace
	}
	if preferIncoming && incoming.SuperClass != "" {
		merged.SuperClass = incoming.SuperClass
	}
	merged.Interfaces = mergeStrings(merged.Interfaces, incoming.Interfaces)
	if preferIncoming && incoming.Access != "" {
		merged.Access = incoming.Access
	}
	merged.Modifiers = mergeStrings(merged.Modifiers, incoming.Modifiers)
	if preferIncoming {
		merged.IsAbstract = incoming.IsAbstract
		merged.IsInterface = incoming.IsInterface
		merged.Dependency = incoming.Dependency
	}
	merged.IsTest = merged.IsTest || incoming.IsTest
	merged.EnumValues = mergeStrings(merged.EnumValues, incoming.EnumValues)
	merged.Fields = mergeFields(merged.Fields, incoming.Fields, preferIncoming)
	merged.StaticFields = mergeFields(merged.StaticFields, incoming.StaticFields, preferIncoming)
	merged.Methods = mergeMethods(merged.Methods, incoming.Methods, preferIncoming)
	merged.FieldOrder = mergeStrings(merged.FieldOrder, incoming.FieldOrder)
	merged.StaticFieldOrder = mergeStrings(merged.StaticFieldOrder, incoming.StaticFieldOrder)
	merged.Constructors = append(append([]Method(nil), merged.Constructors...), incoming.Constructors...)
	merged.StaticInitializers = dependencyMethodsFirst(append(append([]Method(nil), merged.StaticInitializers...), incoming.StaticInitializers...))
	merged.InstanceInitializers = dependencyMethodsFirst(append(append([]Method(nil), merged.InstanceInitializers...), incoming.InstanceInitializers...))
	if classExtendsItself(merged) {
		merged.SuperClass = ""
	}
	return merged
}

func classExtendsItself(class Class) bool {
	superClass := strings.TrimSpace(class.SuperClass)
	if superClass == "" {
		return false
	}
	if strings.EqualFold(superClass, class.Name) {
		return true
	}
	if class.Namespace != "" && strings.EqualFold(superClass, class.Namespace+"."+class.Name) {
		return true
	}
	return false
}

func dependencyMethodsFirst(methods []Method) []Method {
	sort.SliceStable(methods, func(i, j int) bool {
		return methods[i].Dependency && !methods[j].Dependency
	})
	return methods
}

func mergeFields(existing, incoming map[string]Field, preferIncoming bool) map[string]Field {
	out := make(map[string]Field, len(existing)+len(incoming))
	for name, field := range existing {
		out[name] = field
	}
	for name, field := range incoming {
		if existingField, exists := out[name]; exists {
			if duplicateFieldDefsDiffer(existingField, field) {
				if preferIncoming {
					out[name] = field
				} else {
					out[name] = existingField
				}
				out[duplicateFieldMapKey(name, existingField)] = existingField
				out[duplicateFieldMapKey(name, field)] = field
				continue
			}
			if !preferIncoming {
				continue
			}
			out[name] = field
			continue
		}
		out[name] = field
	}
	return out
}

func duplicateFieldDefsDiffer(existing, incoming Field) bool {
	return !strings.EqualFold(existing.Type, incoming.Type)
}

func duplicateFieldMapKey(base string, field Field) string {
	file := filepath.Clean(field.File)
	if file == "." {
		file = ""
	}
	return base + "\x00" + file + "\x00" + fmt.Sprint(field.Dependency)
}

func mergeMethods(existing, incoming map[string]Method, preferIncoming bool) map[string]Method {
	out := make(map[string]Method, len(existing)+len(incoming))
	for name, method := range existing {
		out[name] = method
	}
	for name, method := range incoming {
		if existingMethod, exists := out[name]; exists {
			if duplicateMethodBodiesDiffer(existingMethod, method) {
				delete(out, name)
				widest := widestMethodAccess(existingMethod.Access, method.Access)
				existingMethod.Access = widest
				method.Access = widest
				out[duplicateMethodMapKey(name, existingMethod)] = existingMethod
				out[duplicateMethodMapKey(name, method)] = method
				continue
			}
			out[name] = mergeMethod(existingMethod, method, preferIncoming)
			continue
		}
		out[name] = method
	}
	return out
}

func duplicateMethodBodiesDiffer(existing, incoming Method) bool {
	return !sameMethodFile(existing.File, incoming.File) || existing.Dependency != incoming.Dependency
}

func sameMethodFile(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func duplicateMethodMapKey(base string, method Method) string {
	file := filepath.Clean(method.File)
	if file == "." {
		file = ""
	}
	return base + "\x00" + file + "\x00" + fmt.Sprint(method.Dependency)
}

func mergeMethod(existing, incoming Method, preferIncoming bool) Method {
	merged := existing
	if preferIncoming {
		merged = incoming
	}
	merged.Access = widestMethodAccess(existing.Access, incoming.Access)
	return merged
}

func widestMethodAccess(left, right string) string {
	if accessRank(right) > accessRank(left) {
		return right
	}
	return left
}

func accessRank(access string) int {
	switch strings.ToLower(strings.TrimSpace(access)) {
	case "private":
		return 1
	case "protected":
		return 2
	case "global", "webservice":
		return 4
	default:
		return 3
	}
}

func mergeStrings(existing, incoming []string) []string {
	out := make([]string, 0, len(existing)+len(incoming))
	seen := make(map[string]bool, len(existing)+len(incoming))
	for _, value := range append(append([]string(nil), existing...), incoming...) {
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func orderedFieldNames(fields map[string]Field, order []string) []string {
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, name := range order {
		if _, ok := fields[name]; ok && !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
	}
	missing := make([]string, 0, len(fields)-len(out))
	for name := range fields {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	out = append(out, missing...)
	return out
}

func (vm *VM) runStaticInitializers(class Class) error {
	for _, initializer := range class.StaticInitializers {
		if initializer.Name == "" {
			initializer.Name = class.Name + ".<static_init>"
		}
		if initializer.ClassName == "" {
			initializer.ClassName = class.Name
		}
		initializer.IsStatic = true
		if _, err := vm.callMethod(initializer, nil, &Result{}); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) ensureClassInitialized(className string) error {
	class, ok := vm.Classes[className]
	if !ok {
		return nil
	}
	canonical := class.Name
	if vm.staticInitState == nil {
		vm.staticInitState = make(map[string]staticInitState)
	}
	switch vm.staticInitState[canonical] {
	case staticInitDone, staticInitRunning:
		return nil
	}
	vm.staticInitState[canonical] = staticInitRunning
	if class.SuperClass != "" {
		if err := vm.ensureClassInitialized(class.SuperClass); err != nil {
			vm.staticInitState[canonical] = staticInitUninitialized
			return err
		}
	}
	if err := vm.runStaticInitializers(class); err != nil {
		vm.staticInitState[canonical] = staticInitUninitialized
		return err
	}
	vm.staticInitState[canonical] = staticInitDone
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		vm.staticInitState[class.Namespace+"."+class.Name] = staticInitDone
	}
	return nil
}
