package visualforce

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var visualforceControllerMethodName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Native controller compile captures report a missing Apex class on the page when
// supported controller compiler checks reject it. Unrelated Apex diagnostics
// remain outside this compile gate.
func rejectedVisualforceControllers(p project.Project) (map[string]bool, error) {
	objects, err := schema.LoadProject(p)
	if err != nil {
		return nil, err
	}
	index := typesys.Build(p, objects)
	analysis := sema.AnalyzeWithOptions(index, sema.AnalyzeOptions{Diagnostics: true, SuppressPerformanceDiagnostics: true})
	files := map[string]bool{}
	syntaxFiles := map[string]bool{}
	for _, diag := range analysis.Diagnostics {
		if sema.IsVisualforceControllerDiagnostic(diag) || isRemotingControllerDiagnostic(index, diag) {
			files[filepath.Clean(diag.File)] = true
			if diag.Code == "APEXPARSE001" {
				syntaxFiles[filepath.Clean(diag.File)] = true
			}
		}
	}
	rejected := map[string]bool{}
	for _, typ := range index.Types {
		if !typ.Dependency && files[filepath.Clean(typ.File)] {
			rejected[strings.ToLower(typ.Name)] = true
			if typ.LocalName != "" {
				rejected[strings.ToLower(typ.LocalName)] = true
			}
		}
	}
	// A blocking parser diagnostic removes type symbols. Recover only the
	// measured rejected source's retained declaration names, not filename guesses.
	parser := apexast.NewParser()
	for _, path := range p.ApexFiles {
		if !syntaxFiles[filepath.Clean(path)] {
			continue
		}
		file, err := parser.ParseFile(path)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Declarations {
			if decl.Kind == apexast.DeclarationClass {
				rejected[strings.ToLower(decl.Name)] = true
			}
		}
	}
	return rejected, nil
}

func validateCompiledControllerDeclarations(root *MarkupNode, rejected map[string]bool) error {
	for _, name := range append([]string{strings.TrimSpace(root.Attribute("controller"))}, splitCSV(root.Attribute("extensions"))...) {
		if rejected[strings.ToLower(name)] {
			return fmt.Errorf("Apex class '%s' does not exist", name)
		}
	}
	return nil
}

func visualforceControllerRoot(tree *MarkupNode) *MarkupNode {
	if tree.Namespace == "apex" && tree.Name == "page" {
		return tree
	}
	for _, child := range tree.Children {
		if child.Namespace == "apex" && child.Name == "page" {
			return child
		}
	}
	return nil
}

// Diagnostic captures at API 59/67 distinguish controller constructors,
// extension constructors and action methods from ordinary property expressions.
// Source-free VM fixtures remain runtime-resolved, as with expression validation.
func validateControllerDeclarations(tree *MarkupNode, classes map[string]apexast.Declaration) error {
	if tree.Namespace != "apex" || tree.Name != "page" || len(classes) == 0 {
		return nil
	}
	controller := strings.TrimSpace(tree.Attribute("controller"))
	standard := strings.TrimSpace(tree.Attribute("standardcontroller"))
	if controller != "" && standard != "" {
		return fmt.Errorf("A custom and standard controller cannot be referenced in the same page.")
	}
	var candidates []apexast.Declaration
	if controller != "" {
		class, ok := classes[strings.ToLower(controller)]
		if !ok {
			return fmt.Errorf("Apex class '%s' does not exist", controller)
		}
		if err := visualforceControllerConstructor(class, "", classes); err != nil {
			return err
		}
		candidates = append(candidates, class)
	}
	argument := controller
	if standard != "" {
		argument = "ApexPages.StandardController"
		if strings.TrimSpace(tree.Attribute("recordsetvar")) != "" {
			argument = "ApexPages.StandardSetController"
		}
	}
	var extensions []apexast.Declaration
	for _, name := range splitCSV(tree.Attribute("extensions")) {
		class, ok := classes[strings.ToLower(name)]
		if !ok {
			return fmt.Errorf("Apex class '%s' does not exist", name)
		}
		if err := visualforceControllerConstructor(class, argument, classes); err != nil {
			return err
		}
		extensions = append(extensions, class)
	}
	candidates = append(extensions, candidates...)
	action := actionMethodName(tree.Attribute("action"))
	if !visualforceControllerMethodName.MatchString(action) || len(candidates) == 0 {
		return nil
	}
	for _, class := range candidates {
		if method, ok := visualforceDeclaredAction(class, action, classes, map[string]bool{}); ok {
			if !visualforceControllerMemberVisible(method) {
				return fmt.Errorf("Method is not visible: [%s].%s()", class.Name, method.Name)
			}
			return nil
		}
	}
	return fmt.Errorf("Unknown method '%s.%s()'", candidates[0].Name, action)
}

