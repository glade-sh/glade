package vm

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"
)

var aliasShadowEnabled = os.Getenv("GLADE_ALIAS_SHADOW") == "1"

type aliasShadowVisitKey struct {
	kind             ValueKind
	ref              uint64
	backing          uintptr
	secondaryBacking uintptr
	tertiaryBacking  uintptr
	length           int
	capacity         int
}

func (vm *VM) shadowCheckOccurrences(updated Value) bool {
	if vm == nil || updated.Kind != ValueObject || updated.Ref == 0 {
		return true
	}
	seen := make(map[aliasShadowVisitKey]struct{})
	var walk func(Value, string) bool
	walk = func(value Value, path string) bool {
		if value.Kind == ValueObject && value.Ref == updated.Ref && !sameMapIdentity(value.Fields, updated.Fields) {
			fmt.Fprintf(os.Stderr, "SHADOW-DIVERGE %s %s\n", aliasShadowTestName(), path)
			return false
		}
		backing, secondary, tertiary := aliasShadowBackings(value)
		key := aliasShadowVisitKey{
			kind: value.Kind, ref: value.Ref, backing: backing,
			secondaryBacking: secondary, tertiaryBacking: tertiary,
			length: valueLength(value), capacity: valueCapacity(value),
		}
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
		switch value.Kind {
		case ValueObject:
			index := 0
			for _, child := range value.Fields {
				if !walk(child, fmt.Sprintf("%s.field[%d]", path, index)) {
					return false
				}
				index++
			}
		case ValueMap:
			index := 0
			for _, child := range value.Map {
				if !walk(child, fmt.Sprintf("%s.map[%d]", path, index)) {
					return false
				}
				index++
			}
			index = 0
			for _, child := range value.MapKeys {
				if !walk(child, fmt.Sprintf("%s.mapKey[%d]", path, index)) {
					return false
				}
				index++
			}
		case ValueList:
			for index, child := range value.List {
				if !walk(child, fmt.Sprintf("%s.list[%d]", path, index)) {
					return false
				}
			}
		case ValueSet:
			for index, child := range value.Set {
				if !walk(child, fmt.Sprintf("%s.set[%d]", path, index)) {
					return false
				}
			}
		}
		return true
	}

	for scopeIndex, scope := range vm.liveScopes() {
		rootIndex := 0
		for _, value := range scope {
			if !walk(value, fmt.Sprintf("scope[%d].root[%d]", scopeIndex, rootIndex)) {
				return false
			}
			rootIndex++
		}
	}
	classIndex := 0
	for _, class := range vm.ClassEntries() {
		fieldIndex := 0
		for _, field := range class.StaticFields {
			if !walk(field.Value, fmt.Sprintf("static[%d].field[%d]", classIndex, fieldIndex)) {
				return false
			}
			fieldIndex++
		}
		classIndex++
	}
	return true
}

func valueLength(value Value) int {
	switch value.Kind {
	case ValueList:
		return len(value.List)
	case ValueSet:
		return len(value.Set)
	default:
		return 0
	}
}

func valueCapacity(value Value) int {
	switch value.Kind {
	case ValueList:
		return cap(value.List)
	case ValueSet:
		return cap(value.Set)
	default:
		return 0
	}
}

func aliasShadowBackings(value Value) (uintptr, uintptr, uintptr) {
	switch value.Kind {
	case ValueObject:
		return mapIdentityPointer(value.Fields), 0, 0
	case ValueMap:
		return mapIdentityPointer(value.Map), mapIdentityPointer(value.MapKeys), aliasShadowSlicePointer(value.MapOrder)
	case ValueList:
		return aliasShadowSlicePointer(value.List), 0, 0
	case ValueSet:
		return aliasShadowSlicePointer(value.Set), aliasShadowSlicePointer(value.setInsertionHashes), 0
	}
	return 0, 0, 0
}

func aliasShadowSlicePointer[T any](values []T) uintptr {
	if values == nil || cap(values) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(unsafe.SliceData(values)))
}

func aliasShadowTestName() string {
	var pcs [32]uintptr
	count := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:count])
	for {
		frame, more := frames.Next()
		name := frame.Function
		if start := strings.Index(name, "Test"); start >= 0 {
			name = name[start:]
			if end := strings.IndexByte(name, '.'); end >= 0 {
				name = name[:end]
			}
			return name
		}
		if !more {
			break
		}
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.run=") {
			name := strings.TrimPrefix(arg, "-test.run=")
			if name != "" {
				return name
			}
		}
	}
	return "unknown"
}
