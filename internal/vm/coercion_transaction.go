package vm

import "reflect"

type coercionContainerIdentity struct {
	kind byte
	ptr  uintptr
}

type coercionMapSnapshot struct {
	target map[string]Value
	values map[string]Value
}

type coercionSliceSnapshot struct {
	target []Value
	values []Value
}

type coercionStringSliceSnapshot struct {
	target []string
	values []string
}

type coercionGraphRollback struct {
	seen         map[coercionContainerIdentity]bool
	maps         []coercionMapSnapshot
	slices       []coercionSliceSnapshot
	stringSlices []coercionStringSliceSnapshot
}

func captureCoercionMapBranches(value Value, rawKeys []string) coercionGraphRollback {
	var rollback coercionGraphRollback
	for _, rawKey := range rawKeys {
		rollback.captureValue(mapStoredKey(value, rawKey))
		rollback.captureValue(value.Map[rawKey])
	}
	return rollback
}

func (rollback *coercionGraphRollback) captureValue(value Value) {
	rollback.captureMap(value.Fields)
	rollback.captureSlice(value.List)
	rollback.captureSlice(value.Set)
	rollback.captureMap(value.Map)
	rollback.captureMap(value.MapKeys)
	rollback.captureStringSlice(value.MapOrder)
}

func (rollback *coercionGraphRollback) captureMap(target map[string]Value) {
	if target == nil {
		return
	}
	identity := coercionContainerIdentity{kind: 'm', ptr: reflect.ValueOf(target).Pointer()}
	if rollback.alreadyCaptured(identity) {
		return
	}
	values := make(map[string]Value, len(target))
	for key, value := range target {
		values[key] = value
	}
	rollback.maps = append(rollback.maps, coercionMapSnapshot{target: target, values: values})
	for _, value := range values {
		rollback.captureValue(value)
	}
}

func (rollback *coercionGraphRollback) captureSlice(target []Value) {
	if target == nil {
		return
	}
	identity := coercionContainerIdentity{kind: 's', ptr: reflect.ValueOf(target).Pointer()}
	if rollback.alreadyCaptured(identity) {
		return
	}
	values := append([]Value(nil), target...)
	rollback.slices = append(rollback.slices, coercionSliceSnapshot{target: target, values: values})
	for _, value := range values {
		rollback.captureValue(value)
	}
}

func (rollback *coercionGraphRollback) captureStringSlice(target []string) {
	if target == nil {
		return
	}
	identity := coercionContainerIdentity{kind: 'o', ptr: reflect.ValueOf(target).Pointer()}
	if rollback.alreadyCaptured(identity) {
		return
	}
	rollback.stringSlices = append(rollback.stringSlices, coercionStringSliceSnapshot{
		target: target,
		values: append([]string(nil), target...),
	})
}

func (rollback *coercionGraphRollback) alreadyCaptured(identity coercionContainerIdentity) bool {
	if rollback.seen == nil {
		rollback.seen = make(map[coercionContainerIdentity]bool)
	}
	if rollback.seen[identity] {
		return true
	}
	rollback.seen[identity] = true
	return false
}

func (rollback *coercionGraphRollback) restore() {
	for _, snapshot := range rollback.maps {
		clear(snapshot.target)
		for key, value := range snapshot.values {
			snapshot.target[key] = value
		}
	}
	for _, snapshot := range rollback.slices {
		copy(snapshot.target, snapshot.values)
	}
	for _, snapshot := range rollback.stringSlices {
		copy(snapshot.target, snapshot.values)
	}
}
