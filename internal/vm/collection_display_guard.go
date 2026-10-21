package vm

// Crash guard only, native behavior uncaptured. A List, Set or Map that holds
// itself, directly or through other collections, made Glade's collection text
// (mapString, valuesString, displayList, displaySet) recurse until the Go stack
// overflowed and the whole process died. Such a value now fails display with
// this Glade error instead. It makes no claim about the Salesforce answer.
const recursiveCollectionDisplayMessage = "Glade: cannot display a List, Set or Map that contains itself (crash guard only; native behavior uncaptured)"

func recursiveCollectionDisplayError() error {
	return &RuntimeError{Type: "UnsupportedFeature", Message: recursiveCollectionDisplayMessage}
}

// collectionDisplayRecurses reports whether a collection reaches its own
// backing store through nested collection elements. Object fields are not
// followed: object text already stops on a repeated object Ref. Collections
// without a nested non-empty collection, the common case, return at once
// without allocating, and no display text changes for any other value.
func collectionDisplayRecurses(value Value) bool {
	if !hasNestedNonEmptyCollection(value) {
		return false
	}
	active := make(map[jsonSerializationIdentity]bool)
	var visit func(Value) bool
	visit = func(item Value) bool {
		if !isNonEmptyCollection(item) {
			return false
		}
		identity, ok := jsonSerializationToken(item)
		if !ok {
			return false
		}
		if active[identity] {
			return true
		}
		active[identity] = true
		defer delete(active, identity)
		for _, child := range collectionDisplayChildren(item) {
			if visit(child) {
				return true
			}
		}
		for _, child := range item.Map {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(value)
}

func hasNestedNonEmptyCollection(value Value) bool {
	for _, child := range collectionDisplayChildren(value) {
		if isNonEmptyCollection(child) {
			return true
		}
	}
	for _, child := range value.Map {
		if isNonEmptyCollection(child) {
			return true
		}
	}
	return false
}

// collectionDisplayChildren returns the List or Set elements of a collection.
// Map values are walked separately by the callers.
func collectionDisplayChildren(value Value) []Value {
	switch value.Kind {
	case ValueList:
		return value.List
	case ValueSet:
		return value.Set
	}
	return nil
}

func isNonEmptyCollection(value Value) bool {
	switch value.Kind {
	case ValueList:
		return len(value.List) > 0
	case ValueSet:
		return len(value.Set) > 0
	case ValueMap:
		return len(value.Map) > 0
	}
	return false
}
