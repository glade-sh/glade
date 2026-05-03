package dap

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-aer/oaer/internal/ir"
	"github.com/open-aer/oaer/internal/vm"
)

const (
	localsReference  = 1
	staticsReference = 2
	triggerReference = 3
	firstVarRef      = 4
)

type Handler struct {
	stateMu     sync.Mutex
	seqMu       sync.Mutex
	snapshot    Snapshot
	seq         int
	breakpoints map[string][]Breakpoint
	varRefs     map[int]vm.Value
	varRefKeys  map[string]int
	nextVarRef  int
	live        *LiveSession
}

func NewHandler(snapshot Snapshot) *Handler {
	return &Handler{
		snapshot:    snapshot,
		breakpoints: make(map[string][]Breakpoint),
		varRefs:     make(map[int]vm.Value),
		varRefKeys:  make(map[string]int),
		nextVarRef:  firstVarRef,
	}
}

func (h *Handler) Handle(request Request) []any {
	switch request.Command {
	case CommandInitialize:
		return h.withInitializedEvent(request)
	case CommandSetBreakpoints:
		return []any{h.handleSetBreakpoints(request)}
	case CommandConfigurationDone:
		return []any{h.success(request, nil)}
	case CommandThreads:
		threads := h.normalizedThreads()
		return []any{h.success(request, map[string]any{"threads": threads})}
	case CommandStackTrace:
		return []any{h.handleStackTrace(request)}
	case CommandScopes:
		return []any{h.handleScopes(request)}
	case CommandVariables:
		return []any{h.handleVariables(request)}
	case CommandEvaluate:
		return []any{h.handleEvaluate(request)}
	case CommandContinue:
		if live := h.liveSession(); live != nil {
			live.Continue()
		}
		return []any{h.success(request, map[string]any{"allThreadsContinued": true})}
	case CommandNext, CommandStepIn, CommandStepOut:
		if live := h.liveSession(); live != nil {
			switch request.Command {
			case CommandStepIn:
				live.StepIn()
			case CommandStepOut:
				live.StepOut()
			default:
				live.StepOver()
			}
			return []any{h.success(request, nil)}
		}
		return h.withStoppedEvent(request, "step")
	case CommandPause:
		if live := h.liveSession(); live != nil {
			live.Pause()
			return []any{h.success(request, nil)}
		}
		return h.withStoppedEvent(request, "pause")
	case CommandDisconnect:
		if live := h.liveSession(); live != nil {
			live.Disconnect()
		}
		return []any{h.success(request, nil), h.event("terminated", nil)}
	default:
		return []any{h.failure(request, fmt.Sprintf("unsupported command %q", request.Command))}
	}
}

func (h *Handler) handleEvaluate(request Request) Response {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := request.DecodeArguments(&args); err != nil {
		return h.failure(request, err.Error())
	}
	h.stateMu.Lock()
	value, ok := lookupSnapshotValue(h.snapshot.Vars, h.snapshot.Statics, args.Expression)
	if !ok {
		h.stateMu.Unlock()
		return h.failure(request, fmt.Sprintf("unknown expression %q", args.Expression))
	}
	ref := 0
	if len(childValues(value)) > 0 {
		ref = h.variableReferenceLocked("eval."+args.Expression, value)
	}
	h.stateMu.Unlock()
	return h.success(request, map[string]any{
		"result":             value.String(),
		"type":               string(value.Kind),
		"variablesReference": ref,
	})
}

