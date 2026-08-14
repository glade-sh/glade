package apextest

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"math"
	"reflect"
	"sort"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/vm"
)

type runtimePatchValueContainerIdentity struct {
	kind byte
	ptr  uintptr
}

type runtimePatchFingerprintWriter struct {
	hash         hash.Hash
	bytes        uint64
	ok           bool
	buffer       [8192]byte
	buffered     int
	activeValues map[runtimePatchValueContainerIdentity]bool
	activeExprs  map[*ir.Expr]bool
}

func runtimePatchCompiledPayloadFingerprint(entry runtimeCacheEntry) (string, bool) {
	return runtimePatchCompiledPayloadFingerprintWithPerf(entry, nil)
}

func runtimePatchCompiledPayloadFingerprintWithPerf(entry runtimeCacheEntry, counters *runPerfCounters) (string, bool) {
	writer := runtimePatchFingerprintWriter{
		hash:         sha256.New(),
		ok:           true,
		activeValues: make(map[runtimePatchValueContainerIdentity]bool),
		activeExprs:  make(map[*ir.Expr]bool),
	}
	writer.string(0x01, runtimePatchABI)
	writer.methods(0x02, entry.Methods)
	writer.classes(0x03, entry.Classes)
	writer.triggers(0x04, entry.Triggers)
	writer.errors(0x05, entry.TriggerErrors)
	writer.strings(0x06, entry.PageNames, true)
	writer.string(0x07, runtimePatchErrorIdentity(entry.BaseErr))
	writer.flush()
	if !writer.ok {
		return "", false
	}
	if counters != nil {
		counters.runtimeFingerprintBytes.Add(writer.bytes)
	}
	return hex.EncodeToString(writer.hash.Sum(nil)), true
}

func runtimePatchErrorIdentity(err error) string {
	if err == nil {
		return ""
	}
	return reflect.TypeOf(err).String() + ":" + err.Error()
}

func (writer *runtimePatchFingerprintWriter) raw(values ...byte) {
	if !writer.ok {
		return
	}
	writer.writeBytes(values)
}

func (writer *runtimePatchFingerprintWriter) writeBytes(values []byte) {
	for len(values) > 0 && writer.ok {
		if writer.buffered == len(writer.buffer) {
			writer.flush()
			continue
		}
		n := copy(writer.buffer[writer.buffered:], values)
		writer.buffered += n
		writer.bytes += uint64(n) // #nosec G115 -- copy counts are guaranteed nonnegative and fit in uint64.
		values = values[n:]
	}
}

func (writer *runtimePatchFingerprintWriter) flush() {
	if !writer.ok || writer.buffered == 0 {
		return
	}
	n, err := writer.hash.Write(writer.buffer[:writer.buffered])
	if err != nil || n != writer.buffered {
		writer.ok = false
		return
	}
	writer.buffered = 0
}

func (writer *runtimePatchFingerprintWriter) fixed(tag byte, value uint64) {
	if !writer.ok {
		return
	}
	var encoded [9]byte
	encoded[0] = tag
	binary.BigEndian.PutUint64(encoded[1:], value)
	writer.writeBytes(encoded[:])
}

func (writer *runtimePatchFingerprintWriter) integer(tag byte, value int64) {
	writer.fixed(tag, uint64(value)) // #nosec G115 -- preserve the signed value's two's-complement bit pattern.
}

func (writer *runtimePatchFingerprintWriter) count(tag byte, value int) {
	if value < 0 {
		writer.ok = false
		return
	}
	writer.fixed(tag, uint64(value)) // #nosec G115 -- value is nonnegative and an int always fits in uint64.
}

func (writer *runtimePatchFingerprintWriter) boolean(tag byte, value bool) {
	if value {
		writer.raw(tag, 1)
		return
	}
	writer.raw(tag, 0)
}

func (writer *runtimePatchFingerprintWriter) decimal(tag byte, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		writer.ok = false
		return
	}
	writer.fixed(tag, math.Float64bits(value))
}

func (writer *runtimePatchFingerprintWriter) string(tag byte, value string) {
	writer.raw(tag)
	writer.count(0, len(value))
	if !writer.ok || value == "" {
		return
	}
	writer.writeBytes([]byte(value))
}

