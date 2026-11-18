package vm

import "strings"

// CoverageSink receives Apex code coverage events while a test runs. The VM
// reports source positions and names only; the sink maps them to lines.
// A nil sink, the default, disables collection: every hook is one nil check.
type CoverageSink interface {
	// CoverStatement reports a statement, a conditionally evaluated operand,
	// a catch clause, a tested switch label, or a method, constructor or
	// accessor entry, by the source position that starts it.
	CoverStatement(file string, line, column int)
	// CoverIteration reports one iteration of the SOQL for loop at line and
	// column.
	CoverIteration(file string, line, column int)
	// CoverClassInit reports a class's static or instance initialization.
	CoverClassInit(class string, static bool)
	// CoverProperty reports an access to a class property.
	CoverProperty(class, name string)
}

// SetCoverage installs a coverage sink; nil turns collection off. Clones do
// not inherit it.
func (vm *VM) SetCoverage(sink CoverageSink) {
	if vm != nil {
		vm.coverage = sink
	}
}

// coverageSuppressed reports a method whose program has no source positions
// of its own: a compiled field initializer. Its lines are covered through the
// class initialization event instead.
func (vm *VM) coverageSuppressed() bool {
	name := vm.currentMethod.Name
	if name == vm.coverageLastName {
		return vm.coverageLastSkip
	}
	skip := strings.Contains(name, "field_init>.")
	vm.coverageLastName, vm.coverageLastSkip = name, skip
	return skip
}

func (vm *VM) coverStatement(line, column int) {
	if vm.currentMethod.Name == "" || vm.coverageSuppressed() {
		return
	}
	vm.coverage.CoverStatement(vm.currentMethod.File, line, column)
}

// coverSourcePos reports the position pos of source, in the current method.
func (vm *VM) coverSourcePos(source string, pos int) {
	if pos <= 0 || source == "" || vm.currentMethod.Name == "" || vm.coverageSuppressed() {
		return
	}
	line, column := vm.sourceLineColumnCached(source, pos)
	vm.coverage.CoverStatement(vm.currentMethod.File, line, column)
}

// coverOperand reports a conditionally evaluated operand that is about to
// run. Operands carry offsets into the current statement's source.
func (vm *VM) coverOperand(pos int32) {
	if pos > 0 && vm.hasStatement {
		vm.coverSourcePos(vm.currentSource, int(pos))
	}
}

// coverIteration reports one iteration of a SOQL for loop.
func (vm *VM) coverIteration(source string, pos int) {
	if pos <= 0 || source == "" || vm.currentMethod.Name == "" || vm.coverageSuppressed() {
		return
	}
	line, column := vm.sourceLineColumnCached(source, pos)
	vm.coverage.CoverIteration(vm.currentMethod.File, line, column)
}

// coverMethodEntry reports entering a method, constructor or accessor body.
func (vm *VM) coverMethodEntry(method Method) {
	if method.File == "" || method.Line <= 0 || strings.Contains(method.Name, "field_init>.") {
		return
	}
	vm.coverage.CoverStatement(method.File, method.Line, method.Column)
}

func (vm *VM) coverProperty(field Field, owner string) {
	if field.Property && field.Name != "" {
		vm.coverage.CoverProperty(owner, field.Name)
	}
}
