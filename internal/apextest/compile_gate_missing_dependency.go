package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
)

// The gate starts before orgFromIndex loads project metadata. Capture the
// structured project once when a file-less dependency error needs attribution;
// retain it through subset reruns. A failed load leaves the error unattributed.
func compileGateLoadDependencyProject(root string, diagnostics []diagnostic.Diagnostic) *project.Project {
	if root == "" {
		return nil
	}
	for _, diag := range diagnostics {
		if diag.Severity != diagnostic.Error || diag.File != "" || diag.Code != "dependency_missing" {
			continue
		}
		p, err := project.Load(root)
		if err != nil {
			return nil
		}
		return &p
	}
	return nil
}

// Resolver and schema attribution are local gate policy. Namespace identity
// comes from structured dependency records, and schema identity comes from the
// diagnostic's owning object, never from parsing diagnostic prose.
func (g *compileGate) attributeDependencyDiagnostics(units compileGateUnits, diagnostics []diagnostic.Diagnostic, sources *sourceCache) (map[string][]diagnostic.Diagnostic, bool) {
	var attributed, handled, unmatched []diagnostic.Diagnostic
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error && diag.Code == "GLADESEMA003" && diag.SchemaObject != "" {
			start := len(attributed)
			for _, file := range units.order {
				source, err := sources.read(file)
				if err != nil {
					return nil, false
				}
				if compileGateReferencesSchemaObject(file, source, units, diag.SchemaObject) {
					local := diag
					local.File = file
					attributed = append(attributed, local)
				}
			}
			if len(attributed) == start {
				unmatched = append(unmatched, diag)
			}
			// Schema metadata remains in the subset index after its static users
			// are excluded, or its unmatched error is retained for the project
			// report. Retain the original identity for that rerun's filter.
			handled = append(handled, diag)
			continue
		}
		if diag.Severity != diagnostic.Error || diag.File != "" || diag.Code != "dependency_missing" {
			attributed = append(attributed, diag)
			continue
		}
		if g.dependencyProject == nil {
			return nil, false
		}
		var namespaces []string
		for _, dep := range g.dependencyProject.DependencyDiagnostics {
			if dep.Code == diag.Code && dep.Message == diag.Message && dep.Namespace != "" {
				namespaces = append(namespaces, dep.Namespace)
			}
		}
		if len(namespaces) == 0 {
			return nil, false
		}
		for _, file := range units.order {
			source, err := sources.read(file)
			if err != nil {
				return nil, false
			}
			if !compileGateReferencesMissingNamespace(file, source, units, namespaces) {
				continue
			}
			local := diag
			local.File = file
			attributed = append(attributed, local)
		}
		// Reanalysis of the subset still sees its configured dependency. That
		// diagnostic has already been attributed; do not gate unrelated units a
		// second time. All other errors retain their normal gate handling.
		handled = append(handled, diag)
	}
	byFile, ok := units.attribute(attributed)
	if ok {
		g.dependencyErrors = append(g.dependencyErrors, handled...)
		g.projectErrors = append(g.projectErrors, unmatched...)
	}
	return byFile, ok
}

func (g *compileGate) withoutScopedDependencyErrors(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if g == nil || len(g.dependencyErrors) == 0 {
		return diagnostics
	}
	out := make([]diagnostic.Diagnostic, 0, len(diagnostics))
	for _, diag := range diagnostics {
		handled := false
		if diag.File == "" || diag.Code == "GLADESEMA003" && diag.SchemaObject != "" {
			for _, prior := range g.dependencyErrors {
				if diag.Code == prior.Code && diag.Message == prior.Message && diag.Severity == prior.Severity && diag.File == prior.File && diag.SchemaObject == prior.SchemaObject {
					handled = true
					break
				}
			}
		}
		if !handled {
			out = append(out, diag)
		}
	}
	return out
}

func compileGateReferencesMissingNamespace(file, source string, units compileGateUnits, namespaces []string) bool {
	// Reuse the gate's variable/type binding, string/comment handling and query
	// bind handling. Synthetic target entries represent configured unavailable
	// namespaces; they do not turn dynamic lookup strings into dependencies.
	targets := compileGateUnits{fileOf: map[string]string{}, types: units.types, namespace: units.namespace}
	for _, namespace := range namespaces {
		name := strings.ToLower(namespace)
		if units.fileOf[name] == "" {
			targets.fileOf[name] = namespace
		}
	}
	parsed := apexast.ParseSourceAST(file, source)
	var collect func(apexast.ASTNode)
	collect = func(node apexast.ASTNode) {
		if node.Kind == "type_identifier" || node.Kind == "identifier" {
			start, end := node.Range.Start.Offset, node.Range.End.Offset
			if start >= 0 && end <= len(source) && start <= end {
				name := strings.ToLower(source[start:end])
				for _, namespace := range namespaces {
					// Managed SObject API names use the configured namespace separator.
					if strings.HasPrefix(name, strings.ToLower(namespace)+"__") && units.fileOf[name] == "" {
						targets.fileOf[name] = namespace
					}
				}
			}
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	for _, node := range parsed.Nodes {
		collect(node)
	}
	return len(compileGateReferences(file, source, targets)) > 0
}
