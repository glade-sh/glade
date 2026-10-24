package apextest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const dependencyMissingReason testreport.Reason = "dependency-missing"

func missingDependencyNames(index typesys.Index) []string {
	seen := map[string]bool{}
	for _, dep := range index.Dependencies {
		for _, diag := range dep.Diagnostics {
			if diag.Code == "dependency_missing" && dep.Namespace != "" {
				seen[dep.Namespace] = true
			}
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ExplainDependencyError adds resolver context only when an error names an
// unresolved, statically bound Apex reference. It does not claim which package
// owns that reference: an unloaded package exposes no symbol inventory.
// An explicit source is the anonymous body; named routes use captured file
// identities from the error stack and reject changed source generations.
func ExplainDependencyError(index typesys.Index, source string, err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	names := missingDependencyNames(index)
	if len(names) == 0 {
		return err
	}
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) {
		switch runtimeErr.Type {
		case "", "RuntimeError", "UnsupportedFeature", "CompileError", "DmlException", "System.DmlException":
		default:
			return err
		}
	}
	message := err.Error()
	reference := dependencyErrorReference(message)
	if reference == "" || dependencyTypeKnown(index, reference) {
		return err
	}
	root, _, _ := strings.Cut(reference, ".")

	sources := map[string]string{}
	if source != "" {
		sources[""] = source
	}
	cache := newSourceCache()
	cache.expectedDigestLookup = index.SourceDigest
	cache.configureNamespaceRemaps(index.Types, index.Triggers)
	var stack []vm.StackFrame
	if runtimeErr != nil {
		stack = runtimeErr.Stack
		for _, frame := range stack {
			if frame.File == "" {
				continue
			}
			if _, known := index.SourceDigest(frame.File); !known {
				continue
			}
			if text, readErr := cache.read(frame.File); readErr == nil {
				sources[frame.File] = text
			}
		}
	}
	units := newCompileGateUnits(index)
	targets := compileGateUnits{fileOf: map[string]string{strings.ToLower(root): root}, types: units.types, namespace: units.namespace}
	explained := func() error {
		return &vm.RuntimeError{Type: "dependency_missing", Message: fmt.Sprintf("unresolved Apex reference %q; unloaded declared dependencies: %s; %s", reference, strings.Join(names, ", "), message), Stack: append([]vm.StackFrame(nil), stack...)}
	}
	shadowed := false
	for file, text := range sources {
		if len(compileGateReferences(file, text, targets)) > 0 {
			return explained()
		}
		if compileGateMentions(text, targets.fileOf)[strings.ToLower(root)] {
			shadowed = true
		}
	}
	// Trigger faults can reach the request boundary without a trigger frame.
	// Consult only installed trigger identities, after caller bindings have
	// ruled out a local receiver with the same spelling as the unresolved type.
	if !shadowed && len(sources) > 0 && runtimeErr != nil && runtimeErr.Type != "CompileError" {
		for _, trigger := range index.Triggers {
			if _, known := index.SourceDigest(trigger.File); !known {
				continue
			}
			if text, readErr := cache.read(trigger.File); readErr == nil && len(compileGateReferences(trigger.File, text, targets)) > 0 {
				return explained()
			}
		}
	}

	return err
}

func dependencyErrorReference(message string) string {
	for _, prefix := range []string{"Variable does not exist: ", "Invalid type: "} {
		if start := strings.Index(message, prefix); start >= 0 {
			tail := message[start+len(prefix):]
			end := 0
			for end < len(tail) && (compileGateIdentifierByte(tail[end]) || tail[end] == '.') {
				end++
			}
			return strings.TrimRight(tail[:end], ".")
		}
	}
	for _, pattern := range []struct {
		prefix string
		method bool
	}{{"calls unknown method ", true}, {"constructs unknown type ", false}, {"unsupported call ", true}} {
		if start := strings.Index(message, pattern.prefix); start >= 0 {
			quoted, err := strconv.QuotedPrefix(message[start+len(pattern.prefix):])
			if err != nil {
				return ""
			}
			name, err := strconv.Unquote(quoted)
			if err != nil {
				return ""
			}
			if !pattern.method {
				return name
			}
			if dot := strings.LastIndexByte(name, '.'); dot > 0 {
				return name[:dot]
			}
			return ""
		}
	}
	if end := strings.Index(message, " constructor does not accept arguments"); end >= 0 {
		fields := strings.Fields(message[:end])
		if len(fields) > 0 {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func dependencyTypeKnown(index typesys.Index, name string) bool {
	for _, typ := range index.Types {
		if strings.EqualFold(typ.Name, name) || (typ.Namespace != "" && strings.EqualFold(typ.Namespace+"."+typ.Name, name)) {
			return true
		}
	}
	for _, typ := range typesys.StandardPlatformSymbolView() {
		if strings.EqualFold(typ.Name, name) || strings.EqualFold(typ.Name, "System."+name) || strings.EqualFold(name, "System."+typ.Name) {
			return true
		}
	}
	objectName := name
	if qualifier, local, qualified := strings.Cut(name, "."); qualified && strings.EqualFold(qualifier, "Schema") {
		objectName = local
	}
	for _, object := range index.Objects {
		if strings.EqualFold(object.Name, objectName) {
			return true
		}
	}
	_, standard := storage.ResolveKnownStandardObjectName(objectName)
	return standard
}

func explainDependencyRun(index typesys.Index, run *testreport.Run) {
	if len(missingDependencyNames(index)) == 0 {
		return
	}
	excluded := map[string]testreport.CompileExclusion{}
	for _, entry := range run.ExcludedClasses {
		excluded[strings.ToLower(entry.ClassName)] = entry
	}
	explain := func(problem *testreport.Problem, root testreport.CompileExclusion) {
		if problem == nil {
			return
		}
		original := &vm.RuntimeError{Type: problem.Type, Message: problem.Message}
		for _, frame := range problem.Stack {
			original.Stack = append(original.Stack, vm.StackFrame{Symbol: frame.Symbol, File: frame.File, Line: frame.Line, Column: frame.Column})
		}
		if root.File != "" {
			original.Stack = append(original.Stack, vm.StackFrame{File: root.File})
			for _, detail := range root.Problems {
				if !strings.Contains(original.Message, detail.Message) {
					original.Message += "\n" + detail.Message
				}
			}
		}
		var explained *vm.RuntimeError
		if err := ExplainDependencyError(index, "", original); err != original && errors.As(err, &explained) && explained.Type == "dependency_missing" {
			if problem.Detail == "" {
				problem.Detail = problem.Type + ": " + problem.Message
			}
			problem.Type, problem.Message = explained.Type, "dependency_missing: "+explained.Message
		}
	}
	for suiteIndex := range run.Suites {
		for caseIndex := range run.Suites[suiteIndex].Cases {
			result := &run.Suites[suiteIndex].Cases[caseIndex]
			if result.Status == testreport.StatusPass || result.Status == testreport.StatusSkipped {
				continue
			}
			entry := excluded[strings.ToLower(result.ClassName)]
			root := excluded[strings.ToLower(entry.Root)]
			explain(result.Problem, root)
			if result.Problem != nil && result.Problem.Type == "dependency_missing" {
				result.Reason = dependencyMissingReason
				if result.Status != testreport.StatusCompileError {
					result.Status = testreport.StatusRuntimeError
				}
			}
		}
	}
	for i := range run.ExcludedClasses {
		entry := &run.ExcludedClasses[i]
		root := excluded[strings.ToLower(entry.Root)]
		problem := &testreport.Problem{Type: "CompileError", Message: entry.Message, Stack: []testreport.StackFrame{{File: entry.File}}}
		explain(problem, root)
		if problem.Type == "dependency_missing" {
			entry.Message = problem.Message
		}
	}
}
