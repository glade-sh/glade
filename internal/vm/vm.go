package vm

import (
	"fmt"
	"io"
	"strings"

	"github.com/open-aer/oaer/internal/ir"
	"github.com/open-aer/oaer/internal/trace"
)

type VM struct {
	Globals map[string]Value
	Stdout  io.Writer
}

type Result struct {
	Debug       []string         `json:"debug,omitempty"`
	Vars        map[string]Value `json:"vars,omitempty"`
	TraceFormat string           `json:"traceFormat,omitempty"`
	Trace       []trace.Event    `json:"trace,omitempty"`
}

func New(stdout io.Writer) *VM {
	return &VM{Globals: make(map[string]Value), Stdout: stdout}
}

func Execute(program ir.Program, stdout io.Writer) (Result, error) {
	return New(stdout).Execute(program)
}

func (vm *VM) Execute(program ir.Program) (Result, error) {
	result := Result{Vars: vm.Globals, TraceFormat: trace.FormatChromeTraceEvent}
	for seq, inst := range program.Instructions {
		result.Trace = append(result.Trace, statementTraceEvent(seq, inst))
		switch inst.Op {
		case ir.OpDeclare:
			value := Null
			if inst.Expr.Kind != "" {
				evaluated, err := vm.eval(inst.Expr, &result)
				if err != nil {
					return result, err
				}
				value = evaluated
			}
			if err := ensureAssignable(inst.Type, value); err != nil {
				return result, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
			}
			vm.Globals[inst.Name] = value
		case ir.OpAssign:
			if _, ok := vm.Globals[inst.Name]; !ok {
				return result, fmt.Errorf("unknown variable %q", inst.Name)
			}
			value, err := vm.eval(inst.Expr, &result)
			if err != nil {
				return result, err
			}
			vm.Globals[inst.Name] = value
		case ir.OpExpr:
			if _, err := vm.eval(inst.Expr, &result); err != nil {
				return result, err
			}
		default:
			return result, fmt.Errorf("unsupported instruction %q", inst.Op)
		}
	}
	return result, nil
}

func statementTraceEvent(seq int, inst ir.Instruction) trace.Event {
	args := map[string]any{
		"op":           string(inst.Op),
		"sourceOffset": inst.Pos,
	}
	if inst.Name != "" {
		args["name"] = inst.Name
	}
	if inst.Type != "" {
		args["type"] = inst.Type
	}
	return trace.Instant("apex.statement."+string(inst.Op), "apex.statement", int64(seq), args)
}

func (vm *VM) eval(expr ir.Expr, result *Result) (Value, error) {
	switch expr.Kind {
	case ir.ExprLiteral:
		return parseLiteral(expr.Value)
	case ir.ExprVariable:
		value, ok := vm.Globals[expr.Name]
		if !ok {
			return Null, fmt.Errorf("unknown variable %q", expr.Name)
		}
		return value, nil
	case ir.ExprUnary:
		value, err := vm.eval(*expr.Left, result)
		if err != nil {
			return Null, err
		}
		return evalUnary(expr.Operator, value)
	case ir.ExprBinary:
		left, err := vm.eval(*expr.Left, result)
		if err != nil {
			return Null, err
		}
		right, err := vm.eval(*expr.Right, result)
		if err != nil {
			return Null, err
		}
		return evalBinary(expr.Operator, left, right)
	case ir.ExprCall:
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return Null, err
			}
			args = append(args, value)
		}
		return vm.call(expr.Callee, args, result)
	default:
		return Null, fmt.Errorf("unsupported expression %q", expr.Kind)
	}
}

