package vm

import "strings"

func (vm *VM) callGenericCollectionStaticMember(methodName string, args []Value) (Value, bool, error) {
	if strings.EqualFold(methodName, "findSObjectInListById") {
		if len(args) != 2 {
			return Null, true, nil
		}
		return findSObjectInListByID(args[0], args[1]), true, nil
	}
	return Null, false, nil
}

func findSObjectInListByID(idValue, records Value) Value {
	id, ok := sObjectIDFromValue(idValue)
	if !ok || id == "" || records.Kind != ValueList {
		return Null
	}
	for _, record := range records.List {
		if record.Kind != ValueObject {
			continue
		}
		recordID := sObjectIDFromFields(record.Fields)
		if recordID != "" && apexIDTextEqual(string(recordID), string(id)) {
			return record
		}
	}
	return Null
}
