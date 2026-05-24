package dap

import (
	"errors"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestLiveSessionContinueStepAndDisconnect(t *testing.T) {
	h := NewHandler(Snapshot{})
	h.Handle(request(1, CommandSetBreakpoints, map[string]any{
		"source":      map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{{"line": 2}},
	}))
	program, err := vm.CompileAnonymous("Integer x = 1;\nx = x + 1;\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	session := h.StartLiveSession(machine, program)
	first := waitPaused(t, session)
	if first.Reason != vm.DebugPauseBreakpoint || first.Location.Line != 2 {
		t.Fatalf("first pause = %#v", first)
	}
	h.Handle(request(2, CommandNext, map[string]any{"threadId": 1}))
	second := waitPaused(t, session)
	if second.Reason != vm.DebugPauseStep || second.Location.Line != 3 {
		t.Fatalf("second pause = %#v", second)
	}
	h.Handle(request(3, CommandContinue, map[string]any{"threadId": 1}))
	select {
	case err := <-session.Done():
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("live session did not finish")
	}
	if got := machine.Globals["x"]; got.Kind != vm.ValueInt || got.Int != 3 {
		t.Fatalf("x = %#v", got)
	}
}

func TestLiveSessionStepInOverAndOutUseStackDepth(t *testing.T) {
	h := NewHandler(Snapshot{})
	h.Handle(request(1, CommandSetBreakpoints, map[string]any{
		"source":      map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{{"line": 1}},
	}))
	methodProgram, err := vm.CompileAnonymous("Integer y = a + b;\nreturn y;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymous("Integer x = Util.add(1, 2);\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	if err := machine.RegisterMethod(vm.Method{
		Name:       "Util.add",
		File:       "classes/Util.cls",
		ReturnType: "Integer",
		Params: []vm.Param{
			{Name: "a", Type: "Integer"},
			{Name: "b", Type: "Integer"},
		},
		Program: methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	session := h.StartLiveSession(machine, program)
	first := waitPaused(t, session)
	if first.Location.Line != 1 {
		t.Fatalf("first pause = %#v", first)
	}
	h.Handle(request(2, CommandStepIn, map[string]any{"threadId": 1}))
	inside := waitPaused(t, session)
	if inside.Location.Symbol != "Util.add" || inside.Location.Line != 1 {
		t.Fatalf("step in pause = %#v", inside)
	}
	h.Handle(request(3, CommandStepOut, map[string]any{"threadId": 1}))
	afterCall := waitPaused(t, session)
	if afterCall.Location.Symbol == "Util.add" || afterCall.Location.Line != 2 {
		t.Fatalf("step out pause = %#v", afterCall)
	}
	h.Handle(request(4, CommandContinue, map[string]any{"threadId": 1}))
	select {
	case err := <-session.Done():
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("live session did not finish")
	}

	h2 := NewHandler(Snapshot{})
	h2.Handle(request(1, CommandSetBreakpoints, map[string]any{
		"source":      map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{{"line": 1}},
	}))
	machine2 := vm.New(nil)
	if err := machine2.RegisterMethod(vm.Method{
		Name:       "Util.add",
		File:       "classes/Util.cls",
		ReturnType: "Integer",
		Params:     []vm.Param{{Name: "a", Type: "Integer"}, {Name: "b", Type: "Integer"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	session2 := h2.StartLiveSession(machine2, program)
	waitPaused(t, session2)
	h2.Handle(request(2, CommandNext, map[string]any{"threadId": 1}))
	over := waitPaused(t, session2)
	if over.Location.Symbol == "Util.add" || over.Location.Line != 2 {
		t.Fatalf("step over pause = %#v", over)
	}
	h2.Handle(request(3, CommandContinue, map[string]any{"threadId": 1}))
	select {
	case err := <-session2.Done():
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("second live session did not finish")
	}
}

func TestLiveSessionPauseAndDisconnect(t *testing.T) {
	h := NewHandler(Snapshot{})
	program, err := vm.CompileAnonymous("Integer x = 1;\nx = x + 1;\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	session := h.StartLiveSession(machine, program)
	h.Handle(request(1, CommandPause, map[string]any{"threadId": 1}))
	pause := waitPaused(t, session)
	if pause.Location.Line <= 0 {
		t.Fatalf("pause = %#v", pause)
	}
	h.Handle(request(2, CommandDisconnect, map[string]any{"threadId": 1}))
	select {
	case err := <-session.Done():
		var stop *vm.DebugStopError
		if !errors.As(err, &stop) {
			t.Fatalf("err = %#v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("live session did not stop")
	}
}

func waitPaused(t *testing.T, session *LiveSession) vm.DebugPause {
	t.Helper()
	select {
	case pause := <-session.paused:
		return pause
	case err := <-session.Done():
		t.Fatalf("session finished before pause: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pause")
	}
	return vm.DebugPause{}
}
