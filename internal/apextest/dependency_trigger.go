package apextest

import (
	"errors"
	"strings"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Keep excluded dependency triggers executable as local boundary checks. A
// filtered trigger must not let DML succeed merely because its body was removed.
// Definitions stay on the run's private VM; the semantic subset and cache remain
// unchanged. Unrelated invalid triggers retain the normal compile-gate policy.
func (g *compileGate) restoreDependencyTriggers(machine *vm.VM) ([]typesys.TriggerSymbol, error) {
	if g == nil || g.dependencyIndex == nil {
		return nil, nil
	}
	sources := g.dependencySources
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		return nil, err
	}
	index := *g.dependencyIndex
	roots := map[string]bool{}
	for _, entry := range g.excluded {
		if entry.DependsOn != "" {
			continue
		}
		message := entry.Message
		for _, problem := range entry.Problems {
			if !strings.Contains(message, problem.Message) {
				message += "\n" + problem.Message
			}
		}
		original := &vm.RuntimeError{Type: "CompileError", Message: message, Stack: []vm.StackFrame{{File: entry.File}}}
		var explained *vm.RuntimeError
		if err := ExplainDependencyError(index, "", original); err != original && errors.As(err, &explained) && explained.Type == "dependency_missing" {
			roots[strings.ToLower(entry.Root)] = true
		}
	}
	affected := func(file string) bool {
		entry, ok := g.fileEntry[file]
		return ok && roots[strings.ToLower(g.excluded[entry].Root)]
	}
	var triggers []typesys.TriggerSymbol
	for _, trigger := range index.Triggers {
		if affected(trigger.File) {
			triggers = append(triggers, trigger)
		}
	}
	if len(triggers) == 0 {
		return nil, nil
	}
	// An exclusion records only one root, but a trigger can reach several
	// independently excluded helpers. Follow bound references from every
	// restored trigger instead of using that single attribution as its closure.
	units := newCompileGateUnits(index)
	requiredFiles := map[string]bool{}
	var pending []string
	for _, trigger := range triggers {
		requiredFiles[trigger.File] = true
		pending = append(pending, trigger.File)
	}
	for len(pending) > 0 {
		file := pending[0]
		pending = pending[1:]
		source, err := sources.read(file)
		if err != nil {
			return nil, err
		}
		for _, target := range compileGateReferences(file, source, units) {
			dependency := units.fileOf[target]
			if affected(dependency) && !requiredFiles[dependency] {
				requiredFiles[dependency] = true
				pending = append(pending, dependency)
			}
		}
	}
	include := func(typ typesys.TypeSymbol) bool {
		return requiredFiles[typ.File]
	}
	methods := compileProjectMethodsWhere(index, include, sources)
	classes := compileProjectClassesWhere(index, methods, include, sources)
	index.Triggers = triggers
	compiled, errs := compileProjectTriggers(index, sources)
	if err := sources.sourceSnapshotError(); err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return nil, errs[0]
	}
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		return nil, err
	}
	return triggers, registerBaseRuntime(machine, methods, classes, compiled)
}
