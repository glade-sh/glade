package storage

import "fmt"

type IsolationJournal struct {
	org       *OrgState
	inserted  []journalRecordKey
	updated   []journalRecordBefore
	sequences []journalSequenceBefore
}

type IsolationMark struct {
	inserted  int
	updated   int
	sequences int
}

type journalRecordKey struct {
	object string
	id     ID
}

type journalRecordBefore struct {
	object string
	id     ID
	record Record
	exists bool
}

type journalSequenceBefore struct {
	object string
	value  uint64
	exists bool
}

func NewIsolationJournal(org *OrgState) *IsolationJournal {
	return &IsolationJournal{org: org}
}

func (j *IsolationJournal) Org() *OrgState {
	if j == nil {
		return nil
	}
	return j.org
}

func (j *IsolationJournal) Mark() IsolationMark {
	if j == nil {
		return IsolationMark{}
	}
	return IsolationMark{
		inserted:  len(j.inserted),
		updated:   len(j.updated),
		sequences: len(j.sequences),
	}
}

func (j *IsolationJournal) RecordSequence(object string) {
	if j == nil || j.org == nil || object == "" {
		return
	}
	if j.sequenceRecordedSinceMark(object) {
		return
	}
	value, exists := j.org.IDSequences[object]
	j.sequences = append(j.sequences, journalSequenceBefore{object: object, value: value, exists: exists})
}

func (j *IsolationJournal) RecordInsert(object string, id ID) {
	if j == nil || object == "" || id == "" {
		return
	}
	j.inserted = append(j.inserted, journalRecordKey{object: object, id: id})
}

func (j *IsolationJournal) RecordUpdate(object string, id ID, before Record) {
	if j == nil || object == "" || id == "" {
		return
	}
	if j.recordInsertRecorded(object, id) {
		return
	}
	if j.recordBeforeRecorded(object, id) {
		return
	}
	j.updated = append(j.updated, journalRecordBefore{
		object: object,
		id:     id,
		record: before.Clone(),
		exists: true,
	})
}

func (j *IsolationJournal) Rollback(mark IsolationMark) error {
	if j == nil || j.org == nil {
		return nil
	}
	for i := len(j.inserted) - 1; i >= mark.inserted; i-- {
		key := j.inserted[i]
		object := j.org.Objects[key.object]
		if object.Records != nil {
			delete(object.Records, key.id)
			j.org.Objects[key.object] = object
		}
	}
	for i := len(j.updated) - 1; i >= mark.updated; i-- {
		before := j.updated[i]
		object, ok := j.org.Objects[before.object]
		if !ok {
			return fmt.Errorf("isolation journal rollback missing object %s", before.object)
		}
		if object.Records == nil {
			object.Records = make(map[ID]Record)
		}
		if before.exists {
			object.Records[before.id] = before.record.Clone()
		} else {
			delete(object.Records, before.id)
		}
		j.org.Objects[before.object] = object
	}
	for i := len(j.sequences) - 1; i >= mark.sequences; i-- {
		before := j.sequences[i]
		if j.org.IDSequences == nil {
			j.org.IDSequences = make(map[string]uint64)
		}
		if before.exists {
			j.org.IDSequences[before.object] = before.value
		} else {
			delete(j.org.IDSequences, before.object)
		}
	}
	j.inserted = j.inserted[:mark.inserted]
	j.updated = j.updated[:mark.updated]
	j.sequences = j.sequences[:mark.sequences]
	return nil
}

func (j *IsolationJournal) recordBeforeRecorded(object string, id ID) bool {
	for i := len(j.updated) - 1; i >= 0; i-- {
		before := j.updated[i]
		if before.object == object && before.id == id {
			return true
		}
	}
	return false
}

func (j *IsolationJournal) recordInsertRecorded(object string, id ID) bool {
	for i := len(j.inserted) - 1; i >= 0; i-- {
		inserted := j.inserted[i]
		if inserted.object == object && inserted.id == id {
			return true
		}
	}
	return false
}

func (j *IsolationJournal) sequenceRecordedSinceMark(object string) bool {
	for i := len(j.sequences) - 1; i >= 0; i-- {
		if j.sequences[i].object == object {
			return true
		}
	}
	return false
}
