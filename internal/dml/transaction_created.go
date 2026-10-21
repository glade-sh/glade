package dml

import "github.com/glade-sh/glade/internal/storage"

// RecordCreations tracks inserts in one transaction without retaining the org's
// existing records. Checkpoints only record the length of the insertion log.
type RecordCreations struct {
	ids   map[storage.ID]int
	order []storage.ID
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
		}
	}
	clear(r.order[mark:])
	r.order = r.order[:mark]
}