func (writer *runtimePatchFingerprintWriter) container(tag byte, isNil bool, length int) {
	writer.raw(tag)
	writer.boolean(0, isNil)
	writer.count(0, length)
}

func (writer *runtimePatchFingerprintWriter) strings(tag byte, values []string, preserveNil bool) {
	writer.container(tag, preserveNil && values == nil, len(values))
	for _, value := range values {
		writer.string(0, value)
	}
}

func (writer *runtimePatchFingerprintWriter) errors(tag byte, values []error) {
	// The legacy format normalized nil and empty error slices to the same
	// zero-length list. Preserve that authority behavior.
	writer.container(tag, false, len(values))
	for _, err := range values {
		writer.string(0, runtimePatchErrorIdentity(err))
	}
}

func (writer *runtimePatchFingerprintWriter) methods(tag byte, methods map[string]vm.Method) {
	writer.container(tag, methods == nil, len(methods))
	if methods == nil {
		return
	}
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writer.string(0, name)
		writer.method(methods[name], 0)
	}
}

func (writer *runtimePatchFingerprintWriter) method(method vm.Method, depth int) {
	if depth > 256 {
		writer.ok = false
		return
	}
	writer.raw(0x10)
	writer.string(0x01, method.Name)
	writer.string(0x02, method.ReturnType)
	writer.params(0x03, method.Params)
	writer.program(0x04, method.Program, depth+1)
	writer.string(0x05, method.ClassName)
	writer.boolean(0x06, method.IsStatic)
	writer.boolean(0x07, method.IsConstructor)
	writer.string(0x08, method.Access)
	writer.strings(0x09, method.Modifiers, true)
	writer.string(0x0a, method.File)
	writer.string(0x0b, method.APIVersion)
	writer.integer(0x0c, int64(method.Line))
	writer.integer(0x0d, int64(method.Column))
	writer.string(0x0e, method.Unsupported)
	writer.boolean(0x0f, method.Dependency)
}

func (writer *runtimePatchFingerprintWriter) params(tag byte, params []vm.Param) {
	writer.container(tag, params == nil, len(params))
	for _, param := range params {
		writer.raw(0x11)
		writer.string(0x01, param.Name)
		writer.string(0x02, param.Type)
	}
}

func (writer *runtimePatchFingerprintWriter) program(tag byte, program ir.Program, depth int) {
	writer.raw(tag)
	writer.instructions(0x01, program.Instructions, true, depth+1)
	writer.string(0x02, program.Source)
}

func (writer *runtimePatchFingerprintWriter) instructions(tag byte, instructions []ir.Instruction, preserveNil bool, depth int) {
	if depth > 256 {
		writer.ok = false
		return
	}
	writer.container(tag, preserveNil && instructions == nil, len(instructions))
	for _, instruction := range instructions {
		writer.instruction(instruction, depth+1)
	}
}

func (writer *runtimePatchFingerprintWriter) instruction(instruction ir.Instruction, depth int) {
	if depth > 256 {
		writer.ok = false
		return
	}
	writer.raw(0x12)
	writer.string(0x01, string(instruction.Op))
	writer.string(0x02, instruction.Type)
	writer.strings(0x03, instruction.CatchTypes, false)
	writer.string(0x04, instruction.Name)
	writer.expr(0x05, instruction.Expr, depth+1)
	writer.string(0x06, instruction.Field)
	writer.instructionPointer(0x07, instruction.Init, depth+1)
	writer.instructions(0x08, instruction.Inits, false, depth+1)
	writer.instructionPointer(0x09, instruction.Update, depth+1)
	writer.instructions(0x0a, instruction.Updates, false, depth+1)
	writer.instructions(0x0b, instruction.Then, false, depth+1)
	writer.instructions(0x0c, instruction.Else, false, depth+1)
	writer.instructions(0x0d, instruction.Catch, false, depth+1)
	writer.catches(0x0e, instruction.Catches, depth+1)
	writer.instructions(0x0f, instruction.Finally, false, depth+1)
	writer.cases(0x10, instruction.Cases, depth+1)
	writer.integer(0x11, int64(instruction.Pos))
}

