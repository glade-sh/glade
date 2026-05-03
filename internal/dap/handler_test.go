package dap

import (
	"encoding/json"
	"testing"

	"github.com/open-aer/oaer/internal/trace"
	"github.com/open-aer/oaer/internal/vm"
)

func TestHandlerInitializeAndExecutionControls(t *testing.T) {
	h := NewHandler(Snapshot{})

	initMessages := h.Handle(request(1, CommandInitialize, nil))
	if len(initMessages) != 2 {
		t.Fatalf("initialize messages = %d", len(initMessages))
	}
	initResponse := initMessages[0].(Response)
	if !initResponse.Success || initResponse.Command != CommandInitialize {
		t.Fatalf("initialize response = %#v", initResponse)
	}
	initEvent := initMessages[1].(Event)
	if initEvent.Event != "initialized" {
		t.Fatalf("initialize event = %#v", initEvent)
	}

	controls := []struct {
		command string
		body    any
		event   string
	}{
		{command: CommandConfigurationDone},
		{command: CommandContinue, body: struct {
			AllThreadsContinued bool `json:"allThreadsContinued"`
		}{}},
		{command: CommandNext, body: map[string]any{"threadId": 1}, event: "stopped"},
		{command: CommandPause, body: map[string]any{"threadId": 1}, event: "stopped"},
		{command: CommandDisconnect, event: "terminated"},
	}
	for i, control := range controls {
		messages := h.Handle(request(i+2, control.command, control.body))
		if len(messages) == 0 {
			t.Fatalf("%s returned no messages", control.command)
		}
		response := messages[0].(Response)
		if !response.Success || response.Command != control.command {
			t.Fatalf("%s response = %#v", control.command, response)
		}
		if control.event != "" {
			if len(messages) != 2 {
				t.Fatalf("%s messages = %d", control.command, len(messages))
			}
			event := messages[1].(Event)
			if event.Event != control.event {
				t.Fatalf("%s event = %#v", control.command, event)
			}
		}
	}
}

func TestHandlerBreakpointsTraceScopesAndVariables(t *testing.T) {
	h := NewHandler(Snapshot{
		Trace: []trace.Event{
			trace.Instant("AccountTest.testCreatesAccount", "apex.statement", 0, map[string]any{
				"path":   "force-app/main/classes/AccountTest.cls",
				"line":   42,
				"column": 9,
			}),
			trace.Instant("AccountService.save", "apex.statement", 1, map[string]any{
				"path": "force-app/main/classes/AccountService.cls",
				"line": 7,
			}),
		},
		Vars: map[string]vm.Value{
			"account": {
				Kind: vm.ValueMap,
				Map: map[string]vm.Value{
					"Name": vm.String("Acme"),
				},
			},
			"count": vm.Int(2),
			"names": vm.List(vm.String("Acme"), vm.String("Global Media")),
		},
	})

	breakpoints := h.Handle(request(1, CommandSetBreakpoints, map[string]any{
		"source": map[string]any{
			"name": "AccountTest.cls",
			"path": "force-app/main/classes/AccountTest.cls",
		},
		"breakpoints": []map[string]any{{"line": 42}, {"line": 0}},
	}))
	var bpBody struct {
		Breakpoints []Breakpoint `json:"breakpoints"`
	}
	decodeBody(t, breakpoints[0].(Response), &bpBody)
	if len(bpBody.Breakpoints) != 2 || !bpBody.Breakpoints[0].Verified || bpBody.Breakpoints[1].Verified {
		t.Fatalf("breakpoints = %#v", bpBody.Breakpoints)
	}
	if bpBody.Breakpoints[0].ID == 0 || bpBody.Breakpoints[0].Line != 42 {
		t.Fatalf("breakpoint = %#v", bpBody.Breakpoints[0])
	}

	threads := h.Handle(request(2, CommandThreads, nil))
	var threadsBody struct {
		Threads []Thread `json:"threads"`
	}
	decodeBody(t, threads[0].(Response), &threadsBody)
	if len(threadsBody.Threads) != 1 || threadsBody.Threads[0].Name != "main" {
		t.Fatalf("threads = %#v", threadsBody.Threads)
	}

	stack := h.Handle(request(3, CommandStackTrace, map[string]any{
		"startFrame": 1,
		"levels":     1,
	}))
	var stackBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
		TotalFrames int          `json:"totalFrames"`
	}
	decodeBody(t, stack[0].(Response), &stackBody)
	if stackBody.TotalFrames != 2 || len(stackBody.StackFrames) != 1 {
		t.Fatalf("stack = %#v", stackBody)
	}
	frame := stackBody.StackFrames[0]
	if frame.Name != "AccountService.save" || frame.Line != 7 || frame.Column != 1 {
		t.Fatalf("frame = %#v", frame)
	}

	scopes := h.Handle(request(4, CommandScopes, map[string]any{"frameId": frame.ID}))
	var scopesBody struct {
		Scopes []Scope `json:"scopes"`
	}
	decodeBody(t, scopes[0].(Response), &scopesBody)
	if len(scopesBody.Scopes) != 1 || scopesBody.Scopes[0].Name != "Locals" || scopesBody.Scopes[0].VariablesReference != 1 {
		t.Fatalf("scopes = %#v", scopesBody.Scopes)
	}

	locals := h.Handle(request(5, CommandVariables, map[string]any{"variablesReference": 1}))
	var localsBody struct {
		Variables []Variable `json:"variables"`
	}
	decodeBody(t, locals[0].(Response), &localsBody)
	if names := variableNames(localsBody.Variables); names != "account,count,names" {
		t.Fatalf("variables = %s (%#v)", names, localsBody.Variables)
	}
	namesRef := findVariable(t, localsBody.Variables, "names").VariablesReference
	if namesRef == 0 {
		t.Fatalf("names variable has no child reference: %#v", localsBody.Variables)
	}

	children := h.Handle(request(6, CommandVariables, map[string]any{"variablesReference": namesRef}))
	var childrenBody struct {
		Variables []Variable `json:"variables"`
	}
	decodeBody(t, children[0].(Response), &childrenBody)
	if len(childrenBody.Variables) != 2 || childrenBody.Variables[0].Name != "0" || childrenBody.Variables[0].Value != "Acme" {
		t.Fatalf("children = %#v", childrenBody.Variables)
	}

	evaluated := h.Handle(request(7, CommandEvaluate, map[string]any{"expression": "count"}))
	var evalBody struct {
		Result string `json:"result"`
		Type   string `json:"type"`
	}
	decodeBody(t, evaluated[0].(Response), &evalBody)
	if evalBody.Result != "2" || evalBody.Type != string(vm.ValueInt) {
		t.Fatalf("evaluate = %#v", evalBody)
	}
}

