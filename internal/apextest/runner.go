package apextest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/automation"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sobject"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

type Options struct {
	Filter    string
	LimitMode vm.LimitMode
}

type TestCase struct {
	ClassName  string
	MethodName string
	File       string
	Range      diagnostic.Range
	Body       string
}

func Discover(index typesys.Index, opts Options) []TestCase {
	var out []TestCase
	filter := strings.ToLower(strings.TrimSpace(opts.Filter))
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		classIsTest := typ.IsTest
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			if !member.IsTest && !classIsTest {
				continue
			}
			if isTestSetup(member.Modifiers) {
				continue
			}
			testName := typ.Name + "." + member.Name
			if filter != "" && !strings.Contains(strings.ToLower(testName), filter) {
				continue
			}
			out = append(out, TestCase{
				ClassName:  typ.Name,
				MethodName: member.Name,
				File:       typ.File,
				Range:      member.Range,
			})
		}
	}
	return out
}

func Run(index typesys.Index, opts Options) testreport.Run {
	return RunContext(context.Background(), index, opts)
}

func RunContext(ctx context.Context, index typesys.Index, opts Options) testreport.Run {
	cases := Discover(index, opts)
	methods := compileProjectMethods(index)
	classes := compileProjectClasses(index, methods)
	setups, setupErrors := compileTestSetupMethods(index)
	triggers, triggerErrors := compileProjectTriggers(index)
	org := orgFromIndex(index)
	suites := make(map[string][]testreport.Case)
	setupOrgs := make(map[string]storage.OrgState)
	setupRunErrors := make(map[string]error)
	order := make([]string, 0)
	for _, testCase := range cases {
		if err := ctx.Err(); err != nil {
			if _, ok := suites[testCase.ClassName]; !ok {
				order = append(order, testCase.ClassName)
			}
			suites[testCase.ClassName] = append(suites[testCase.ClassName], canceledCase(testCase, err))
			continue
		}
		if _, ok := suites[testCase.ClassName]; !ok {
			order = append(order, testCase.ClassName)
			setupOrgs[testCase.ClassName], setupRunErrors[testCase.ClassName] = prepareTestSetupOrg(ctx, testCase.ClassName, methods, classes, setups[testCase.ClassName], setupErrors[testCase.ClassName], triggers, triggerErrors, org, opts)
		}
		suites[testCase.ClassName] = append(suites[testCase.ClassName], runCase(ctx, testCase, methods, classes, setupRunErrors[testCase.ClassName], triggers, triggerErrors, setupOrgs[testCase.ClassName], opts))
	}

	run := testreport.Run{Name: "oaer test"}
	for _, name := range order {
		run.Suites = append(run.Suites, testreport.Suite{Name: name, Cases: suites[name]})
	}
	return run
}

func prepareTestSetupOrg(ctx context.Context, className string, methods map[string]vm.Method, classes []vm.Class, setups []vm.Method, setupErr error, triggers []vm.Trigger, triggerErrors []error, org storage.OrgState, opts Options) (storage.OrgState, error) {
	setupOrg := org.Clone()
	if err := ctx.Err(); err != nil {
		return setupOrg, err
	}
	if setupErr != nil {
		return setupOrg, setupErr
	}
	if len(triggerErrors) > 0 {
		return setupOrg, triggerErrors[0]
	}
	if len(setups) == 0 {
		return setupOrg, nil
	}
	machine := vm.New(nil)
	if opts.LimitMode != "" {
		machine.SetLimitMode(opts.LimitMode)
	}
	machine.SetOrg(&setupOrg)
	machine.SetContext(ctx)
	machine.EnableTestContext()
	if err := registerRuntime(machine, methods, classes, setups, triggers); err != nil {
		return setupOrg, err
	}
	for _, setup := range setups {
		if err := ctx.Err(); err != nil {
			return setupOrg, err
		}
		program, err := vm.CompileAnonymous(setup.Name + "();")
		if err != nil {
			return setupOrg, err
		}
		if _, err := machine.Execute(program); err != nil {
			return setupOrg, err
		}
	}
	return setupOrg, nil
}