func (writer *runtimePatchFingerprintWriter) instructionPointer(tag byte, instruction *ir.Instruction, depth int) {
	writer.raw(tag)
	writer.boolean(0, instruction == nil)
	if instruction != nil {
		writer.instruction(*instruction, depth+1)
	}
}

func (writer *runtimePatchFingerprintWriter) catches(tag byte, catches []ir.CatchClause, depth int) {
	writer.container(tag, false, len(catches))
	for _, clause := range catches {
		writer.raw(0x13)
		writer.strings(0x01, clause.Types, false)
		writer.string(0x02, clause.Name)
		writer.instructions(0x03, clause.Body, false, depth+1)
		writer.integer(0x04, int64(clause.Pos))
	}
}

func (writer *runtimePatchFingerprintWriter) cases(tag byte, cases []ir.SwitchCase, depth int) {
	writer.container(tag, false, len(cases))
	for _, switchCase := range cases {
		writer.raw(0x14)
		writer.exprs(0x01, switchCase.Exprs, depth+1)
		writer.instructions(0x02, switchCase.Body, false, depth+1)
		writer.boolean(0x03, switchCase.Else)
		writer.integer(0x04, int64(switchCase.Pos))
	}
}

func (writer *runtimePatchFingerprintWriter) exprs(tag byte, expressions []ir.Expr, depth int) {
	writer.container(tag, false, len(expressions))
	for _, expression := range expressions {
		writer.expr(0, expression, depth+1)
	}
}

func (writer *runtimePatchFingerprintWriter) expr(tag byte, expression ir.Expr, depth int) {
	if depth > 256 {
		writer.ok = false
		return
	}
	writer.raw(tag, 0x15)
	writer.string(0x01, string(expression.Kind))
	writer.string(0x02, expression.Value)
	writer.string(0x03, expression.Name)
	writer.string(0x04, expression.Callee)
	writer.string(0x05, expression.Operator)
	writer.exprs(0x06, expression.Args, depth+1)
	writer.namedArgs(0x07, expression.NamedArgs, depth+1)
	writer.exprPointer(0x08, expression.Left, depth+1)
	writer.exprPointer(0x09, expression.Right, depth+1)
}

func (writer *runtimePatchFingerprintWriter) namedArgs(tag byte, args []ir.NamedArg, depth int) {
	writer.container(tag, false, len(args))
	for _, arg := range args {
		writer.raw(0x16)
		writer.string(0x01, arg.Name)
		writer.expr(0x02, arg.Expr, depth+1)
	}
}

func (writer *runtimePatchFingerprintWriter) exprPointer(tag byte, expression *ir.Expr, depth int) {
	writer.raw(tag)
	writer.boolean(0, expression == nil)
	if expression == nil {
		return
	}
	if writer.activeExprs[expression] {
		writer.ok = false
		return
	}
	writer.activeExprs[expression] = true
	writer.expr(0, *expression, depth+1)
	delete(writer.activeExprs, expression)
}

func (writer *runtimePatchFingerprintWriter) classes(tag byte, classes []vm.Class) {
	writer.container(tag, classes == nil, len(classes))
	for _, class := range classes {
		writer.class(class)
	}
}

func (writer *runtimePatchFingerprintWriter) class(class vm.Class) {
	writer.raw(0x20)
	writer.string(0x01, class.Name)
	writer.string(0x02, class.Namespace)
	writer.string(0x03, class.SuperClass)
	writer.strings(0x04, class.Interfaces, true)
	writer.fields(0x05, class.Fields)
	writer.fields(0x06, class.StaticFields)
	writer.strings(0x07, class.FieldOrder, true)
	writer.strings(0x08, class.StaticFieldOrder, true)
	writer.methods(0x09, class.Methods)
	writer.methodSlice(0x0a, class.Constructors)
	writer.methodSlice(0x0b, class.StaticInitializers)
	writer.methodSlice(0x0c, class.InstanceInitializers)
	writer.strings(0x0d, class.EnumValues, true)
	writer.string(0x0e, class.Access)
	writer.strings(0x0f, class.Modifiers, true)
	writer.boolean(0x10, class.IsAbstract)
	writer.boolean(0x11, class.IsInterface)
	writer.boolean(0x12, class.IsTest)
	writer.boolean(0x13, class.Dependency)
}

