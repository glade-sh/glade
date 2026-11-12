package storage

import (
	"maps"
	"reflect"
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
//
// Contract limit: the token covers the map's keys and the Record values stored
// in it through the funnel. Records are copied shallow, so a Record's Fields,
// ExplicitNulls and other maps or slices are shared with every other copy of
// that record, including copies held by another object after SetRecords or by
// a caller that passed the record to PutRecord. Editing those in place changes
// every holder's rows, and only the holder that then calls PutRecord or
// TouchRecords sees its token move. A consumer that needs row content, not row
// membership, must clone the record before editing it and must not reuse a
// token across objects that may share rows.
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

// SetRecords replaces the record map with a copy of records. The copy keeps a
// map from ever sharing a clock with, or escaping the clock of, another map:
// the caller's map stays the caller's, and writes through it are not tracked.
// A versioned object gets a new clock for the new map, so tokens taken before
// the call never match tokens taken after it. An object that had no map gets a
// clock too. An object whose existing map was never versioned, such as a
// struct literal in a test, stays unversioned, so clones and rebuilds of it
// still compare equal to it. Passing the object's own current map is a no-op.
func (o *ObjectState) SetRecords(records map[ID]Record) {
	if records != nil && o.Records != nil && reflect.ValueOf(records).UnsafePointer() == reflect.ValueOf(o.Records).UnsafePointer() {
		return
	}
	o.adoptRecords(maps.Clone(records))
}

// adoptRecords takes ownership of a map the caller has just built and holds no
// other reference to. Storage code uses it where a copy would double the cost
// of a clone.
func (o *ObjectState) adoptRecords(records map[ID]Record) {
	versioned := o.Records == nil || o.recordsClock != nil
	o.Records = records
	if records == nil || !versioned {
		o.recordsClock = nil
		return
	}
	o.recordsClock = newRecordsClock()
}

// PutRecord stores a record under id, creating the map when it is nil. The
// record is stored shallow; see the RecordsToken contract.
func (o *ObjectState) PutRecord(id ID, record Record) {
	if o.Records == nil {
		o.adoptRecords(make(map[ID]Record))
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