func runCase(ctx context.Context, testCase TestCase, methods map[string]vm.Method, classes []vm.Class, setupErr error, triggers []vm.Trigger, triggerErrors []error, org storage.OrgState, opts Options) testreport.Case {
	if err := ctx.Err(); err != nil {
		return canceledCase(testCase, err)
	}
	out := testreport.Case{
		ClassName:  testCase.ClassName,
		MethodName: testCase.MethodName,
		Status:     testreport.StatusPass,
	}
	source, err := os.ReadFile(testCase.File)
	if err != nil {
		out.Status = testreport.StatusCompileError
		out.Problem = problem("FileError", err.Error(), testCase)
		return out
	}
	if setupErr != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", setupErr.Error(), testCase)
		return out
	}
	if len(triggerErrors) > 0 {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", triggerErrors[0].Error(), testCase)
		return out
	}
	testMethod, err := compileProjectMethod(testCase.ClassName, testCase.MethodName, "void", []string{"static"}, testCase.File, testCase.Range, string(source))
	if err != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", err.Error(), testCase)
		return out
	}
	machine := vm.New(nil)
	if opts.LimitMode != "" {
		machine.SetLimitMode(opts.LimitMode)
	}
	machine.SetOrg(&org)
	machine.SetContext(ctx)
	if err := registerRuntime(machine, methods, classes, nil, triggers); err != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", err.Error(), testCase)
		return out
	}
	if err := machine.RegisterMethod(testMethod); err != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", err.Error(), testCase)
		return out
	}
	org = org.Clone()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.ResetStatics(); err != nil {
		out.Status = testreport.StatusFail
		out.Problem = problemFromError(err, testCase)
		return out
	}
	program, err := vm.CompileAnonymous(testMethod.Name + "();")
	if err != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", err.Error(), testCase)
		return out
	}
	if _, err := machine.Execute(program); err != nil {
		out.Status = testreport.StatusFail
		out.Problem = problemFromError(err, testCase)
		return out
	}
	return out
}

func canceledCase(testCase TestCase, err error) testreport.Case {
	return testreport.Case{
		ClassName:  testCase.ClassName,
		MethodName: testCase.MethodName,
		Status:     testreport.StatusUnsupported,
		Problem:    problem("Canceled", err.Error(), testCase),
	}
}

func registerRuntime(machine *vm.VM, methods map[string]vm.Method, classes []vm.Class, setups []vm.Method, triggers []vm.Trigger) error {
	for _, class := range classes {
		if err := machine.RegisterClass(class); err != nil {
			return err
		}
	}
	for _, trigger := range triggers {
		if err := machine.RegisterTrigger(trigger); err != nil {
			return err
		}
	}
	for _, setup := range setups {
		if err := machine.RegisterMethod(setup); err != nil {
			return err
		}
	}
	for _, method := range methods {
		if err := machine.RegisterMethod(method); err != nil {
			return err
		}
	}
	return nil
}

// RegisterProjectRuntime compiles project classes, methods, and triggers from an
// index and installs them into the VM. It is used by non-test runtimes that need
// the same supported Apex subset as the local test runner.
func RegisterProjectRuntime(machine *vm.VM, index typesys.Index) error {
	methods := compileProjectMethods(index)
	classes := compileProjectClasses(index, methods)
	triggers, triggerErrors := compileProjectTriggers(index)
	if len(triggerErrors) > 0 {
		return triggerErrors[0]
	}
	return registerRuntime(machine, methods, classes, nil, triggers)
}