func (writer *runtimePatchFingerprintWriter) methodSlice(tag byte, methods []vm.Method) {
	writer.container(tag, methods == nil, len(methods))
	for _, method := range methods {
		writer.method(method, 0)
	}
}

func (writer *runtimePatchFingerprintWriter) fields(tag byte, fields map[string]vm.Field) {
	writer.container(tag, fields == nil, len(fields))
	if fields == nil {
		return
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writer.string(0, name)
		writer.field(fields[name])
	}
}

func (writer *runtimePatchFingerprintWriter) field(field vm.Field) {
	writer.raw(0x21)
	writer.string(0x01, field.Name)
	writer.string(0x02, field.Type)
	writer.boolean(0x03, field.Static)
	writer.value(0x04, field.Value, 0)
	writer.value(0x05, field.InitialValue, 0)
	writer.string(0x06, field.Access)
	writer.strings(0x07, field.Modifiers, true)
	writer.boolean(0x08, field.Property)
	writer.methodPointer(0x09, field.Getter)
	writer.methodPointer(0x0a, field.Setter)
	writer.boolean(0x0b, field.HasSetter)
	writer.string(0x0c, field.File)
	writer.boolean(0x0d, field.Dependency)
	writer.string(0x0e, field.StorageName)
}

func (writer *runtimePatchFingerprintWriter) methodPointer(tag byte, method *vm.Method) {
	writer.raw(tag)
	writer.boolean(0, method == nil)
	if method != nil {
		writer.method(*method, 0)
	}
}

func (writer *runtimePatchFingerprintWriter) triggers(tag byte, triggers []vm.Trigger) {
	writer.container(tag, triggers == nil, len(triggers))
	for _, trigger := range triggers {
		writer.raw(0x30)
		writer.string(0x01, trigger.Name)
		writer.string(0x02, trigger.Namespace)
		writer.string(0x03, trigger.Object)
		writer.string(0x04, trigger.Timing)
		writer.string(0x05, trigger.Operation)
		writer.string(0x0a, trigger.APIVersion)
		writer.program(0x06, trigger.Program, 0)
		writer.string(0x07, trigger.File)
		writer.integer(0x08, int64(trigger.Line))
		writer.integer(0x09, int64(trigger.Column))
	}
}

func (writer *runtimePatchFingerprintWriter) value(tag byte, value vm.Value, depth int) {
	if depth > 256 {
		writer.ok = false
		return
	}
	writer.raw(tag, 0x40)
	writer.string(0x01, string(value.Kind))
	writer.integer(0x02, value.Int)
	writer.decimal(0x03, value.Decimal)
	writer.boolean(0x04, value.Bool)
	writer.string(0x05, value.Text)
	writer.string(0x06, value.Type)
	writer.string(0x07, value.Static)
	writer.string(0x08, value.Runtime)
	writer.fixed(0x09, value.Ref)
	writer.valueMap(0x0a, 'f', value.Fields, depth+1)
	writer.valueSlice(0x0b, 'l', value.List, depth+1)
	writer.valueSlice(0x0c, 's', value.Set, depth+1)
	writer.valueMap(0x0d, 'm', value.Map, depth+1)
	writer.valueMap(0x0e, 'k', value.MapKeys, depth+1)
	writer.strings(0x0f, value.MapOrder, true)
}

func (writer *runtimePatchFingerprintWriter) valueMap(tag, kind byte, values map[string]vm.Value, depth int) {
	writer.container(tag, values == nil, len(values))
	if values == nil {
		return
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(values).Pointer()}
	if writer.activeValues[identity] {
		writer.ok = false
		return
	}
	writer.activeValues[identity] = true
	defer delete(writer.activeValues, identity)
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writer.string(0, name)
		writer.value(0, values[name], depth+1)
	}
}

func (writer *runtimePatchFingerprintWriter) valueSlice(tag, kind byte, values []vm.Value, depth int) {
	writer.container(tag, values == nil, len(values))
	if values == nil {
		return
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(values).Pointer()}
	if writer.activeValues[identity] {
		writer.ok = false
		return
	}
	writer.activeValues[identity] = true
	defer delete(writer.activeValues, identity)
	for _, value := range values {
		writer.value(0, value, depth+1)
	}
}