func TestHandlerDebugHooksAndApplyPause(t *testing.T) {
	h := NewHandler(Snapshot{})
	h.Handle(request(1, CommandSetBreakpoints, map[string]any{
		"source": map[string]any{
			"name": "anonymous.apex",
		},
		"breakpoints": []map[string]any{{"line": 2}, {"line": 0}},
	}))
	var pauses []vm.DebugPause
	hooks := h.DebugHooks(func(pause vm.DebugPause) vm.DebugAction {
		pauses = append(pauses, pause)
		h.ApplyPause(pause)
		return vm.DebugActionContinue
	})
	if len(hooks.Breakpoints) != 1 || hooks.Breakpoints[0].Line != 2 {
		t.Fatalf("hooks = %#v", hooks.Breakpoints)
	}
	program, err := vm.CompileAnonymous("Integer x = 1;\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	machine.SetDebugHooks(hooks)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if len(pauses) != 1 || pauses[0].Location.Line != 2 {
		t.Fatalf("pauses = %#v", pauses)
	}

	stack := h.Handle(request(2, CommandStackTrace, nil))
	var stackBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
	}
	decodeBody(t, stack[0].(Response), &stackBody)
	if len(stackBody.StackFrames) == 0 || stackBody.StackFrames[0].Line != 2 {
		t.Fatalf("stack = %#v", stackBody.StackFrames)
	}
	vars := h.Handle(request(3, CommandVariables, map[string]any{"variablesReference": 1}))
	var varsBody struct {
		Variables []Variable `json:"variables"`
	}
	decodeBody(t, vars[0].(Response), &varsBody)
	if findVariable(t, varsBody.Variables, "x").Value != "1" {
		t.Fatalf("vars = %#v", varsBody.Variables)
	}
}

func TestHandlerExecuteToBreakpointStopsBeforeStatement(t *testing.T) {
	h := NewHandler(Snapshot{})
	h.Handle(request(1, CommandSetBreakpoints, map[string]any{
		"source": map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{
			{"line": 2},
		},
	}))
	program, err := vm.CompileAnonymous("Integer x = 1;\nx = x + 1;\nSystem.debug(x);")
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	_, events, err := h.ExecuteToBreakpoint(machine, program)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v", events)
	}
	if got := machine.Globals["x"]; got.Kind != vm.ValueInt || got.Int != 1 {
		t.Fatalf("x after breakpoint = %#v", got)
	}

	stack := h.Handle(request(2, CommandStackTrace, nil))
	var stackBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
	}
	decodeBody(t, stack[0].(Response), &stackBody)
	if len(stackBody.StackFrames) == 0 || stackBody.StackFrames[0].Line != 2 {
		t.Fatalf("stack = %#v", stackBody.StackFrames)
	}
}

func TestHandlerUnsupportedCommandReturnsFailure(t *testing.T) {
	messages := NewHandler(Snapshot{}).Handle(request(1, "launch", nil))
	response := messages[0].(Response)
	if response.Success || response.Message == "" {
		t.Fatalf("response = %#v", response)
	}
}

func request(seq int, command string, args any) Request {
	var raw json.RawMessage
	if args != nil {
		data, err := json.Marshal(args)
		if err != nil {
			panic(err)
		}
		raw = data
	}
	return Request{
		Seq:       seq,
		Type:      MessageTypeRequest,
		Command:   command,
		Arguments: raw,
	}
}

func decodeBody(t *testing.T, response Response, out any) {
	t.Helper()
	data, err := json.Marshal(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}

func variableNames(variables []Variable) string {
	out := ""
	for i, variable := range variables {
		if i > 0 {
			out += ","
		}
		out += variable.Name
	}
	return out
}

func findVariable(t *testing.T, variables []Variable, name string) Variable {
	t.Helper()
	for _, variable := range variables {
		if variable.Name == name {
			return variable
		}
	}
	t.Fatalf("missing variable %q in %#v", name, variables)
	return Variable{}
}