func compileProjectClasses(index typesys.Index, methods map[string]vm.Method) []vm.Class {
	var out []vm.Class
	sources := make(map[string]string)
	knownTypes := knownTypeNames(index.Types)
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		source, ok := sources[typ.File]
		if !ok {
			data, err := os.ReadFile(typ.File)
			if err != nil {
				continue
			}
			source = string(data)
			sources[typ.File] = source
		}
		class := vm.Class{
			Name:         typ.Name,
			Namespace:    index.Project.Namespace,
			Access:       accessModifier(typ.Modifiers),
			IsAbstract:   hasModifier(typ.Modifiers, "abstract"),
			IsInterface:  typ.Kind == apexast.DeclarationInterface,
			IsTest:       typ.IsTest,
			Fields:       make(map[string]vm.Field),
			StaticFields: make(map[string]vm.Field),
			Methods:      make(map[string]vm.Method),
		}
		typeSource, _ := extractSourceRange(source, typ.Range)
		class.SuperClass = qualifyNestedTypeName(typ.Name, parseExtends(typeSource), knownTypes)
		class.Interfaces = qualifyNestedTypeNames(typ.Name, parseImplements(typeSource), knownTypes)
		if typ.Kind == apexast.DeclarationEnum {
			class.EnumValues = parseEnumValues(typeSource)
		}
		for _, method := range methods {
			if method.ClassName == typ.Name {
				class.Methods[methodShortName(method.Name)+methodParamKey(method.Params)] = method
			}
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationField, apexast.DeclarationProperty:
				field := vm.Field{
					Name:     member.Name,
					Type:     member.Type,
					Static:   hasModifier(member.Modifiers, "static"),
					Access:   accessModifier(member.Modifiers),
					Property: member.Kind == apexast.DeclarationProperty,
				}
				if member.Kind == apexast.DeclarationProperty {
					attachPropertyAccessors(&field, typ.Name, typ.File, member, source)
				}
				if initializer, ok := compileFieldInitializerMethod(typ.Name, field.Name, field.Static, typ.File, member.Range, source); ok {
					if field.Static {
						class.StaticInitializers = append(class.StaticInitializers, initializer)
					} else {
						class.InstanceInitializers = append(class.InstanceInitializers, initializer)
					}
				} else if value, ok := compileFieldInitializer(member.Type, member.Range, source); ok {
					field.Value = value
					field.InitialValue = value
				}
				if field.Static {
					class.StaticFields[field.Name] = field
					class.StaticFieldOrder = append(class.StaticFieldOrder, field.Name)
				} else {
					class.Fields[field.Name] = field
					class.FieldOrder = append(class.FieldOrder, field.Name)
				}
			case apexast.DeclarationConstructor:
				ctor, err := compileProjectConstructor(typ.Name, typ.File, member.Range, source)
				if err == nil {
					class.Constructors = append(class.Constructors, ctor)
				}
			case apexast.DeclarationInitializer:
				init, err := compileProjectInitializer(typ.Name, typ.File, member.Range, source, hasModifier(member.Modifiers, "static"))
				if err == nil {
					if init.IsStatic {
						class.StaticInitializers = append(class.StaticInitializers, init)
					} else {
						class.InstanceInitializers = append(class.InstanceInitializers, init)
					}
				}
			}
		}
		out = append(out, class)
	}
	return out
}

func knownTypeNames(types []typesys.TypeSymbol) map[string]bool {
	out := make(map[string]bool, len(types))
	for _, typ := range types {
		out[typ.Name] = true
	}
	return out
}

func qualifyNestedTypeNames(owner string, names []string, known map[string]bool) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, qualifyNestedTypeName(owner, name, known))
	}
	return out
}

func qualifyNestedTypeName(owner, name string, known map[string]bool) string {
	if name == "" || known[name] || strings.Contains(name, ".") {
		return name
	}
	for {
		dot := strings.LastIndex(owner, ".")
		if dot < 0 {
			return name
		}
		owner = owner[:dot]
		candidate := owner + "." + name
		if known[candidate] {
			return candidate
		}
	}
}

func attachPropertyAccessors(field *vm.Field, className, file string, member typesys.MemberSymbol, source string) {
	for _, accessor := range member.Accessors {
		if !accessor.HasBody {
			continue
		}
		method, err := compilePropertyAccessor(className, file, member, accessor, source)
		if err != nil {
			continue
		}
		switch accessor.Kind {
		case "get":
			field.Getter = &method
		case "set":
			field.Setter = &method
		}
	}
}

