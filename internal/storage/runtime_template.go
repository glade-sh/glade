package storage

import "reflect"

type RuntimeTemplate struct {
	Org OrgState
}

func NewRuntimeTemplate(org OrgState) RuntimeTemplate {
	return RuntimeTemplate{Org: org}
}

func (t RuntimeTemplate) CloneRuntimeOrg() OrgState {
	cloneStats.cloneRuntime.Add(1)
	out := t.Org
	if t.Org.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(t.Org.Objects))
		for name, object := range t.Org.Objects {
			out.Objects[name] = object.CloneRuntimeFrozenDefinition()
		}
	}
	if t.Org.IDSequences != nil {
		out.IDSequences = make(map[string]uint64, len(t.Org.IDSequences))
		for object, sequence := range t.Org.IDSequences {
			out.IDSequences[object] = sequence
		}
	}
	if t.Org.Transactions != nil {
		out.Transactions = make([]TransactionFrame, len(t.Org.Transactions))
		for i, transaction := range t.Org.Transactions {
			out.Transactions[i] = transaction.Clone()
		}
	}
	return out
}

func (o ObjectState) CloneRuntimeFrozenDefinition() ObjectState {
	out := o
	if o.Records != nil {
		out.Records = make(map[ID]Record, len(o.Records))
		for id, record := range o.Records {
			out.Records[id] = record.Clone()
		}
	}
	if o.Indexes != nil {
		out.Indexes = make(map[string]IndexSet, len(o.Indexes))
		for name, index := range o.Indexes {
			out.Indexes[name] = index.Clone()
		}
	}
	return out
}

func EnsureMutableObjectDefinition(org *OrgState, objectName string) (*ObjectDefinition, bool) {
	if org == nil {
		return nil, false
	}
	canonical, ok := ResolveObjectName(*org, objectName)
	if !ok {
		return nil, false
	}
	object := org.Objects[canonical]
	object.Definition = object.Definition.Clone()
	org.Objects[canonical] = object
	return &object.Definition, true
}

func sameFieldMap(left, right map[string]Field) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
