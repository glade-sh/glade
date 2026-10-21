package apextest

import (
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Triggers are source compile units, just like classes. They
// enter the same bound-reference closure, but their names are not Apex types.
func compileGateAddTriggerUnits(units *compileGateUnits, index typesys.Index) {
	for _, trigger := range index.Triggers {
		if trigger.Dependency || trigger.File == "" || !trigger.HasSourceSnapshot() {
			continue
		}
		file := filepath.Clean(trigger.File)
		if _, ok := units.classes[file]; !ok {
			units.order = append(units.order, file)
		}
		units.classes[file] = append(units.classes[file], trigger.Name)
	}
}

func compileGateFilterTriggers(triggers []typesys.TriggerSymbol, excluded map[string]bool) []typesys.TriggerSymbol {
	out := make([]typesys.TriggerSymbol, 0, len(triggers))
	for _, trigger := range triggers {
		if !trigger.Dependency && trigger.HasSourceSnapshot() && excluded[filepath.Clean(trigger.File)] {
			continue
		}
		out = append(out, trigger)
	}
	return out
}

func compileGateTriggerDependencyMessage(index typesys.Index, file string, dependency testreport.CompileExclusion, sources *sourceCache) string {
	dependent := compileGateDependentPrefix + dependency.ClassName + " : " + dependency.Message
	for _, trigger := range index.Triggers {
		if filepath.Clean(trigger.File) != file {
			continue
		}
		source, err := sources.read(file)
		if err != nil {
			return dependent
		}
		// Never-saved trigger source resolves an invalid class as an unavailable
		// variable in a call qualifier and as an invalid type in a type position.
		if problem := compileGateTriggerReferenceProblem(file, source, dependency.ClassName); problem != "" {
			return problem
		}
	}
	return dependent
}

func compileGateTriggerReferenceProblem(file, source, className string) string {
	parsed := apexast.ParseSourceAST(file, source)
	var visit func(apexast.ASTNode) string
	visit = func(node apexast.ASTNode) string {
		if node.Kind == "type_identifier" {
			start, end := node.Range.Start.Offset, node.Range.End.Offset
			if start >= 0 && end <= len(source) && strings.EqualFold(source[start:end], className) {
				return "Invalid type: " + className
			}
		}
		if node.Kind == "method_invocation" && len(node.Children) > 2 && node.Children[0].Kind == "identifier" {
			qualifier := node.Children[0]
			start, end := qualifier.Range.Start.Offset, qualifier.Range.End.Offset
			if start >= 0 && end <= len(source) && strings.EqualFold(source[start:end], className) {
				return "Variable does not exist: " + className
			}
		}
		for _, child := range node.Children {
			if problem := visit(child); problem != "" {
				return problem
			}
		}
		return ""
	}
	for _, node := range parsed.Nodes {
		if problem := visit(node); problem != "" {
			return problem
		}
	}
	return ""
}