func compileProjectMethods(index typesys.Index) map[string]vm.Method {
	out := make(map[string]vm.Method)
	sources := make(map[string]string)
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod || member.IsTest || isTestSetup(member.Modifiers) {
				continue
			}
			source, ok := sources[typ.File]
			if !ok {
				data, err := os.ReadFile(typ.File)
				if err != nil {
					continue
				}
				source = string(data)
				sources[typ.File] = source
			}
			method, err := compileProjectMethod(typ.Name, member.Name, member.Type, member.Modifiers, typ.File, member.Range, source)
			if err != nil {
				continue
			}
			out[method.Name+methodParamKey(method.Params)] = method
		}
	}
	return out
}

func compileTestSetupMethods(index typesys.Index) (map[string][]vm.Method, map[string]error) {
	out := make(map[string][]vm.Method)
	errs := make(map[string]error)
	sources := make(map[string]string)
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod || !isTestSetup(member.Modifiers) {
				continue
			}
			source, ok := sources[typ.File]
			if !ok {
				data, err := os.ReadFile(typ.File)
				if err != nil {
					errs[typ.Name] = err
					continue
				}
				source = string(data)
				sources[typ.File] = source
			}
			method, err := compileProjectMethod(typ.Name, member.Name, member.Type, member.Modifiers, typ.File, member.Range, source)
			if err != nil {
				errs[typ.Name] = err
				continue
			}
			method.IsStatic = true
			out[typ.Name] = append(out[typ.Name], method)
		}
	}
	return out, errs
}

func compileProjectTriggers(index typesys.Index) ([]vm.Trigger, []error) {
	var out []vm.Trigger
	var errs []error
	for _, trigger := range index.Triggers {
		sourceData, err := os.ReadFile(trigger.File)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		body, err := extractMethodBody(string(sourceData), trigger.Range)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		program, err := vm.CompileAnonymous(body)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, event := range trigger.Events {
			timing, op := triggerEventParts(event)
			if timing == "" || op == "" {
				continue
			}
			out = append(out, vm.Trigger{
				Name:      trigger.Name,
				Object:    trigger.ObjectName,
				Timing:    timing,
				Operation: op,
				Program:   program,
				File:      trigger.File,
				Line:      trigger.Range.Start.Line,
				Column:    trigger.Range.Start.Column,
			})
		}
	}
	return out, errs
}

func orgFromIndex(index typesys.Index) storage.OrgState {
	org := storage.NewOrgState()
	org.Namespace = index.Project.Namespace
	registry := sobject.BuildDescribeRegistry(schemaFromIndex(index))
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	if index.Project.Root != "" {
		if p, err := project.Load(index.Project.Root); err == nil {
			if automationIndex, err := automation.LoadProject(p); err == nil {
				automation.ApplyToOrg(&org, automationIndex)
			}
		}
	}
	return org
}

func schemaFromIndex(index typesys.Index) schema.Schema {
	return schema.Schema{Objects: append([]schema.Object(nil), index.Objects...)}
}

func triggerEventParts(event string) (string, string) {
	event = strings.ToLower(strings.ReplaceAll(event, " ", ""))
	for _, timing := range []string{"before", "after"} {
		if strings.HasPrefix(event, timing) {
			return timing, strings.TrimPrefix(event, timing)
		}
	}
	return "", ""
}

func compileProjectMethod(className, methodName, returnType string, modifiers []string, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:       className + "." + methodName,
		ReturnType: returnType,
		Params:     params,
		Program:    program,
		ClassName:  className,
		IsStatic:   hasModifier(modifiers, "static"),
		Access:     accessModifier(modifiers),
		Modifiers:  modifiers,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
}

func compileProjectConstructor(className, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:          className + ".<init>",
		ReturnType:    "void",
		Params:        params,
		Program:       program,
		ClassName:     className,
		IsConstructor: true,
		File:          file,
		Line:          r.Start.Line,
		Column:        r.Start.Column,
	}, nil
}

