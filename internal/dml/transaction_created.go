package dml

import "github.com/glade-sh/glade/internal/storage"

// RecordCreations tracks inserts in one transaction without retaining the org's
// existing records. Checkpoints only record the length of the insertion log.
type RecordCreations struct {
	ids   map[storage.ID]int
	order []storage.ID
	// baselines holds a created record's state at its first duplicate-rule
	// checked update in the transaction (see Engine.updateGateBaseline).
	baselines map[storage.ID]storage.Record
	// duplicates holds the created records whose insert matched a duplicate
	// (see Engine.updateGateBaseline).
	duplicates map[storage.ID]bool
}

// MarkDuplicate notes that a created record was inserted as an allowed duplicate.
func (r *RecordCreations) MarkDuplicate(id storage.ID) {
	if r == nil || !r.Contains(id) {
		return
	}
	if r.duplicates == nil {
		r.duplicates = make(map[storage.ID]bool)
	}
	r.duplicates[creationID(id)] = true
}

// InsertedAsDuplicate reports whether MarkDuplicate noted the record.
func (r *RecordCreations) InsertedAsDuplicate(id storage.ID) bool {
	return r != nil && r.duplicates[creationID(id)]
}

// Baseline returns the state recorded by SetBaseline for a created record.
func (r *RecordCreations) Baseline(id storage.ID) (storage.Record, bool) {
	if r == nil {
		return storage.Record{}, false
	}
	record, ok := r.baselines[creationID(id)]
	return record, ok
}

// SetBaseline records the state of a record created in this transaction.
func (r *RecordCreations) SetBaseline(id storage.ID, record storage.Record) {
	if r == nil || !r.Contains(id) {
		return
	}
	if r.baselines == nil {
		r.baselines = make(map[storage.ID]storage.Record)
	}
	r.baselines[creationID(id)] = record
}

func creationID(id storage.ID) storage.ID {
	if len(id) > 15 {
		return id[:15]
	}
	return id
}

func (r *RecordCreations) Contains(id storage.ID) bool {
	if r == nil || id == "" {
		return false
	}
	_, ok := r.ids[creationID(id)]
	return ok
}

func (r *RecordCreations) Record(id storage.ID) {
	if r == nil || id == "" || r.Contains(id) {
		return
	}
	if r.ids == nil {
		r.ids = make(map[storage.ID]int)
	}
	id = creationID(id)
	r.ids[id] = len(r.order)
	r.order = append(r.order, id)
}

func (r *RecordCreations) Forget(id storage.ID) {
	if r != nil {
		delete(r.ids, creationID(id))
		delete(r.baselines, creationID(id))
		delete(r.duplicates, creationID(id))
	}
}

func (r *RecordCreations) Mark() int {
	if r == nil {
		return 0
	}
	return len(r.order)
}

func (r *RecordCreations) Rollback(mark int) {
	if r == nil || mark < 0 || mark > len(r.order) {
		return
	}
	for i := len(r.order) - 1; i >= mark; i-- {
		id := r.order[i]
		if index, ok := r.ids[id]; ok && index == i {
			delete(r.ids, id)
			delete(r.baselines, id)
			delete(r.duplicates, id)
		}
	}
	clear(r.order[mark:])
	r.order = r.order[:mark]
}
