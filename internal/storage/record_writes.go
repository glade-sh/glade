package storage

import (
	"sync/atomic"
	"unsafe"
)

// recordsClock is the write counter of one ObjectState.Records map. Every copy
// of an ObjectState that shares the map also shares the clock, so a writer
// that holds a stale struct copy still advances the version every reader
// sees. A new map always gets a new clock, so a clock identity plus a version
// names one exact content of one map.
//
// ObjectState holds the clock as a pointer to its zero-size first field. The
// struct stays equal under reflect.DeepEqual whatever the version, which
// existing tests rely on when they compare cloned and rebuilt object states.
// Two states still differ when only one of them is versioned.
type recordsClock struct {
	tag     recordsClockTag
	version atomic.Uint64
}

type recordsClockTag struct{}

func newRecordsClock() *recordsClockTag {
	clock := &recordsClock{}
	return &clock.tag
}

func clockOf(tag *recordsClockTag) *recordsClock {
	if tag == nil {
		return nil
	}
	return (*recordsClock)(unsafe.Pointer(tag))
}

// RecordsToken names the content of an ObjectState.Records map at one moment.
// Two equal tokens mean no write went through the funnel between them. It is
// comparable and keeps its clock alive, so it is safe as a cache key.
type RecordsToken struct {
	clock   *recordsClockTag
	version uint64
}

// Valid reports whether the token came from a versioned object.
func (t RecordsToken) Valid() bool { return t.clock != nil }

// RecordsToken returns the current token of the record map. It reports false
// when the object has no clock, as a struct literal or a decoded snapshot does
// until it is cloned or its map is replaced through SetRecords. An unversioned
// object must not be cached.
func (o ObjectState) RecordsToken() (RecordsToken, bool) {
	clock := clockOf(o.recordsClock)
	if clock == nil {
		return RecordsToken{}, false
	}
	return RecordsToken{clock: o.recordsClock, version: clock.version.Load()}, true
}

// RecordsVersion returns the write version of the record map, or 0 when the
// object is unversioned.
func (o ObjectState) RecordsVersion() uint64 {
	if clock := clockOf(o.recordsClock); clock != nil {
		return clock.version.Load()
	}
	return 0
}

// The methods below are the only code that may write ObjectState.Records.
// A repoguard test rejects a direct write anywhere else. Each one advances the
// version of the map it writes.

// SetRecords replaces the record map. A versioned object gets a new clock for
// the new map, so tokens taken before the call never match tokens taken after
// it. An object that had no map gets a clock too. An object whose existing map
// was never versioned, such as a struct literal in a test, stays unversioned,
// so clones and rebuilds of it still compare equal to it.
func (o *ObjectState) SetRecords(records map[ID]Record) {
	versioned := o.Records == nil || o.recordsClock != nil
	o.Records = records
	if records == nil || !versioned {
		o.recordsClock = nil
		return
	}
	o.recordsClock = newRecordsClock()
}

// PutRecord stores a record under id, creating the map when it is nil.
func (o *ObjectState) PutRecord(id ID, record Record) {
	if o.Records == nil {
		o.SetRecords(make(map[ID]Record))
	}
	o.Records[id] = record
	o.TouchRecords()
}

// DeleteRecord removes the record under id. It does nothing when the id is
// absent, which leaves the version unchanged.
func (o *ObjectState) DeleteRecord(id ID) {
	if _, ok := o.Records[id]; !ok {
		return
	}
	delete(o.Records, id)
	o.TouchRecords()
}

// ClearRecords removes every record and keeps the map.
func (o *ObjectState) ClearRecords() {
	if len(o.Records) == 0 {
		return
	}
	clear(o.Records)
	o.TouchRecords()
}

// TouchRecords advances the version. A writer that mutates a record map
// through an alias, or mutates a stored record in place, calls it after the
// write.
func (o *ObjectState) TouchRecords() {
	if clock := clockOf(o.recordsClock); clock != nil {
		clock.version.Add(1)
	}
}