func compileProjectInitializer(className, file string, r diagnostic.Range, source string, static bool) (vm.Method, error) {
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	name := className + ".<init_block>"
	if static {
		name = className + ".<static_init>"
	}
	return vm.Method{
		Name:       name,
		ReturnType: "void",
		Program:    program,
		ClassName:  className,
		IsStatic:   static,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
}

func compilePropertyAccessor(className, file string, member typesys.MemberSymbol, accessor apexast.Accessor, source string) (vm.Method, error) {
	body, err := extractMethodBody(source, accessor.Range)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	method := vm.Method{
		Name:       className + "." + member.Name + "." + accessor.Kind,
		ReturnType: "void",
		Program:    program,
		ClassName:  className,
		IsStatic:   hasModifier(member.Modifiers, "static"),
		Access:     accessModifier(accessor.Modifiers),
		Modifiers:  accessor.Modifiers,
		File:       file,
		Line:       accessor.Range.Start.Line,
		Column:     accessor.Range.Start.Column,
	}
	if accessor.Kind == "get" {
		method.ReturnType = member.Type
	}
	if accessor.Kind == "set" {
		method.Params = []vm.Param{{Name: "value", Type: member.Type}}
	}
	return method, nil
}

func extractMethodSource(source string, r diagnostic.Range) (string, error) {
	return extractSourceRange(source, r)
}

func extractSourceRange(source string, r diagnostic.Range) (string, error) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", fmt.Errorf("source range is unavailable")
	}
	return source[start:end], nil
}

func compileFieldInitializer(typeName string, r diagnostic.Range, source string) (vm.Value, bool) {
	expr, ok := fieldInitializerExpr(r, source)
	if !ok {
		return vm.Value{}, false
	}
	if expr == "" {
		return vm.Value{}, false
	}
	program, err := vm.CompileAnonymous(typeName + " __field = " + expr + ";")
	if err != nil {
		return vm.Value{}, false
	}
	machine := vm.New(nil)
	result, err := machine.Execute(program)
	if err != nil {
		return vm.Value{}, false
	}
	value, ok := result.Vars["__field"]
	return value, ok
}

func compileFieldInitializerMethod(className, fieldName string, static bool, file string, r diagnostic.Range, source string) (vm.Method, bool) {
	expr, ok := fieldInitializerExpr(r, source)
	if !ok || expr == "" {
		return vm.Method{}, false
	}
	program, err := vm.CompileAnonymous(fieldName + " = " + expr + ";")
	if err != nil {
		return vm.Method{}, false
	}
	name := className + ".<field_init>." + fieldName
	if static {
		name = className + ".<static_field_init>." + fieldName
	}
	return vm.Method{
		Name:       name,
		ReturnType: "void",
		Program:    program,
		ClassName:  className,
		IsStatic:   static,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, true
}

func fieldInitializerExpr(r diagnostic.Range, source string) (string, bool) {
	fieldSource, err := extractSourceRange(source, r)
	if err != nil {
		return "", false
	}
	eq := strings.IndexByte(fieldSource, '=')
	if eq < 0 {
		return "", false
	}
	expr := strings.TrimSpace(fieldSource[eq+1:])
	expr = strings.TrimRight(expr, ";,")
	return strings.TrimSpace(expr), true
}

func methodShortName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func methodParamKey(params []vm.Param) string {
	var b strings.Builder
	b.WriteString("#")
	for _, param := range params {
		b.WriteString(param.Type)
		b.WriteString(";")
	}
	return b.String()
}

func accessModifier(modifiers []string) string {
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "public", "private", "protected", "global", "webservice":
			return strings.ToLower(modifier)
		}
	}
	return ""
}