func (h *Handler) handleSetBreakpoints(request Request) Response {
	var args struct {
		Source      Source `json:"source"`
		Breakpoints []struct {
			Line   int `json:"line"`
			Column int `json:"column,omitempty"`
		} `json:"breakpoints"`
		Lines []int `json:"lines"`
	}
	if err := request.DecodeArguments(&args); err != nil {
		return h.failure(request, err.Error())
	}
	path := args.Source.Path
	if path == "" {
		path = args.Source.Name
	}
	var points []Breakpoint
	for i, bp := range args.Breakpoints {
		points = append(points, Breakpoint{
			ID:       breakpointID(path, i, bp.Line, bp.Column),
			Verified: bp.Line > 0,
			Source:   &args.Source,
			Line:     bp.Line,
			Column:   bp.Column,
		})
	}
	if len(args.Breakpoints) == 0 {
		for i, line := range args.Lines {
			points = append(points, Breakpoint{
				ID:       breakpointID(path, i, line, 0),
				Verified: line > 0,
				Source:   &args.Source,
				Line:     line,
			})
		}
	}
	h.stateMu.Lock()
	h.breakpoints[path] = points
	h.stateMu.Unlock()
	return h.success(request, map[string]any{"breakpoints": points})
}

func (h *Handler) DebugHooks(onPause func(vm.DebugPause) vm.DebugAction) vm.DebugHooks {
	points := make([]vm.DebugBreakpoint, 0)
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	for path, breakpoints := range h.breakpoints {
		for _, breakpoint := range breakpoints {
			if !breakpoint.Verified {
				continue
			}
			file := path
			if file == "" && breakpoint.Source != nil {
				file = breakpoint.Source.Path
				if file == "" {
					file = breakpoint.Source.Name
				}
			}
			points = append(points, vm.DebugBreakpoint{
				File:   file,
				Line:   breakpoint.Line,
				Column: breakpoint.Column,
			})
		}
	}
	return vm.DebugHooks{Breakpoints: points, OnPause: onPause}
}

func (h *Handler) ExecuteToBreakpoint(machine *vm.VM, program ir.Program) (vm.Result, []Event, error) {
	var events []Event
	hooks := h.DebugHooks(func(pause vm.DebugPause) vm.DebugAction {
		events = append(events, h.ApplyPause(pause))
		return vm.DebugActionStop
	})
	machine.SetDebugHooks(hooks)
	result, err := machine.Execute(program)
	if err != nil {
		var stop *vm.DebugStopError
		if !errors.As(err, &stop) {
			return result, events, err
		}
		return result, events, nil
	}
	return result, events, nil
}

func (h *Handler) ApplyPause(pause vm.DebugPause) Event {
	h.stateMu.Lock()
	h.snapshot = Snapshot{
		Threads: []Thread{{ID: 1, Name: "main"}},
		Frames:  stackFramesFromPause(pause),
		Vars:    pause.Vars,
		Statics: pause.Statics,
	}
	h.varRefs = make(map[int]vm.Value)
	h.varRefKeys = make(map[string]int)
	h.nextVarRef = firstVarRef
	h.stateMu.Unlock()
	reason := string(pause.Reason)
	if reason == "" {
		reason = "pause"
	}
	return h.event("stopped", map[string]any{
		"reason":            reason,
		"threadId":          1,
		"allThreadsStopped": true,
	})
}

func stackFramesFromPause(pause vm.DebugPause) []StackFrame {
	if len(pause.Stack) == 0 {
		return []StackFrame{stackFrameFromDebugLocation(1, pause.Location)}
	}
	frames := make([]StackFrame, 0, len(pause.Stack))
	for i, frame := range pause.Stack {
		frames = append(frames, StackFrame{
			ID:     i + 1,
			Name:   frame.Symbol,
			Source: sourceFromFile(frame.File),
			Line:   frame.Line,
			Column: frame.Column,
		})
	}
	return frames
}

func stackFrameFromDebugLocation(id int, location vm.DebugLocation) StackFrame {
	return StackFrame{
		ID:     id,
		Name:   location.Symbol,
		Source: sourceFromFile(location.File),
		Line:   location.Line,
		Column: location.Column,
	}
}

func sourceFromFile(file string) *Source {
	if file == "" {
		return &Source{Name: "anonymous.apex"}
	}
	return &Source{Name: filepath.Base(file), Path: file}
}

