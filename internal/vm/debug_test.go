package vm

import (
	"errors"
	"testing"
)

func TestExecutePausesOnDebugBreakpoint(t *testing.T) {
	program, err := CompileAnonymous("Integer x = 1;\nx = x + 1;\nSystem.debug(x);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	var pauses []DebugPause
	machine.SetDebugHooks(DebugHooks{
		Breakpoints: []DebugBreakpoint{{File: "anonymous.apex", Line: 2}},
		OnPause: func(pause DebugPause) DebugAction {
			pauses = append(pauses, pause)
			return DebugActionContinue
		},
	})
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(pauses) != 1 {
		t.Fatalf("pauses = %d", len(pauses))
	}
	pause := pauses[0]
	if pause.Reason != DebugPauseBreakpoint || pause.Location.Line != 2 || pause.Location.Column != 1 {
		t.Fatalf("pause = %#v", pause)
	}
	if got := pause.Vars["x"]; got.Kind != ValueInt || got.Int != 1 {
		t.Fatalf("pause vars x = %#v", got)
	}
	if got := result.Vars["x"]; got.Kind != ValueInt || got.Int != 2 {
		t.Fatalf("result vars x = %#v", got)
	}
}

func TestExecuteDebugBreakpointCanStop(t *testing.T) {
	program, err := CompileAnonymous("Integer x = 1;\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetDebugHooks(DebugHooks{
		Breakpoints: []DebugBreakpoint{{Line: 2}},
		OnPause: func(DebugPause) DebugAction {
			return DebugActionStop
		},
	})
	_, err = machine.Execute(program)
	var stop *DebugStopError
	if !errors.As(err, &stop) || stop.Location.Line != 2 {
		t.Fatalf("err = %#v", err)
	}
	if got := machine.Globals["x"]; got.Kind != ValueInt || got.Int != 1 {
		t.Fatalf("x after stop = %#v", got)
	}
}

func TestExecuteDebugStepPausesEachStatement(t *testing.T) {
	program, err := CompileAnonymous("Integer x = 1;\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	var lines []int
	machine.SetDebugHooks(DebugHooks{
		Step: true,
		OnPause: func(pause DebugPause) DebugAction {
			lines = append(lines, pause.Location.Line)
			return DebugActionContinue
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != 1 || lines[1] != 2 {
		t.Fatalf("lines = %#v", lines)
	}
}