func parseExtends(typeSource string) string {
	header := typeHeader(typeSource)
	fields := strings.FieldsFunc(header, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ','
	})
	for i, field := range fields {
		if strings.EqualFold(field, "extends") && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

func parseImplements(typeSource string) []string {
	header := typeHeader(typeSource)
	i := strings.Index(strings.ToLower(header), "implements")
	if i < 0 {
		return nil
	}
	raw := strings.TrimSpace(header[i+len("implements"):])
	raw = strings.TrimSuffix(raw, "{")
	var out []string
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func parseEnumValues(typeSource string) []string {
	open := strings.IndexByte(typeSource, '{')
	close := strings.LastIndexByte(typeSource, '}')
	if open < 0 || close <= open {
		return nil
	}
	body := typeSource[open+1 : close]
	if semi := strings.IndexByte(body, ';'); semi >= 0 {
		body = body[:semi]
	}
	var out []string
	for _, part := range strings.Split(body, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func typeHeader(typeSource string) string {
	if i := strings.IndexByte(typeSource, '{'); i >= 0 {
		return typeSource[:i]
	}
	return typeSource
}

func parseParams(methodSource string) ([]vm.Param, error) {
	open := strings.IndexByte(methodSource, '(')
	if open < 0 {
		return nil, fmt.Errorf("method parameter list is unavailable")
	}
	close := findMatchingParen(methodSource, open)
	if close < 0 {
		return nil, fmt.Errorf("method parameter list is incomplete")
	}
	raw := strings.TrimSpace(methodSource[open+1 : close])
	if raw == "" {
		return nil, nil
	}
	parts := splitTopLevelCommas(raw)
	params := make([]vm.Param, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		filtered := fields[:0]
		for _, field := range fields {
			if strings.EqualFold(field, "final") {
				continue
			}
			filtered = append(filtered, field)
		}
		if len(filtered) < 2 {
			return nil, fmt.Errorf("unsupported parameter %q", part)
		}
		params = append(params, vm.Param{
			Type: strings.Join(filtered[:len(filtered)-1], " "),
			Name: filtered[len(filtered)-1],
		})
	}
	return params, nil
}

func findMatchingParen(source string, open int) int {
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '\'':
			i = skipApexString(source, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitTopLevelCommas(raw string) []string {
	var parts []string
	start := 0
	angleDepth := 0
	for i, r := range raw {
		switch r {
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case ',':
			if angleDepth == 0 {
				parts = append(parts, raw[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, raw[start:])
	return parts
}

func extractMethodBody(source string, r diagnostic.Range) (string, error) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", fmt.Errorf("method source range is unavailable")
	}
	text := source[start:end]
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "", fmt.Errorf("test method has no executable body")
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipApexString(text, i)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				bodyStart := start + open + 1
				return sourcePositionPrefix(source[:bodyStart]) + text[open+1:i], nil
			}
		}
	}
	return "", fmt.Errorf("test method body is incomplete")
}

func sourcePositionPrefix(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	for _, r := range source {
		if r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
	}
	return b.String()
}

func skipApexString(source string, start int) int {
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				continue
			}
			return i
		}
	}
	return len(source) - 1
}

func isTestSetup(modifiers []string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), "TestSetup") {
			return true
		}
	}
	return false
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
			return true
		}
	}
	return false
}

func problem(kind, message string, testCase TestCase) *testreport.Problem {
	return &testreport.Problem{
		Type:    kind,
		Message: message,
		Stack: []testreport.StackFrame{{
			Symbol: testCase.ClassName + "." + testCase.MethodName,
			File:   testCase.File,
			Line:   testCase.Range.Start.Line,
			Column: testCase.Range.Start.Column,
		}},
	}
}

func problemFromError(err error, testCase TestCase) *testreport.Problem {
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) {
		stack := make([]testreport.StackFrame, 0, len(runtimeErr.Stack))
		for _, frame := range runtimeErr.Stack {
			stack = append(stack, testreport.StackFrame{
				Symbol: frame.Symbol,
				File:   frame.File,
				Line:   frame.Line,
				Column: frame.Column,
			})
		}
		if len(stack) == 0 {
			stack = problem("RuntimeError", err.Error(), testCase).Stack
		}
		kind := runtimeErr.Type
		if kind == "" {
			kind = "RuntimeError"
		}
		return &testreport.Problem{
			Type:    kind,
			Message: runtimeErr.Message,
			Stack:   stack,
		}
	}
	return problem("RuntimeError", err.Error(), testCase)
}