func visualforceControllerConstructor(class apexast.Declaration, argument string, classes map[string]apexast.Declaration) error {
	var constructors []apexast.Declaration
	for _, member := range class.Members {
		if member.Kind == apexast.DeclarationConstructor {
			constructors = append(constructors, member)
		}
	}
	if argument != "" && len(constructors) > 1 {
		// Native extension_overload_* controls select the most specific overload
		// before checking access, independently of declaration order.
		selected, ok, err := visualforceOverloadedExtensionConstructor(class, argument, classes)
		if err != nil {
			return err
		}
		if ok {
			if !visualforceControllerModifiersVisible(selected.Modifiers) {
				return fmt.Errorf("Constructor is not visible: [%s]<init>(%s)", class.Name, argument)
			}
			return nil
		}
	} else {
		for _, member := range constructors {
			matches := argument == "" && len(member.Parameters) == 0
			if argument != "" && len(member.Parameters) == 1 {
				matches = visualforceControllerArgumentMatches(argument, member.Parameters[0].Type, classes)
			}
			if !matches {
				continue
			}
			if !visualforceControllerMemberVisible(member) {
				return fmt.Errorf("Constructor is not visible: [%s]<init>(%s)", class.Name, argument)
			}
			return nil
		}
	}
	if argument == "" && len(constructors) == 0 {
		return nil
	}
	if argument != "" {
		argument += " controller"
	}
	return fmt.Errorf("Unknown constructor '%s.%s(%s)'", class.Name, class.Name, argument)
}

func visualforceOverloadedExtensionConstructor(class apexast.Declaration, argument string, classes map[string]apexast.Declaration) (vm.Method, bool, error) {
	machine := vm.New(nil)
	seen := map[string]bool{}
	for _, declaration := range classes {
		key := strings.ToLower(declaration.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		metadata := vm.Class{Name: declaration.Name, SuperClass: declaration.SuperClass, Interfaces: declaration.Interfaces}
		for _, member := range declaration.Members {
			if member.Kind != apexast.DeclarationConstructor {
				continue
			}
			constructor := vm.Method{Name: declaration.Name + ".<init>", ClassName: declaration.Name, Modifiers: member.Modifiers, IsConstructor: true}
			for _, parameter := range member.Parameters {
				constructor.Params = append(constructor.Params, vm.Param{Name: parameter.Name, Type: parameter.Type})
			}
			metadata.Constructors = append(metadata.Constructors, constructor)
		}
		if err := machine.RegisterClass(metadata); err != nil {
			return vm.Method{}, false, err
		}
	}
	selected, ok, ambiguous := machine.ResolveVisualforceControllerConstructor(class.Name, []vm.Value{vm.Object(argument)})
	return selected, ok && !ambiguous, nil
}

func visualforceControllerMemberVisible(member apexast.Declaration) bool {
	return visualforceControllerModifiersVisible(member.Modifiers)
}

func visualforceControllerModifiersVisible(modifiers []string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, "public") || strings.EqualFold(modifier, "global") {
			return true
		}
	}
	return false
}

func visualforceControllerArgumentMatches(argument, parameter string, classes map[string]apexast.Declaration) bool {
	if strings.EqualFold(parameter, "Object") {
		return true
	}
	seen := map[string]bool{}
	for argument != "" && !seen[strings.ToLower(argument)] {
		if strings.EqualFold(argument, parameter) {
			return true
		}
		seen[strings.ToLower(argument)] = true
		class, ok := classes[strings.ToLower(argument)]
		if !ok {
			break
		}
		for _, implemented := range class.Interfaces {
			if strings.EqualFold(implemented, parameter) {
				return true
			}
		}
		argument = class.SuperClass
	}
	return false
}

func visualforceDeclaredAction(class apexast.Declaration, name string, classes map[string]apexast.Declaration, seen map[string]bool) (apexast.Declaration, bool) {
	key := strings.ToLower(class.Name)
	if seen[key] {
		return apexast.Declaration{}, false
	}
	seen[key] = true
	for _, member := range class.Members {
		if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, name) && len(member.Parameters) == 0 {
			return member, true
		}
	}
	if parent, ok := classes[strings.ToLower(class.SuperClass)]; ok {
		return visualforceDeclaredAction(parent, name, classes, seen)
	}
	return apexast.Declaration{}, false
}
