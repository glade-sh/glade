package storage

// CloneCopyOnWrite returns a staging copy of the org for one write request. It
// copies the object index, the ID sequences and the transaction frames, and
// shares every object definition, record map and index with the receiver. Each
// copied object is marked RecordsShared and IndexesShared, so the first write
// that goes through EnsureMutableObjectRecords copies only that object's
// records. Definitions must go through EnsureMutableObjectDefinition. The
// receiver is never mutated, so a failed or abandoned request leaves it as it
// was and concurrent readers of the receiver stay valid.
func (o OrgState) CloneCopyOnWrite() OrgState {
	out := o.CloneObjectsShallow(0)
	out.Metadata.FeatureParameters = append([]FeatureParameterMetadata(nil), o.Metadata.FeatureParameters...)
	if o.IDSequences != nil {
		out.IDSequences = make(map[string]uint64, len(o.IDSequences))
		for object, sequence := range o.IDSequences {
			out.IDSequences[object] = sequence
		}
	}
	if o.Transactions != nil {
		out.Transactions = make([]TransactionFrame, len(o.Transactions))
		for i, transaction := range o.Transactions {
			out.Transactions[i] = transaction.Clone()
		}
	}
	return out
}
