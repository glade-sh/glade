package dap

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/vm"
)

type LiveSession struct {
	handler *Handler
	control chan liveControl
	paused  chan vm.DebugPause
	done    chan error

	mu             sync.Mutex
	mode           liveMode
	pauseRequested bool
	disconnect     bool
	stepDepth      int
}

type liveMode string

const (
	liveModeContinue liveMode = "continue"
	liveModeStepIn   liveMode = "stepIn"
	liveModeStepOver liveMode = "stepOver"
	liveModeStepOut  liveMode = "stepOut"
)

type liveControl string

const (
	liveControlContinue   liveControl = "continue"
	liveControlStepIn     liveControl = "stepIn"
	liveControlStepOver   liveControl = "stepOver"
	liveControlStepOut    liveControl = "stepOut"
	liveControlPause      liveControl = "pause"
	liveControlDisconnect liveControl = "disconnect"
)

func (h *Handler) StartLiveSession(machine *vm.VM, program ir.Program) *LiveSession {
	session := &LiveSession{
		handler: h,
		control: make(chan liveControl, 1),
		paused:  make(chan vm.DebugPause, 1),
		done:    make(chan error, 1),
		mode:    liveModeContinue,
	}
	h.setLiveSession(session)
	hooks := h.DebugHooks(session.onPause)
	hooks.Step = true
	machine.SetDebugHooks(hooks)
	machine.SetDebugOutputSink(func(event vm.DebugEvent) {
		session.handler.PublishEvent(session.handler.EventMessage("output", map[string]any{
			"category": "console",
			"output":   "USER_DEBUG|" + consoleDebugMessage(event.Message) + "\n",
		}))
	})
	go func() {
		_, err := machine.Execute(program)
		if err != nil {
			session.handler.PublishEvent(session.handler.EventMessage("output", map[string]any{
				"category": "stderr",
				"output":   err.Error() + "\n",
			}))
		}
		session.handler.PublishEvent(session.handler.EventMessage("terminated", nil))
		session.done <- err
	}()
	return session
}

func (s *LiveSession) Continue() {
	s.send(liveControlContinue)
}

func (s *LiveSession) StepIn() {
	s.send(liveControlStepIn)
}

func (s *LiveSession) StepOver() {
	s.send(liveControlStepOver)
}

func (s *LiveSession) StepOut() {
	s.send(liveControlStepOut)
}

func (s *LiveSession) Pause() {
	s.mu.Lock()
	s.pauseRequested = true
	s.mu.Unlock()
}

func (s *LiveSession) Paused() <-chan vm.DebugPause {
	return s.paused
}

func (s *LiveSession) Disconnect() {
	s.mu.Lock()
	s.disconnect = true
	s.mu.Unlock()
	s.send(liveControlDisconnect)
}

func (s *LiveSession) WaitPaused() vm.DebugPause {
	return <-s.paused
}

func (s *LiveSession) Done() <-chan error {
	return s.done
}

func (s *LiveSession) onPause(pause vm.DebugPause) vm.DebugAction {
	if s.shouldDisconnect() {
		return vm.DebugActionStop
	}
	if !s.shouldStop(pause) {
		return vm.DebugActionContinue
	}
	s.handler.PublishEvent(s.handler.ApplyPause(pause))
	select {
	case s.paused <- pause:
	default:
	}
	for {
		switch <-s.control {
		case liveControlContinue:
			s.setMode(liveModeContinue, false)
			return vm.DebugActionContinue
		case liveControlStepIn:
			s.setStepMode(liveModeStepIn, len(pause.Stack))
			return vm.DebugActionContinue
		case liveControlStepOver:
			s.setStepMode(liveModeStepOver, len(pause.Stack))
			return vm.DebugActionContinue
		case liveControlStepOut:
			depth := len(pause.Stack) - 1
			if depth < 0 {
				depth = 0
			}
			s.setStepMode(liveModeStepOut, depth)
			return vm.DebugActionContinue
		case liveControlPause:
			s.setMode(liveModeStepIn, true)
		case liveControlDisconnect:
			return vm.DebugActionStop
		}
	}
}

func (s *LiveSession) shouldDisconnect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disconnect
}

func (s *LiveSession) shouldStop(pause vm.DebugPause) bool {
	if pause.Reason == vm.DebugPauseBreakpoint {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pauseRequested {
		s.pauseRequested = false
		return true
	}
	switch s.mode {
	case liveModeStepIn:
		s.mode = liveModeContinue
		return true
	case liveModeStepOver, liveModeStepOut:
		if len(pause.Stack) <= s.stepDepth {
			s.mode = liveModeContinue
			return true
		}
	}
	return false
}

func (s *LiveSession) setMode(mode liveMode, pauseRequested bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.pauseRequested = pauseRequested
}

func (s *LiveSession) setStepMode(mode liveMode, depth int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.pauseRequested = false
	s.stepDepth = depth
}

func (s *LiveSession) send(control liveControl) {
	select {
	case s.control <- control:
	default:
	}
}

func consoleDebugMessage(message string) string {
	return strings.Join(strings.Fields(message), " ")
}
