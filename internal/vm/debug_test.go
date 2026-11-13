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

func TestDebugPrecheckSkipsSnapshotForRejectedPoints(t *testing.T) {
	program, err := CompileAnonymous("Integer x = 1;\nx = x + 1;\nx = x + 1;\nSystem.debug(x);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	probes := 0
	var pauses []DebugPause
	machine.SetDebugHooks(DebugHooks{
		Step: true,
		Precheck: func(probe DebugProbe) bool {
			probes++
			if probe.Reason != DebugPauseStep || probe.StackDepth < 1 {
				t.Errorf("probe = %#v", probe)
			}
			return probe.Location.Line == 3
		},
		OnPause: func(pause DebugPause) DebugAction {
			pauses = append(pauses, pause)
			return DebugActionContinue
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if probes < 4 {
		t.Fatalf("probes = %d, want one per statement", probes)
	}
	if len(pauses) != 1 || pauses[0].Location.Line != 3 {
		t.Fatalf("pauses = %#v, want only line 3", pauses)
	}
	if got := pauses[0].Vars["x"]; got.Kind != ValueInt || got.Int != 2 {
		t.Fatalf("accepted pause vars x = %#v", got)
	}
	if len(pauses[0].Stack) != 1 {
		t.Fatalf("accepted pause stack = %#v", pauses[0].Stack)
	}
}

func TestDebugStackDepthMatchesStackFrames(t *testing.T) {
	program, err := CompileAnonymous("Integer x = 1;\nx = x + 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetDebugHooks(DebugHooks{
		Step: true,
		Precheck: func(probe DebugProbe) bool {
			if got, want := probe.StackDepth, len(stackFrames(machine.rawStackFrames())); got != want {
				t.Errorf("depth = %d, frames = %d", got, want)
			}
			return false
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// BenchmarkDebugContinueThroughLoop runs a loop with a large local under step
// hooks that never stop, the shape of a Continue with no breakpoint. The
// precheck variant is what the debug adapter uses.
func BenchmarkDebugContinueThroughLoop(b *testing.B) {
	program, err := CompileAnonymous("List<Integer> big = new List<Integer>();\nfor (Integer i = 0; i < 2000; i++) { big.add(i); }\nInteger sum = 0;\nfor (Integer i = 0; i < 300; i++) {\n  sum += i;\n}")
	if err != nil {
		b.Fatal(err)
	}
	for _, mode := range []string{"snapshot-every-statement", "precheck"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				machine := New(nil)
				hooks := DebugHooks{Step: true, OnPause: func(DebugPause) DebugAction { return DebugActionContinue }}
				if mode == "precheck" {
					hooks.Precheck = func(DebugProbe) bool { return false }
				}
				machine.SetDebugHooks(hooks)
				if _, err := machine.Execute(program); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
