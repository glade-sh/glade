package vm

import (
	"fmt"

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
}

func (vm *VM) RegisterMethod(method Method) error {
	if method.Name == "" {
		return fmt.Errorf("method name is required")
	}
	if vm.Methods == nil {
		vm.Methods = make(map[string]Method)
	}
	vm.Methods[method.Name] = method
	return nil
}

type Field struct {
	Name         string
	Type         string
	Static       bool
	Value        Value
	InitialValue Value
	Access       string
	Property     bool
}

type Class struct {
	Name         string
	Namespace    string
	SuperClass   string
	Interfaces   []string
	Fields       map[string]Field
	StaticFields map[string]Field
	Methods      map[string]Method
	Constructors []Method
	EnumValues   []string
	Access       string
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
	for name, field := range class.StaticFields {
		if field.InitialValue.Kind == "" {
			field.InitialValue = defaultValue(field.Type, field.Value)
		}
		field.Value = defaultValue(field.Type, field.InitialValue)
		class.StaticFields[name] = field
	}
	if class.Methods == nil {
		class.Methods = make(map[string]Method)
	}
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
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
	vm.Classes[class.Name] = class
	return nil
}
