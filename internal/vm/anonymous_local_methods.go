package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

// anonymousLocalClass is the type that owns an anonymous block's local
// methods. Native reports it as "anon" (anonymous_local_methods_c4 duplicate:
// "Method already defined: d Integer anon.d() from the type anon").
const anonymousLocalClass = "anon"

type anonymousLocalScope struct {
	names  map[string]bool
	frame  map[string]Value
	types  map[string]string
	finals map[string]bool
}

// registerAnonymousLocalMethods registers the block's methods as static
// methods of anon. Their bodies read and write the block's top-level frame
// when a name is not local to the method (anonymous_local_methods_c4
// outer_var, counter, var_after, static_outer_var).
func (vm *VM) registerAnonymousLocalMethods(program ir.Program) (func(), error) {
	previousScope := vm.anonymousLocal
	_, hadClass := vm.Classes[anonymousLocalClass]
	scope := &anonymousLocalScope{names: map[string]bool{}, frame: vm.Globals, types: vm.VarTypes}
	class := Class{Name: anonymousLocalClass, APIVersion: program.APIVersion, Methods: map[string]Method{}}
	var methods []Method
	for _, local := range program.LocalMethods {
		params := make([]Param, 0, len(local.Params))
		for _, param := range local.Params {
			params = append(params, Param{Name: param.Name, Type: param.Type})
		}
		method := Method{
			Name:       anonymousLocalClass + "." + local.Name,
			ReturnType: local.ReturnType,
			Params:     params,
			Program:    ir.Program{Instructions: local.Body, Source: program.Source, APIVersion: program.APIVersion},
			ClassName:  anonymousLocalClass,
			IsStatic:   true,
			Access:     "public",
			Modifiers:  append([]string{"static"}, local.Modifiers...),
			APIVersion: program.APIVersion,
		}
		methods = append(methods, method)
		key := local.Name + "#"
		for _, param := range params {
			key += param.Type + ";"
		}
		class.Methods[key] = method
		scope.names[strings.ToLower(local.Name)] = true
	}
	if !hadClass {
		if err := vm.RegisterClass(class); err != nil {
			return nil, err
		}
	}
	for _, method := range methods {
		if err := vm.RegisterMethod(method); err != nil {
			return nil, err
		}
	}
	vm.anonymousLocal = scope
	return func() {
		for _, method := range methods {
			vm.unregisterMethod(method)
		}
		if !hadClass {
			delete(vm.Classes, anonymousLocalClass)
			vm.execNamespaceMemo = execNamespaceMemo{}
		}
		vm.anonymousLocal = previousScope
	}, nil
}

// anonymousLocalCallee maps an unqualified call in the anonymous block to its
// local method.
func (vm *VM) anonymousLocalCallee(callee string) (string, bool) {
	if vm.anonymousLocal == nil || strings.Contains(callee, ".") || !vm.anonymousLocal.names[strings.ToLower(callee)] {
		return "", false
	}
	if vm.currentClass != "" && !strings.EqualFold(vm.currentClass, anonymousLocalClass) {
		return "", false
	}
	return anonymousLocalClass + "." + callee, true
}

// anonymousOuterName finds a top-level variable of the block from inside one
// of its local methods.
func (vm *VM) anonymousOuterName(name string) (string, bool) {
	if vm.anonymousLocal == nil || !strings.EqualFold(vm.currentClass, anonymousLocalClass) || strings.Contains(name, ".") {
		return "", false
	}
	if _, ok := vm.anonymousLocal.frame[name]; ok {
		return name, true
	}
	for candidate := range vm.anonymousLocal.frame {
		if strings.EqualFold(candidate, name) {
			return candidate, true
		}
	}
	return "", false
}