func (vm *VM) call(callee string, args []Value, result *Result) (Value, error) {
	if strings.HasPrefix(callee, "new:") {
		return constructValue(strings.TrimPrefix(callee, "new:"), args)
	}
	if value, ok, err := vm.callMember(callee, args); ok || err != nil {
		return value, err
	}
	switch callee {
	case "System.assert":
		if len(args) != 1 {
			return Null, fmt.Errorf("System.assert expects 1 argument")
		}
		if args[0].Kind != ValueBool {
			return Null, fmt.Errorf("System.assert expects Boolean, got %s", args[0].Kind)
		}
		if !args[0].Bool {
			return Null, fmt.Errorf("System.AssertException: assertion failed")
		}
		return Null, nil
	case "System.assertEquals":
		if len(args) != 2 {
			return Null, fmt.Errorf("System.assertEquals expects 2 arguments")
		}
		if !args[0].Equal(args[1]) {
			return Null, fmt.Errorf("System.AssertException: expected <%s>, actual <%s>", args[0].String(), args[1].String())
		}
		return Null, nil
	case "System.assertNotEquals":
		if len(args) != 2 {
			return Null, fmt.Errorf("System.assertNotEquals expects 2 arguments")
		}
		if args[0].Equal(args[1]) {
			return Null, fmt.Errorf("System.AssertException: values should not be equal: <%s>", args[0].String())
		}
		return Null, nil
	case "System.debug":
		if len(args) != 1 {
			return Null, fmt.Errorf("System.debug expects 1 argument")
		}
		line := args[0].String()
		result.Debug = append(result.Debug, line)
		if vm.Stdout != nil {
			fmt.Fprintln(vm.Stdout, line)
		}
		return Null, nil
	default:
		return Null, fmt.Errorf("unsupported call %q", callee)
	}
}

func (vm *VM) callMember(callee string, args []Value) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) != 2 {
		return Null, false, nil
	}
	receiverName, method := parts[0], parts[1]
	receiver, ok := vm.Globals[receiverName]
	if !ok {
		return Null, false, nil
	}
	if value, updated, mutated, ok, err := callStdlibMember(receiver, method, args); ok || err != nil {
		if mutated {
			vm.Globals[receiverName] = updated
		}
		return value, true, err
	}

	switch receiver.Kind {
	case ValueList:
		switch method {
		case "add":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("List.add expects 1 argument")
			}
			receiver.List = append(receiver.List, args[0])
			vm.Globals[receiverName] = receiver
			return Bool(true), true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.size expects 0 arguments")
			}
			return Int(int64(len(receiver.List))), true, nil
		case "get":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.get expects integer index")
			}
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, fmt.Errorf("List index out of bounds: %d", i)
			}
			return receiver.List[i], true, nil
		case "contains":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("List.contains expects 1 argument")
			}
			return Bool(containsValue(receiver.List, args[0])), true, nil
		}
	case ValueSet:
		switch method {
		case "add":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.add expects 1 argument")
			}
			if !containsValue(receiver.Set, args[0]) {
				receiver.Set = append(receiver.Set, args[0])
				vm.Globals[receiverName] = receiver
				return Bool(true), true, nil
			}
			return Bool(false), true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Set.size expects 0 arguments")
			}
			return Int(int64(len(receiver.Set))), true, nil
		case "contains":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.contains expects 1 argument")
			}
			return Bool(containsValue(receiver.Set, args[0])), true, nil
		}
	case ValueMap:
		switch method {
		case "put":
			if len(args) != 2 {
				return Null, true, fmt.Errorf("Map.put expects 2 arguments")
			}
			receiver.Map[mapKey(args[0])] = args[1]
			vm.Globals[receiverName] = receiver
			return Null, true, nil
		case "get":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.get expects 1 argument")
			}
			value, ok := receiver.Map[mapKey(args[0])]
			if !ok {
				return Null, true, nil
			}
			return value, true, nil
		case "containsKey":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.containsKey expects 1 argument")
			}
			_, ok := receiver.Map[mapKey(args[0])]
			return Bool(ok), true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Map.size expects 0 arguments")
			}
			return Int(int64(len(receiver.Map))), true, nil
		}
	}
	return Null, true, fmt.Errorf("unsupported call %q", callee)
}
