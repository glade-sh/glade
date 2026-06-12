package dap

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/vm"
)

const (
	MessageTypeRequest  = "request"
	MessageTypeResponse = "response"
	MessageTypeEvent    = "event"
)

const (
	CommandInitialize        = "initialize"
	CommandLaunch            = "launch"
	CommandAttach            = "attach"
	CommandSetBreakpoints    = "setBreakpoints"
	CommandConfigurationDone = "configurationDone"
	CommandThreads           = "threads"
	CommandStackTrace        = "stackTrace"
	CommandScopes            = "scopes"
	CommandVariables         = "variables"
	CommandContinue          = "continue"
	CommandNext              = "next"
	CommandStepIn            = "stepIn"
	CommandStepOut           = "stepOut"
	CommandPause             = "pause"
	CommandEvaluate          = "evaluate"
	CommandDisconnect        = "disconnect"
)

type LaunchRequest struct {
	Program    string `json:"program"`
	Project    string `json:"project"`
	Source     string `json:"source,omitempty"`
	DBPath     string `json:"dbPath,omitempty"`
	ClassName  string `json:"className,omitempty"`
	MethodName string `json:"methodName,omitempty"`
}

type LaunchHandler func(request LaunchRequest) error

type Request struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func (r Request) DecodeArguments(v any) error {
	if len(r.Arguments) == 0 {
		return nil
	}
	return json.Unmarshal(r.Arguments, v)
}

type Response struct {
	Seq        int    `json:"seq"`
	Type       string `json:"type"`
	RequestSeq int    `json:"request_seq"`
	Success    bool   `json:"success"`
	Command    string `json:"command"`
	Message    string `json:"message,omitempty"`
	Body       any    `json:"body,omitempty"`
}

type Event struct {
	Seq   int    `json:"seq"`
	Type  string `json:"type"`
	Event string `json:"event"`
	Body  any    `json:"body,omitempty"`
}

type Source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

type Breakpoint struct {
	ID       int     `json:"id,omitempty"`
	Verified bool    `json:"verified"`
	Message  string  `json:"message,omitempty"`
	Source   *Source `json:"source,omitempty"`
	Line     int     `json:"line,omitempty"`
	Column   int     `json:"column,omitempty"`
}

type Thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type StackFrame struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Source *Source `json:"source,omitempty"`
	Line   int     `json:"line"`
	Column int     `json:"column"`
}

type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
}

type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
}

type Snapshot struct {
	Threads []Thread
	Frames  []StackFrame
	Trace   []trace.Event
	Vars    map[string]vm.Value
	Statics map[string]vm.Value
}

type namedValue struct {
	name  string
	value vm.Value
}

func NewSnapshot(events []trace.Event, vars map[string]vm.Value) Snapshot {
	return Snapshot{
		Trace: events,
		Vars:  vars,
	}
}

func (s Snapshot) normalizedThreads() []Thread {
	if len(s.Threads) > 0 {
		out := append([]Thread(nil), s.Threads...)
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].ID < out[j].ID
		})
		return out
	}
	return []Thread{{ID: 1, Name: "main"}}
}

func (s Snapshot) normalizedFrames() []StackFrame {
	if len(s.Frames) > 0 {
		out := append([]StackFrame(nil), s.Frames...)
		for i := range out {
			if out[i].ID == 0 {
				out[i].ID = i + 1
			}
			if out[i].Name == "" {
				out[i].Name = fmt.Sprintf("frame %d", i)
			}
			if out[i].Line <= 0 {
				out[i].Line = 1
			}
			if out[i].Column <= 0 {
				out[i].Column = 1
			}
		}
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].ID < out[j].ID
		})
		return out
	}
	if len(s.Trace) == 0 {
		return []StackFrame{{
			ID:     1,
			Name:   "entry",
			Source: &Source{Name: "anonymous.apex"},
			Line:   1,
			Column: 1,
		}}
	}
	out := make([]StackFrame, 0, len(s.Trace))
	for i, event := range s.Trace {
		frame := StackFrame{
			ID:     i + 1,
			Name:   event.Name,
			Line:   positiveInt(event.Args, "line", int(event.Timestamp)+1),
			Column: positiveInt(event.Args, "column", 1),
		}
		if frame.Name == "" {
			frame.Name = fmt.Sprintf("trace event %d", i)
		}
		path := stringArg(event.Args, "path")
		if path == "" {
			path = stringArg(event.Args, "file")
		}
		sourceName := stringArg(event.Args, "source")
		if sourceName == "" && path != "" {
			sourceName = filepath.Base(path)
		}
		if sourceName != "" || path != "" {
			frame.Source = &Source{Name: sourceName, Path: path}
		}
		out = append(out, frame)
	}
	return out
}

func variablesFromMap(vars map[string]vm.Value) []Variable {
	values := namedValuesFromMap(vars)
	out := make([]Variable, 0, len(values))
	for _, value := range values {
		out = append(out, variableFromValue(value.name, value.value, 0))
	}
	return out
}

func namedValuesFromMap(vars map[string]vm.Value) []namedValue {
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]namedValue, 0, len(names))
	for _, name := range names {
		out = append(out, namedValue{name: name, value: vars[name]})
	}
	return out
}

func variableFromValue(name string, value vm.Value, ref int) Variable {
	out := Variable{
		Name:               name,
		Value:              value.String(),
		Type:               string(value.Kind),
		VariablesReference: ref,
	}
	switch value.Kind {
	case vm.ValueList:
		out.IndexedVariables = len(value.List)
	case vm.ValueSet:
		out.IndexedVariables = len(value.Set)
	case vm.ValueMap:
		out.NamedVariables = len(value.Map)
	case vm.ValueObject:
		out.NamedVariables = len(value.Fields)
	}
	return out
}

func variableChildren(value vm.Value) []Variable {
	children := childValues(value)
	out := make([]Variable, 0, len(children))
	for _, child := range children {
		out = append(out, variableFromValue(child.name, child.value, 0))
	}
	return out
}

func childValues(value vm.Value) []namedValue {
	switch value.Kind {
	case vm.ValueList:
		out := make([]namedValue, 0, len(value.List))
		for i, child := range value.List {
			out = append(out, namedValue{name: strconv.Itoa(i), value: child})
		}
		return out
	case vm.ValueSet:
		out := make([]namedValue, 0, len(value.Set))
		for i, child := range value.Set {
			out = append(out, namedValue{name: strconv.Itoa(i), value: child})
		}
		return out
	case vm.ValueMap:
		keys := make([]string, 0, len(value.Map))
		for key := range value.Map {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make([]namedValue, 0, len(keys))
		for _, key := range keys {
			out = append(out, namedValue{name: key, value: value.Map[key]})
		}
		return out
	case vm.ValueObject:
		keys := make([]string, 0, len(value.Fields))
		for key := range value.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make([]namedValue, 0, len(keys))
		for _, key := range keys {
			out = append(out, namedValue{name: key, value: value.Fields[key]})
		}
		return out
	default:
		return nil
	}
}

func positiveInt(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	switch value := args[key].(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	case json.Number:
		n, err := value.Int64()
		if err == nil && n > 0 {
			return int(n)
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
