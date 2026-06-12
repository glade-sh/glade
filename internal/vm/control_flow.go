package vm

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

const (
	signalNone     controlSignal = ""
	signalReturn   controlSignal = "return"
	signalBreak    controlSignal = "break"
	signalContinue controlSignal = "continue"
	signalThrow    controlSignal = "throw"
)

type execOutcome struct {
	value       Value
	signal      controlSignal
	thrown      Value
	thrownStack []callFrame
}

type apexThrowError struct {
	value Value
	stack []callFrame
}

type activeException struct {
	value Value
	stack []callFrame
}

func (e *apexThrowError) Error() string {
	return e.value.String()
}

func apexConditionBool(value Value, context string) (bool, error) {
	if value.Kind == ValueBool {
		return value.Bool, nil
	}
	if value.Kind == ValueNull {
		return false, newNullDereferenceError(context)
	}
	return false, fmt.Errorf("%s requires Boolean, got %s", context, value.Kind)
}

func (vm *VM) executeProgram(program ir.Program, result *Result) (execOutcome, error) {
	for seq, inst := range program.Instructions {
		vm.limits.CPUTimeMS++
		if vm.limits.CPUTimeMS%64 == 0 {
			if err := vm.ctx.Err(); err != nil {
				return execOutcome{}, err
			}
		}
		if vm.limitCaps.CPUTimeMS >= 0 && vm.limits.CPUTimeMS > vm.limitCaps.CPUTimeMS {
			if vm.limitMode == LimitModeStrict || vm.limits.CPUTimeMS == vm.limitCaps.CPUTimeMS+1 {
				if err := vm.checkLimit("cpuTime", vm.limits.CPUTimeMS, vm.limitCaps.CPUTimeMS); err != nil {
					return execOutcome{}, err
				}
			}
		}
		if vm.traceEnabled && result != nil {
			result.Trace = append(result.Trace, statementTraceEvent(seq, inst, program.Source))
		}
		vm.setCurrentStatement(inst, program.Source)
		if err := vm.maybePauseForDebug(inst); err != nil {
			return execOutcome{}, err
		}
		switch inst.Op {
		case ir.OpDeclare:
			value := Null
			if inst.Expr.Kind != "" {
				evaluated, err := vm.evalForType(inst.Expr, inst.Type, result)
				if err != nil {
					return execOutcome{}, err
				}
				value = plainNull(evaluated)
			}
			coerced, err := vm.coerceAssignable(inst.Type, value)
			if err != nil {
				return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
			}
			value = coerced
			value.Static = inst.Type
			vm.Globals[inst.Name] = value
			vm.VarTypes[inst.Name] = inst.Type
		case ir.OpAssign:
			value, err := vm.evalForAssignment(inst.Name, inst.Expr, result)
			if err != nil {
				return execOutcome{}, err
			}
			if err := vm.assign(inst.Name, value); err != nil {
				return execOutcome{}, err
			}
		case ir.OpExpr:
			if _, err := vm.eval(inst.Expr, result); err != nil {
				return execOutcome{}, err
			}
		case ir.OpReturn:
			value := Null
			if inst.Expr.Kind != "" {
				returnType := ""
				if vm.currentMethod.ReturnType != "" && !strings.EqualFold(vm.currentMethod.ReturnType, "void") {
					returnType = vm.currentMethod.ReturnType
				}
				evaluated, err := vm.evalForType(inst.Expr, returnType, result)
				if err != nil {
					return execOutcome{}, err
				}
				value = evaluated
			}
			if err := vm.updateHeapLimit(); err != nil {
				return execOutcome{}, err
			}
			return execOutcome{value: value, signal: signalReturn}, nil
		case ir.OpBlock:
			out, err := vm.executeProgram(childProgram(program, inst.Then), result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpIf:
			condition, err := vm.eval(inst.Expr, result)
			if err != nil {
				return execOutcome{}, err
			}
			conditionValue, err := apexConditionBool(condition, "if condition")
			if err != nil {
				return execOutcome{}, err
			}
			branch := inst.Else
			if conditionValue {
				branch = inst.Then
			}
			if len(branch) > 0 {
				out, err := vm.executeProgram(childProgram(program, branch), result)
				if err != nil || out.signal != signalNone {
					return out, err
				}
			}
		case ir.OpWhile:
			out, err := vm.executeWhile(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpDoWhile:
			out, err := vm.executeDoWhile(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpFor:
			out, err := vm.executeFor(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpForEach:
			out, err := vm.executeForEach(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpBreak:
			return execOutcome{signal: signalBreak}, nil
		case ir.OpContinue:
			return execOutcome{signal: signalContinue}, nil
		case ir.OpThrow:
			thrown := Null
			stack := vm.rawStackFrames()
			if inst.Expr.Kind != "" {
				value, err := vm.eval(inst.Expr, result)
				if err != nil {
					return execOutcome{}, err
				}
				thrown = annotateException(value, stack)
			} else {
				if len(vm.activeExceptions) == 0 {
					return execOutcome{}, fmt.Errorf("rethrow outside catch block")
				}
				active := vm.activeExceptions[len(vm.activeExceptions)-1]
				thrown = active.value
				stack = active.stack
			}
			return execOutcome{signal: signalThrow, thrown: thrown, thrownStack: stack}, nil
		case ir.OpTry:
			out, err := vm.executeTry(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpSwitch:
			out, err := vm.executeSwitch(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpRunAs:
			out, err := vm.executeRunAs(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpDML:
			if err := vm.executeDML(inst.Name, inst.Expr, inst.Field, result); err != nil {
				return execOutcome{}, err
			}
		default:
			return execOutcome{}, fmt.Errorf("unsupported instruction %q", inst.Op)
		}
		if err := vm.updateHeapLimit(); err != nil {
			return execOutcome{}, err
		}
	}
	return execOutcome{}, nil
}

func (vm *VM) updateHeapLimit() error {
	if vm.limitMode != LimitModeStrict {
		return nil
	}
	total := vm.currentHeapSize()
	vm.limits.HeapSize = total
	return vm.checkLimit("heapSize", vm.limits.HeapSize, vm.limitCaps.HeapSize)
}

func (vm *VM) currentHeapSize() int {
	total := 0
	for name, value := range vm.Globals {
		total += len(name) + approxValueSize(value)
	}
	return total
}

func statementTraceEvent(seq int, inst ir.Instruction, source string) trace.Event {
	args := map[string]any{
		"op":           string(inst.Op),
		"sourceOffset": inst.Pos,
	}
	if source != "" {
		line, column := sourceLineColumn(source, inst.Pos)
		args["line"] = line
		args["column"] = column
	}
	if inst.Name != "" {
		args["name"] = inst.Name
	}
	if inst.Type != "" {
		args["type"] = inst.Type
	}
	return trace.Instant("apex.statement."+string(inst.Op), "apex.statement", int64(seq), args)
}

func appendTrace(result *Result, name, category string, args map[string]any) {
	if result == nil || !result.traceEnabled {
		return
	}
	result.Trace = append(result.Trace, trace.Instant(name, category, int64(len(result.Trace)), args))
}

func appendDurationTrace(result *Result, name, category string, startSeq int64, durationUS int64, args map[string]any) {
	if result == nil || !result.traceEnabled || durationUS <= 0 {
		return
	}
	result.Trace = append(result.Trace, trace.Duration(name, category, startSeq, durationUS, args))
}

func traceSeqLen(result *Result) int {
	if result == nil {
		return 0
	}
	return len(result.Trace)
}

func traceSpanStart(result *Result) (int64, time.Time) {
	return int64(traceSeqLen(result)), time.Now()
}

func traceDurationSince(start time.Time) int64 {
	elapsed := time.Since(start).Microseconds()
	if elapsed <= 0 {
		return 1
	}
	return elapsed
}

func (vm *VM) appendTrace(result *Result, name, category string, args map[string]any) {
	appendTrace(result, name, category, args)
}

func limitTraceArgs(limits Limits) map[string]any {
	return map[string]any{
		"queries":          limits.Queries,
		"queryRows":        limits.QueryRows,
		"dmlStatements":    limits.DMLStatements,
		"dmlRows":          limits.DMLRows,
		"heapSize":         limits.HeapSize,
		"cpuTimeMs":        limits.CPUTimeMS,
		"callouts":         limits.Callouts,
		"asyncJobs":        limits.AsyncJobs,
		"futureCalls":      limits.FutureCalls,
		"queueableJobs":    limits.QueueableJobs,
		"batchJobs":        limits.BatchJobs,
		"scheduledJobs":    limits.ScheduledJobs,
		"emailInvocations": limits.EmailInvokes,
		"runAs":            limits.RunAs,
	}
}

func childProgram(parent ir.Program, instructions []ir.Instruction) ir.Program {
	return ir.Program{Instructions: instructions, Source: parent.Source}
}

func sourceLineColumn(source string, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func (vm *VM) setCurrentStatement(inst ir.Instruction, source string) {
	if source == "" {
		vm.hasStatement = false
		return
	}
	line, column := sourceLineColumn(source, inst.Pos)
	symbol := string(inst.Op)
	file := ""
	if vm.currentMethod.Name != "" {
		symbol = vm.apexMethodFrameSymbol(vm.currentMethod)
		file = vm.currentMethod.File
	}
	vm.currentStatement = callFrame{Symbol: symbol, File: file, Line: line, Column: column}
	vm.hasStatement = true
}

func (vm *VM) apexMethodFrameSymbol(method Method) string {
	symbol := strings.TrimSpace(method.Name)
	className := strings.TrimSpace(method.ClassName)
	if className == "" {
		className = classNameFromMethod(symbol)
	}
	if !strings.Contains(className, ".") {
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			if class, ok := vm.lookupClassInNamespace(namespace, className); ok {
				className = runtimeClassName(class)
			} else if class, ok := vm.lookupClass(namespace + "." + className); ok {
				className = runtimeClassName(class)
			}
		} else if vm.Org != nil && strings.TrimSpace(vm.Org.Namespace) != "" {
			namespace := strings.TrimSpace(vm.Org.Namespace)
			if class, ok := vm.lookupClassInNamespace(namespace, className); ok {
				className = runtimeClassName(class)
			} else if class, ok := vm.lookupClass(namespace + "." + className); ok {
				className = runtimeClassName(class)
			}
		}
	}
	if symbol == "" || className == "" {
		return symbol
	}
	class, ok := vm.lookupClass(className)
	if !ok {
		return symbol
	}
	token := vm.classTypeToken(class)
	if token == "" || strings.EqualFold(token, className) {
		return symbol
	}
	lowerSymbol := strings.ToLower(symbol)
	lowerToken := strings.ToLower(token)
	if lowerSymbol == lowerToken || strings.HasPrefix(lowerSymbol, lowerToken+".") {
		return symbol
	}
	lowerClass := strings.ToLower(className)
	if lowerSymbol == lowerClass {
		return token
	}
	if strings.HasPrefix(lowerSymbol, lowerClass+".") {
		return token + symbol[len(className):]
	}
	shortClass := shortTypeName(className)
	lowerShortClass := strings.ToLower(shortClass)
	if shortClass != "" && !strings.EqualFold(shortClass, className) {
		if lowerSymbol == lowerShortClass {
			return token
		}
		if strings.HasPrefix(lowerSymbol, lowerShortClass+".") {
			return token + symbol[len(shortClass):]
		}
	}
	return symbol
}

func (vm *VM) executeWhile(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("while loop exceeded %d iterations", maxLoopIterations)
		}
		condition, err := vm.eval(inst.Expr, result)
		if err != nil {
			return execOutcome{}, err
		}
		conditionValue, err := apexConditionBool(condition, "while condition")
		if err != nil {
			return execOutcome{}, err
		}
		if !conditionValue {
			return execOutcome{}, nil
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone:
		case signalContinue:
			continue
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
	}
}

func (vm *VM) executeDoWhile(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("do/while loop exceeded %d iterations", maxLoopIterations)
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone, signalContinue:
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
		condition, err := vm.eval(inst.Expr, result)
		if err != nil {
			return execOutcome{}, err
		}
		conditionValue, err := apexConditionBool(condition, "do/while condition")
		if err != nil {
			return execOutcome{}, err
		}
		if !conditionValue {
			return execOutcome{}, nil
		}
	}
}

func (vm *VM) executeFor(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	inits := inst.Inits
	if len(inits) == 0 && inst.Init != nil {
		inits = []ir.Instruction{*inst.Init}
	}
	if len(inits) > 0 {
		out, err := vm.executeProgram(ir.Program{Instructions: inits, Source: source}, result)
		if err != nil || out.signal != signalNone {
			return out, err
		}
	}
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("for loop exceeded %d iterations", maxLoopIterations)
		}
		condition, err := vm.eval(inst.Expr, result)
		if err != nil {
			return execOutcome{}, err
		}
		conditionValue, err := apexConditionBool(condition, "for condition")
		if err != nil {
			return execOutcome{}, err
		}
		if !conditionValue {
			return execOutcome{}, nil
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone, signalContinue:
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
		updates := inst.Updates
		if len(updates) == 0 && inst.Update != nil {
			updates = []ir.Instruction{*inst.Update}
		}
		if len(updates) > 0 {
			out, err := vm.executeProgram(ir.Program{Instructions: updates, Source: source}, result)
			if err != nil || out.signal == signalReturn || out.signal == signalThrow {
				return out, err
			}
		}
	}
}

func (vm *VM) executeForEach(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	iterable, err := vm.eval(inst.Expr, result)
	if err != nil {
		return execOutcome{}, err
	}
	if iterable.Kind == ValueObject {
		return vm.executeObjectForEach(source, inst, result, iterable)
	}
	values := iterable.List
	if iterable.Kind == ValueSet {
		values = iterable.Set
	}
	if iterable.Kind == ValueNull {
		if children, ok := vm.emptyChildRelationshipForEach(inst.Expr); ok {
			iterable = children
			values = iterable.List
		} else {
			return execOutcome{}, newNullDereferenceError("enhanced for over null collection" + forEachExprContext(inst.Expr))
		}
	}
	if iterable.Kind != ValueList && iterable.Kind != ValueSet {
		return execOutcome{}, fmt.Errorf("enhanced for requires List or Set, got %s", iterable.Kind)
	}
	if shouldChunkEnhancedForList(inst.Type, iterable) {
		values = chunkValuesForEnhancedFor(inst.Type, values, 200)
	}
	_, existed := vm.Globals[inst.Name]
	previous := vm.Globals[inst.Name]
	previousType, hadType := vm.VarTypes[inst.Name]
	defer func() {
		if existed {
			vm.Globals[inst.Name] = previous
		} else {
			delete(vm.Globals, inst.Name)
		}
		if hadType {
			vm.VarTypes[inst.Name] = previousType
		} else {
			delete(vm.VarTypes, inst.Name)
		}
	}()
	for iteration, value := range values {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("enhanced for loop exceeded %d iterations", maxLoopIterations)
		}
		coerced, err := vm.coerceAssignable(inst.Type, value)
		if err != nil {
			return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
		}
		vm.Globals[inst.Name] = coerced
		vm.VarTypes[inst.Name] = inst.Type
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone:
		case signalContinue:
			continue
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
	}
	return execOutcome{}, nil
}

func shouldChunkEnhancedForList(loopType string, iterable Value) bool {
	if collectionBase(loopType) != "List" || iterable.Kind != ValueList {
		return false
	}
	loopElement, ok := collectionElementType(loopType)
	if !ok || loopElement == "" {
		return false
	}
	iterElement, ok := collectionElementType(iterable.Type)
	if !ok || iterElement == "" {
		return false
	}
	return strings.EqualFold(loopElement, iterElement)
}

func chunkValuesForEnhancedFor(loopType string, values []Value, size int) []Value {
	if size <= 0 || len(values) == 0 {
		return nil
	}
	chunks := make([]Value, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunk := typedList(loopType)
		chunk.List = append(chunk.List, values[start:end]...)
		chunks = append(chunks, chunk)
	}
	return chunks
}

func (vm *VM) executeTry(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
	if err != nil {
		var thrown *apexThrowError
		if !errors.As(err, &thrown) {
			return execOutcome{}, err
		}
		if len(thrown.stack) == 0 {
			thrown.stack = vm.rawStackFrames()
		}
		thrown.value = annotateException(thrown.value, thrown.stack)
		out = execOutcome{signal: signalThrow, thrown: thrown.value, thrownStack: thrown.stack}
	}
	if out.signal == signalThrow {
		for _, catchClause := range vmCatchClauses(inst) {
			if !vm.exceptionMatchesAny(catchClause.Types, out.thrown) {
				continue
			}
			previous, existed := vm.Globals[catchClause.Name]
			previousType, typeExisted := vm.VarTypes[catchClause.Name]
			caught := out.thrown
			catchType := vm.catchVariableType(catchClause)
			if catchType != "" {
				caught.Static = catchType
				vm.VarTypes[catchClause.Name] = catchType
			}
			vm.Globals[catchClause.Name] = caught
			vm.activeExceptions = append(vm.activeExceptions, activeException{value: out.thrown, stack: out.thrownStack})
			out, err = vm.executeProgram(ir.Program{Instructions: catchClause.Body, Source: source}, result)
			vm.activeExceptions = vm.activeExceptions[:len(vm.activeExceptions)-1]
			if existed {
				vm.Globals[catchClause.Name] = previous
			} else {
				delete(vm.Globals, catchClause.Name)
			}
			if typeExisted {
				vm.VarTypes[catchClause.Name] = previousType
			} else {
				delete(vm.VarTypes, catchClause.Name)
			}
			if err != nil {
				return execOutcome{}, err
			}
			break
		}
	}
	if len(inst.Finally) > 0 {
		finallyOut, err := vm.executeProgram(ir.Program{Instructions: inst.Finally, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		if finallyOut.signal != signalNone {
			return finallyOut, nil
		}
	}
	return out, nil
}

func (vm *VM) catchVariableType(catchClause ir.CatchClause) string {
	if len(catchClause.Types) != 1 {
		return ""
	}
	return vm.resolveTypeNameInClass(vm.currentClass, catchClause.Types[0])
}

func (vm *VM) executeSwitch(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	value, err := vm.eval(inst.Expr, result)
	if err != nil {
		return execOutcome{}, err
	}
	var elseCase *ir.SwitchCase
	for i := range inst.Cases {
		c := &inst.Cases[i]
		if c.Else {
			elseCase = c
			continue
		}
		for _, expr := range c.Exprs {
			if caseType, binding, ok := switchTypeCase(expr); ok {
				if !vm.switchTypeCaseMatches(value, caseType) {
					continue
				}
				out, err := vm.executeSwitchBodyWithBinding(source, c.Body, binding, caseType, value, result)
				if err != nil {
					return execOutcome{}, err
				}
				if out.signal == signalBreak {
					return execOutcome{}, nil
				}
				return out, nil
			}
			if switchCaseEnumLiteralMatch(value, expr) {
				out, err := vm.executeProgram(ir.Program{Instructions: c.Body, Source: source}, result)
				if err != nil {
					return execOutcome{}, err
				}
				if out.signal == signalBreak {
					return execOutcome{}, nil
				}
				return out, nil
			}
			if value.Kind == ValueNull {
				continue
			}
			caseValue, err := vm.eval(expr, result)
			if err != nil {
				matches, handled := switchCaseEnumNameMatch(value, expr, err)
				if !handled {
					return execOutcome{}, err
				}
				if !matches {
					continue
				}
				out, err := vm.executeProgram(ir.Program{Instructions: c.Body, Source: source}, result)
				if err != nil {
					return execOutcome{}, err
				}
				if out.signal == signalBreak {
					return execOutcome{}, nil
				}
				return out, nil
			}
			if value.Equal(caseValue) {
				out, err := vm.executeProgram(ir.Program{Instructions: c.Body, Source: source}, result)
				if err != nil {
					return execOutcome{}, err
				}
				if out.signal == signalBreak {
					return execOutcome{}, nil
				}
				return out, nil
			}
		}
	}
	if elseCase != nil {
		out, err := vm.executeProgram(ir.Program{Instructions: elseCase.Body, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		if out.signal == signalBreak {
			return execOutcome{}, nil
		}
		return out, nil
	}
	return execOutcome{}, nil
}

func switchCaseEnumLiteralMatch(value Value, expr ir.Expr) bool {
	if value.Kind == ValueNull {
		return (expr.Kind == ir.ExprVariable && strings.EqualFold(expr.Name, "null")) ||
			(expr.Kind == ir.ExprLiteral && strings.EqualFold(expr.Value, "null"))
	}
	if value.Kind != ValueObject || value.Text == "" {
		return false
	}
	switch expr.Kind {
	case ir.ExprVariable:
		return strings.EqualFold(expr.Name, value.Text)
	default:
		return false
	}
}

func switchTypeCase(expr ir.Expr) (string, string, bool) {
	if expr.Kind != ir.ExprVariable || !strings.HasPrefix(expr.Name, "__typecase:") {
		return "", "", false
	}
	rest := strings.TrimPrefix(expr.Name, "__typecase:")
	typeName, binding, ok := strings.Cut(rest, ":")
	return typeName, binding, ok && typeName != "" && binding != ""
}

func (vm *VM) switchTypeCaseMatches(value Value, typeName string) bool {
	if value.Kind == ValueNull || value.Type == "" {
		return false
	}
	return strings.EqualFold(value.Type, typeName) || vm.typeAssignableTo(value.Type, typeName) || vm.typeAssignableTo(typeName, value.Type)
}

func (vm *VM) executeSwitchBodyWithBinding(source string, body []ir.Instruction, binding, typeName string, value Value, result *Result) (execOutcome, error) {
	previousValue, hadValue := vm.Globals[binding]
	previousType, hadType := vm.VarTypes[binding]
	bound := value
	if coerced, err := vm.coerceAssignable(typeName, value); err == nil {
		bound = coerced
	}
	bound.Static = typeName
	vm.Globals[binding] = bound
	vm.VarTypes[binding] = typeName
	defer func() {
		if hadValue {
			vm.Globals[binding] = previousValue
		} else {
			delete(vm.Globals, binding)
		}
		if hadType {
			vm.VarTypes[binding] = previousType
		} else {
			delete(vm.VarTypes, binding)
		}
	}()
	return vm.executeProgram(ir.Program{Instructions: body, Source: source}, result)
}

func switchCaseEnumNameMatch(value Value, expr ir.Expr, err error) (bool, bool) {
	if err == nil || expr.Kind != ir.ExprVariable {
		return false, false
	}
	if !strings.Contains(err.Error(), "unknown variable") {
		return false, false
	}
	if value.Kind == ValueNull {
		return strings.EqualFold(expr.Name, "null"), true
	}
	if value.Kind == ValueString {
		return strings.EqualFold(expr.Name, value.Text), true
	}
	if value.Kind != ValueObject || value.Text == "" {
		return false, false
	}
	return strings.EqualFold(expr.Name, value.Text), true
}

func (vm *VM) executeRunAs(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	if vm.testContext == nil {
		return execOutcome{}, fmt.Errorf("System.runAs is only available in test context")
	}
	userExpr := inst.Expr
	var packageExpr *ir.Expr
	if inst.Expr.Kind == ir.ExprCall && inst.Expr.Callee == "System.runAs" {
		if len(inst.Expr.Args) != 2 {
			return execOutcome{}, fmt.Errorf("System.runAs expects User, Version")
		}
		userExpr = inst.Expr.Args[0]
		packageExpr = &inst.Expr.Args[1]
	}
	user, err := vm.eval(userExpr, result)
	if err != nil {
		return execOutcome{}, err
	}
	var packageVersion Value
	changesUser := true
	if packageExpr != nil {
		packageVersion, err = vm.eval(*packageExpr, result)
		if err != nil {
			return execOutcome{}, err
		}
		if !isPackageVersionValue(packageVersion) {
			return execOutcome{}, fmt.Errorf("System.runAs package context expects Package.Version")
		}
	} else if isPackageVersionValue(user) {
		packageVersion = user
		changesUser = false
	}
	if packageVersion.Kind != "" && !isPackageVersionValue(packageVersion) {
		return execOutcome{}, fmt.Errorf("System.runAs package context expects Package.Version")
	}
	if packageVersion.Kind == ValueNull {
		return execOutcome{}, fmt.Errorf("System.runAs package context expects Package.Version")
	}
	if changesUser && (user.Kind != ValueObject || !strings.EqualFold(user.Type, "User")) {
		return execOutcome{}, fmt.Errorf("System.runAs expects User or Package.Version, got %s", runtimeValueTypeName(user))
	}
	if changesUser {
		vm.ensureRunAsUserRecord(&user)
	}
	if err := vm.incrementLimit("runAs", 1); err != nil {
		return execOutcome{}, err
	}
	previous := vm.testContext.CurrentUser
	previousPackageVersion := vm.testContext.CurrentPackageVersion
	if changesUser {
		vm.testContext.CurrentUser = user
	}
	vm.testContext.RunAsDepth++
	if packageVersion.Kind != "" {
		vm.testContext.CurrentPackageVersion = packageVersion
		vm.testContext.PackageRunAsDepth++
	}
	defer func() {
		if packageVersion.Kind != "" {
			vm.testContext.PackageRunAsDepth--
			vm.testContext.CurrentPackageVersion = previousPackageVersion
		}
		vm.testContext.RunAsDepth--
		if changesUser {
			vm.testContext.CurrentUser = previous
		}
	}()
	return vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
}

func isPackageVersionValue(value Value) bool {
	return strings.EqualFold(value.Type, "Version") ||
		strings.EqualFold(value.Type, "Package.Version") ||
		strings.EqualFold(value.Static, "Package.Version") ||
		(value.Kind == ValueString && packageVersionString(value.Text))
}

func packageVersionString(text string) bool {
	parts := strings.Split(strings.TrimSpace(text), ".")
	if len(parts) != 2 && len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func (vm *VM) ensureRunAsUserRecord(user *Value) {
	if vm == nil || vm.Org == nil || user == nil || user.Kind != ValueObject || !strings.EqualFold(user.Type, "User") {
		return
	}
	objectName, ok := vm.resolveObjectName("User")
	if !ok {
		objectName = "User"
	}
	storage.EnsureMutableObjectRecords(vm.Org, objectName)
	object := vm.Org.Objects[objectName]
	if strings.TrimSpace(object.Definition.APIName) == "" {
		if definition, ok := storage.StandardObjectDefinition("User"); ok {
			object.Definition = definition
		} else {
			object.Definition = storage.ObjectDefinition{APIName: "User"}
		}
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	id := sObjectIDFromFields(user.Fields)
	if id == "" {
		prefix := object.Definition.KeyPrefix
		if prefix == "" {
			prefix = "005"
		}
		generator := storage.NewRuntimeIDGenerator(map[string]string{objectName: prefix})
		generator.Sequences = copyOrgIDSequences(vm.Org.IDSequences)
		nextID, err := generator.Next(objectName)
		if err != nil {
			return
		}
		if user.Fields == nil {
			user.Fields = make(map[string]Value)
		}
		vm.recordIsolationJournalSequence(objectName)
		id = nextID
		user.Fields["Id"] = platformScalar("Id", string(id))
		vm.Org.IDSequences = copyOrgIDSequences(generator.Sequences)
	}
	record, err := vm.recordFromValue(user)
	if err != nil {
		return
	}
	record.ID = id
	record.Object = objectName
	if storedID, stored, ok := storage.LookupRecordByID(object.Records, id); ok {
		vm.recordIsolationJournalMutation(objectName, storedID, stored, true)
		for field, value := range record.Fields {
			if stored.Fields == nil {
				stored.Fields = make(map[string]storage.Value)
			}
			stored.Fields[field] = value
		}
		object.Records[storedID] = stored
	} else {
		vm.recordIsolationJournalMutation(objectName, id, storage.Record{}, false)
		object.Records[id] = record
	}
	vm.Org.Objects[objectName] = object
	vm.ensureUserProfilePermissionSetAssignment(record)
}
