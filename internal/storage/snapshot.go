package storage

import "reflect"

func (t RuntimeTemplate) CloneSnapshotOrg() OrgState {
	cloneStats.cloneRuntime.Add(1)
	out := t.Org
	if t.Org.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(t.Org.Objects))
		for name, object := range t.Org.Objects {
			out.Objects[name] = object.CloneRuntimeSnapshot()
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

func SnapshotRuntimeOrg(org *OrgState) OrgState {
	cloneStats.cloneRollbackSnapshot.Add(1)
	if org == nil {
		return OrgState{}
	}
	out := *org
	if org.Objects != nil {
		out.Objects = make(map[string]ObjectState, len(org.Objects))
		for name, object := range org.Objects {
			live := object
			snapshot := object
			if object.Records != nil {
				live.RecordsShared = true
				snapshot.RecordsShared = true
			}
			if object.Indexes != nil {
				live.IndexesShared = true
				snapshot.IndexesShared = true
			}
			org.Objects[name] = live
			out.Objects[name] = snapshot
		}
	}
	if org.IDSequences != nil {
		out.IDSequences = make(map[string]uint64, len(org.IDSequences))
		for object, sequence := range org.IDSequences {
			out.IDSequences[object] = sequence
		}
	}
	if org.Transactions != nil {
		out.Transactions = make([]TransactionFrame, len(org.Transactions))
		for i, transaction := range org.Transactions {
			out.Transactions[i] = transaction.Clone()
		}
	}
	return out
}

func (o ObjectState) CloneRuntimeSnapshot() ObjectState {
	out := o
	out.RecordsShared = o.Records != nil
	out.IndexesShared = o.Indexes != nil
	return out
}

func EnsureMutableObjectRecords(org *OrgState, objectName string) (*ObjectState, bool) {
	if org == nil {
		return nil, false
	}
	canonical, ok := ResolveObjectName(*org, objectName)
	if !ok {
		return nil, false
	}
	object := org.Objects[canonical]
	cloned := false
	if object.RecordsShared {
		object.Records = cloneRecordMap(object.Records)
		object.RecordsShared = false
		cloned = true
	}
	if object.IndexesShared {
		object.Indexes = cloneIndexMap(object.Indexes)
		object.IndexesShared = false
		cloned = true
	}
	org.Objects[canonical] = object
	return &object, cloned
}

func cloneRecordMap(records map[ID]Record) map[ID]Record {
	if records == nil {
		return nil
	}
	out := make(map[ID]Record, len(records))
	for id, record := range records {
		out[id] = record.Clone()
	}
	return out
}

func cloneIndexMap(indexes map[string]IndexSet) map[string]IndexSet {
	if indexes == nil {
		return nil
	}
	out := make(map[string]IndexSet, len(indexes))
	for name, index := range indexes {
		out[name] = index.Clone()
	}
	return out
}

func sameRecordMap(left, right map[ID]Record) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