func (h *Handler) handleStackTrace(request Request) Response {
	var args struct {
		StartFrame int `json:"startFrame"`
		Levels     int `json:"levels"`
	}
	if err := request.DecodeArguments(&args); err != nil {
		return h.failure(request, err.Error())
	}
	frames := h.normalizedFrames()
	start := args.StartFrame
	if start < 0 {
		start = 0
	}
	if start > len(frames) {
		start = len(frames)
	}
	end := len(frames)
	if args.Levels > 0 && start+args.Levels < end {
		end = start + args.Levels
	}
	return h.success(request, map[string]any{
		"stackFrames": frames[start:end],
		"totalFrames": len(frames),
	})
}

func (h *Handler) handleScopes(request Request) Response {
	var args struct {
		FrameID int `json:"frameId"`
	}
	if err := request.DecodeArguments(&args); err != nil {
		return h.failure(request, err.Error())
	}
	h.stateMu.Lock()
	scopes := h.scopesLocked()
	h.stateMu.Unlock()
	return h.success(request, map[string]any{"scopes": scopes})
}

func (h *Handler) handleVariables(request Request) Response {
	var args struct {
		VariablesReference int `json:"variablesReference"`
		Start              int `json:"start"`
		Count              int `json:"count"`
	}
	if err := request.DecodeArguments(&args); err != nil {
		return h.failure(request, err.Error())
	}
	var variables []Variable
	h.stateMu.Lock()
	if args.VariablesReference == localsReference {
		variables = h.variablesFromNamedValuesLocked("locals", h.localsLocked())
	} else if args.VariablesReference == staticsReference {
		variables = h.variablesFromNamedValuesLocked("statics", namedValuesFromMap(h.snapshot.Statics))
	} else if args.VariablesReference == triggerReference {
		variables = h.variablesFromNamedValuesLocked("trigger", h.triggerContextLocked())
	} else if value, ok := h.varRefs[args.VariablesReference]; ok {
		variables = h.variablesFromNamedValuesLocked(fmt.Sprintf("ref:%d", args.VariablesReference), childValues(value))
	} else {
		h.stateMu.Unlock()
		return h.failure(request, fmt.Sprintf("unknown variablesReference %d", args.VariablesReference))
	}
	h.stateMu.Unlock()
	variables = sliceVariables(variables, args.Start, args.Count)
	return h.success(request, map[string]any{"variables": variables})
}

func (h *Handler) withInitializedEvent(request Request) []any {
	return []any{
		h.success(request, map[string]any{
			"supportsConfigurationDoneRequest": true,
			"supportsSetVariable":              false,
			"supportsStepBack":                 false,
			"supportsRestartRequest":           false,
		}),
		h.event("initialized", nil),
	}
}

func (h *Handler) withStoppedEvent(request Request, reason string) []any {
	threadID := 1
	var args struct {
		ThreadID int `json:"threadId"`
	}
	if err := request.DecodeArguments(&args); err == nil && args.ThreadID > 0 {
		threadID = args.ThreadID
	}
	return []any{
		h.success(request, nil),
		h.event("stopped", map[string]any{
			"reason":            reason,
			"threadId":          threadID,
			"allThreadsStopped": true,
		}),
	}
}

func (h *Handler) success(request Request, body any) Response {
	return Response{
		Seq:        h.nextSeq(),
		Type:       MessageTypeResponse,
		RequestSeq: request.Seq,
		Success:    true,
		Command:    request.Command,
		Body:       body,
	}
}

func (h *Handler) failure(request Request, message string) Response {
	return Response{
		Seq:        h.nextSeq(),
		Type:       MessageTypeResponse,
		RequestSeq: request.Seq,
		Success:    false,
		Command:    request.Command,
		Message:    message,
	}
}

func (h *Handler) event(name string, body any) Event {
	return Event{
		Seq:   h.nextSeq(),
		Type:  MessageTypeEvent,
		Event: name,
		Body:  body,
	}
}

func (h *Handler) nextSeq() int {
	h.seqMu.Lock()
	defer h.seqMu.Unlock()
	h.seq++
	return h.seq
}

func (h *Handler) normalizedThreads() []Thread {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return h.snapshot.normalizedThreads()
}

