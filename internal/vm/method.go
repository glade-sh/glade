package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/ir"
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
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	vm.methodResolveCache = nil
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
