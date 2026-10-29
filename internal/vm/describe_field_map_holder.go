package vm

// SObject field maps returned by Schema.SObjectFieldMap.getMap share one
// frozen storage tuple (Map, MapKeys, MapOrder) with the describe wrapper and
// every other returned holder of this VM. Each participant carries its own
// ownership descriptor in mapKeySetSource: the Ref it belongs to and the frozen
// map that its alias headers name. Copies of a header share the descriptor, so
// when a write detaches one holder (prepareSharedMapMutation or an
// entry-changing coercion), its detached tuple is recorded there and every
// true alias, including Values held outside VM scopes, adopts it at the read
// boundaries that already normalize live key sets. Other holders and the
// wrapper keep the frozen tuple. Field tokens stay shallowly shared within the
// VM exactly as eager copies share them; nothing here crosses a VM, because
// clones copy Values through cloneValue, which drops the descriptor.

// fieldMapHolderDescriptor reports the ownership descriptor of value, if it is
// a field-map participant.
func fieldMapHolderDescriptor(value Value) *mapKeySetBacking {
	if value.Kind != ValueMap || value.mapKeySetSource == nil || value.mapKeySetSource.frozen == nil {
		return nil
	}
	return value.mapKeySetSource
}

// currentFieldMapHolder returns value with the storage its holder detached,
// when value is a stale alias header that still names the frozen tuple.
func (value Value) currentFieldMapHolder() Value {
	descriptor := fieldMapHolderDescriptor(value)
	if descriptor == nil || descriptor.value.Map == nil || descriptor.value.Ref != value.Ref ||
		!sameMapIdentity(value.Map, descriptor.frozen) {
		return value
	}
	value.Map = descriptor.value.Map
	value.MapKeys = descriptor.value.MapKeys
	value.MapOrder = descriptor.value.MapOrder
	value.mapBackingShared = false
	value.globalDescribeEntriesCanonical = false
	return value
}

// namesFrozenFieldMap reports a header whose storage is still the frozen tuple
// of its descriptor. Private copies made from a holder (clone, eager copies)
// keep their own storage and are written in place as before.
func namesFrozenFieldMap(value Value) bool {
	descriptor := fieldMapHolderDescriptor(value)
	return descriptor != nil && sameMapIdentity(value.Map, descriptor.frozen)
}

// recordFieldMapHolderDetach publishes the private tuple of a detached holder
// to the aliases that share its descriptor. Copies with another Ref (clones,
// coercion results with a new identity) never write into a descriptor.
func recordFieldMapHolderDetach(detached Value) {
	descriptor := fieldMapHolderDescriptor(detached)
	if descriptor == nil || descriptor.value.Ref != detached.Ref || descriptor.value.Map != nil {
		return
	}
	descriptor.value.Map = detached.Map
	descriptor.value.MapKeys = detached.MapKeys
	descriptor.value.MapOrder = detached.MapOrder
}

// frozenFieldMapParticipant marks value as sharing frozen storage, with its own
// descriptor for the Ref it carries.
func frozenFieldMapParticipant(value Value) Value {
	value.mapBackingShared = true
	value.globalDescribeEntriesCanonical = false
	value.mapKeySetSource = &mapKeySetBacking{value: Value{Kind: ValueMap, Ref: value.Ref}, frozen: value.Map}
	return value
}

// fieldMapSourceFrozen reports whether a wrapper's field map still names the
// frozen tuple its descriptor was created for.
func fieldMapSourceFrozen(source Value) bool {
	descriptor := fieldMapHolderDescriptor(source)
	return descriptor != nil && source.mapBackingShared && descriptor.value.Ref == source.Ref &&
		descriptor.value.Map == nil && sameMapIdentity(source.Map, descriptor.frozen)
}

// provenSObjectFieldMap reports a wrapper map built by describeSObjectFieldMap:
// the Schema.SObjectFieldMap wrapper type, the platform map provenance, and a
// canonical key representation.
func provenSObjectFieldMap(owner, source Value) bool {
	if owner.Kind != ValueObject || owner.Type != "Schema.SObjectFieldMap" || source.Kind != ValueMap || source.Ref == 0 || source.Map == nil {
		return false
	}
	if _, ok := sObjectFieldMapObjectName(source); !ok {
		return false
	}
	return len(source.MapKeys) == len(source.Map) && len(source.MapOrder) == len(source.Map)
}

// sharedSObjectFieldMapHolder returns a new holder identity over the frozen
// storage of a proven wrapper map. ok is false when the source is not frozen
// shared storage; the caller then keeps the eager private copy.
func sharedSObjectFieldMapHolder(source Value) (Value, bool) {
	if !fieldMapSourceFrozen(source) {
		return Value{}, false
	}
	holder := source
	holder.Ref = newValueRef()
	return frozenFieldMapParticipant(holder), true
}

// normalizeFieldMapHolderArgs lets previously evaluated map arguments observe a
// detach of their holder before a member reads them.
func normalizeFieldMapHolderArgs(args []Value) {
	for index := range args {
		if args[index].Kind == ValueMap && args[index].mapKeySetSource != nil {
			args[index] = args[index].currentFieldMapHolder()
		}
	}
}