func (h *Handler) normalizedFrames() []StackFrame {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return h.snapshot.normalizedFrames()
}

func (h *Handler) liveSession() *LiveSession {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return h.live
}

func (h *Handler) setLiveSession(session *LiveSession) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.live = session
}

func (h *Handler) scopesLocked() []Scope {
	scopes := []Scope{{
		Name:               "Locals",
		VariablesReference: localsReference,
		Expensive:          false,
		NamedVariables:     len(h.localsLocked()),
	}}
	if len(h.snapshot.Statics) > 0 {
		scopes = append(scopes, Scope{
			Name:               "Statics",
			VariablesReference: staticsReference,
			Expensive:          false,
			NamedVariables:     len(h.snapshot.Statics),
		})
	}
	if triggerValues := h.triggerContextLocked(); len(triggerValues) > 0 {
		scopes = append(scopes, Scope{
			Name:               "Trigger",
			VariablesReference: triggerReference,
			Expensive:          false,
			NamedVariables:     len(triggerValues),
		})
	}
	return scopes
}

func (h *Handler) localsLocked() []namedValue {
	values := make(map[string]vm.Value, len(h.snapshot.Vars))
	for name, value := range h.snapshot.Vars {
		if strings.HasPrefix(name, "Trigger.") {
			continue
		}
		values[name] = value
	}
	return namedValuesFromMap(values)
}

func (h *Handler) triggerContextLocked() []namedValue {
	values := make(map[string]vm.Value)
	for name, value := range h.snapshot.Vars {
		if !strings.HasPrefix(name, "Trigger.") {
			continue
		}
		values[strings.TrimPrefix(name, "Trigger.")] = value
	}
	return namedValuesFromMap(values)
}

func (h *Handler) variablesFromNamedValuesLocked(prefix string, values []namedValue) []Variable {
	variables := make([]Variable, 0, len(values))
	for _, value := range values {
		ref := 0
		if len(childValues(value.value)) > 0 {
			ref = h.variableReferenceLocked(prefix+"."+value.name, value.value)
		}
		variables = append(variables, variableFromValue(value.name, value.value, ref))
	}
	return variables
}

func (h *Handler) variableReferenceLocked(key string, value vm.Value) int {
	if ref, ok := h.varRefKeys[key]; ok {
		h.varRefs[ref] = value
		return ref
	}
	ref := h.nextVarRef
	h.nextVarRef++
	h.varRefKeys[key] = ref
	h.varRefs[ref] = value
	return ref
}

func breakpointID(path string, index, line, column int) int {
	id := 17
	for _, r := range path {
		id = id*31 + int(r)
	}
	id = id*31 + index
	id = id*31 + line
	id = id*31 + column
	if id < 0 {
		id = -id
	}
	if id == 0 {
		return 1
	}
	return id
}

func sliceVariables(variables []Variable, start, count int) []Variable {
	if start < 0 {
		start = 0
	}
	if start > len(variables) {
		start = len(variables)
	}
	end := len(variables)
	if count > 0 && start+count < end {
		end = start + count
	}
	return variables[start:end]
}

func lookupSnapshotValue(vars, statics map[string]vm.Value, expression string) (vm.Value, bool) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return vm.Null, false
	}
	if value, ok := vars[expression]; ok {
		return value, true
	}
	parts := strings.Split(expression, ".")
	if len(parts) > 1 {
		if value, ok := lookupValuePath(statics[parts[0]], parts[1:]); ok {
			return value, true
		}
	}
	current, ok := vars[parts[0]]
	if !ok {
		return vm.Null, false
	}
	return lookupValuePath(current, parts[1:])
}

func lookupValuePath(current vm.Value, parts []string) (vm.Value, bool) {
	for _, part := range parts {
		if current.Kind != vm.ValueObject {
			return vm.Null, false
		}
		next, ok := current.Fields[part]
		if !ok {
			return vm.Null, false
		}
		current = next
	}
	return current, true
}
