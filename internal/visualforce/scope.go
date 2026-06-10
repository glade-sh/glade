package visualforce

import (
	"strings"

	"github.com/glade-sh/glade/internal/vm"
)

type ScopeStack struct {
	frames []map[string]vm.Value
}

func NewScopeStack() *ScopeStack {
	return &ScopeStack{}
}

func (stack *ScopeStack) PushFrame() {
	if stack == nil {
		return
	}
	stack.frames = append(stack.frames, make(map[string]vm.Value))
}

func (stack *ScopeStack) PopFrame() {
	if stack == nil || len(stack.frames) == 0 {
		return
	}
	stack.frames = stack.frames[:len(stack.frames)-1]
}

func (stack *ScopeStack) Set(name string, value vm.Value) {
	if stack == nil {
		return
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return
	}
	if len(stack.frames) == 0 {
		stack.PushFrame()
	}
	stack.frames[len(stack.frames)-1][name] = value
}

func (stack *ScopeStack) Get(name string) (vm.Value, bool) {
	if stack == nil {
		return vm.Null, false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for i := len(stack.frames) - 1; i >= 0; i-- {
		if value, ok := stack.frames[i][name]; ok {
			return value, true
		}
	}
	return vm.Null, false
}

func (stack *ScopeStack) WithFrame(fn func()) {
	stack.PushFrame()
	defer stack.PopFrame()
	fn()
}
