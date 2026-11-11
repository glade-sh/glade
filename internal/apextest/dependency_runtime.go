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

// flowActionMissingMarker prefixes the class name in the VM error for a flow
// action whose Apex class has no invocable method.
const flowActionMissingMarker = "no static @InvocableMethod found on "

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
	return newDependencyExplainer(index).explain(source, err)
}

// dependencyExplainer answers ExplainDependencyError for one index. A test run
// asks the same question for every excluded class and failing case, and those
// share files: the compile-gate units of the index are built once, and whether
// a file's text binds an unresolved root is remembered per (file, root) with
// the text it was answered for. Sources are still read and digest-checked per
// question through a fresh source cache.
type dependencyExplainer struct {
	index typesys.Index
	names []string
	units *compileGateUnits
	known map[string]bool
	refs  map[dependencyReferenceKey]dependencyReferenceAnswer
}

// typeKnown is dependencyTypeKnown remembered per reference: it scans the
// index and the platform symbol view, and a run asks it for the same few
// references many times.
func (e *dependencyExplainer) typeKnown(reference string) bool {
	if known, ok := e.known[reference]; ok {
		return known
	}
	known := dependencyTypeKnown(e.index, reference)
	if e.known == nil {
		e.known = make(map[string]bool)
	}
	e.known[reference] = known
	return known
}

type dependencyReferenceKey struct {
	file string
	root string
}

type dependencyReferenceAnswer struct {
	text       string
	referenced bool
	mentioned  bool
}

func newDependencyExplainer(index typesys.Index) *dependencyExplainer {
	return &dependencyExplainer{index: index, names: missingDependencyNames(index)}
}

func (e *dependencyExplainer) compileGateUnits() compileGateUnits {
	if e.units == nil {
		units := newCompileGateUnits(e.index)
		e.units = &units
	}
	return *e.units
}

// referenceAnswer reports whether text, read for file, binds root as a static
// Apex reference and, when it does not, whether it mentions root at all.
// compileGateReferences and compileGateMentions are deterministic in (file,
// text, targets), so an answer is reused only for identical text.
func (e *dependencyExplainer) referenceAnswer(file, text, root string, targets compileGateUnits) dependencyReferenceAnswer {
	key := dependencyReferenceKey{file: file, root: root}
	if answer, ok := e.refs[key]; ok && answer.text == text {
		return answer
	}
	answer := dependencyReferenceAnswer{text: text}
	answer.referenced = len(compileGateReferences(file, text, targets)) > 0
	if !answer.referenced {
		answer.mentioned = compileGateMentions(text, targets.fileOf)[strings.ToLower(root)]
	}
	if e.refs == nil {
		e.refs = make(map[dependencyReferenceKey]dependencyReferenceAnswer)
	}
	e.refs[key] = answer
	return answer
}

func (e *dependencyExplainer) explain(source string, err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	index := e.index
	names := e.names
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
	if reference == "" || e.typeKnown(reference) {
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
	units := e.compileGateUnits()
	targets := compileGateUnits{fileOf: map[string]string{strings.ToLower(root): root}, types: units.types, namespace: units.namespace}
	explained := func() error {
		return &vm.RuntimeError{Type: "dependency_missing", Message: fmt.Sprintf("unresolved Apex reference %q; unloaded declared dependencies: %s; %s", reference, strings.Join(names, ", "), message), Stack: append([]vm.StackFrame(nil), stack...)}
	}
	// A flow action names its class in flow metadata, not in Apex source, so
	// there is no source reference to confirm. An unknown class with unloaded
	// declared dependencies is the dependency_missing shape.
	if strings.Contains(message, flowActionMissingMarker) {
		return explained()
	}
	shadowed := false
	for file, text := range sources {
		answer := e.referenceAnswer(file, text, root, targets)
		if answer.referenced {
			return explained()
		}
		if answer.mentioned {
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
			if text, readErr := cache.read(trigger.File); readErr == nil && e.referenceAnswer(trigger.File, text, root, targets).referenced {
				return explained()
			}
		}
	}

	return err
}

func dependencyErrorReference(message string) string {
	if start := strings.Index(message, flowActionMissingMarker); start >= 0 {
		tail := message[start+len(flowActionMissingMarker):]
		end := 0
		for end < len(tail) && (compileGateIdentifierByte(tail[end]) || tail[end] == '.') {
			end++
		}
		return strings.TrimRight(tail[:end], ".")
	}
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
	explainer := newDependencyExplainer(index)
	if len(explainer.names) == 0 {
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
		if err := explainer.explain("", original); err != original && errors.As(err, &explained) && explained.Type == "dependency_missing" {
			if problem.Detail == "" {
				problem.Detail = problem.Type + ": " + problem.Message
			}
			problem.Type, problem.Message = explained.Type, "dependency_missing: "+explained.Message
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
	// A flow action names its class in flow metadata. When the compile gate
	// excluded that class because it depends on an unloaded declared
	// dependency, the action fails for the same reason as the class.
	explainedExcluded := map[string]string{}
	for _, entry := range run.ExcludedClasses {
		if strings.HasPrefix(entry.Message, "dependency_missing: ") {
			explainedExcluded[strings.ToLower(entry.ClassName)] = entry.Message
		}
	}
	explainFlowAction := func(problem *testreport.Problem) {
		if problem == nil || problem.Type == "dependency_missing" {
			return
		}
		start := strings.Index(problem.Message, flowActionMissingMarker)
		if start < 0 {
			return
		}
		target := dependencyErrorReference(problem.Message[start:])
		message, ok := explainedExcluded[strings.ToLower(target)]
		if !ok {
			return
		}
		if problem.Detail == "" {
			problem.Detail = problem.Type + ": " + problem.Message
		}
		problem.Type, problem.Message = "dependency_missing", message+"; flow action Apex class "+target+" is excluded; "+problem.Message
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
			explainFlowAction(result.Problem)
			if result.Problem != nil && result.Problem.Type == "dependency_missing" {
				result.Reason = dependencyMissingReason
				if result.Status != testreport.StatusCompileError {
					result.Status = testreport.StatusRuntimeError
				}
			}
		}
	}
}
