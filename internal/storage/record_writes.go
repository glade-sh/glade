package storage

import "sync/atomic"

// recordsClock is the write counter of one ObjectState.Records map. Every copy
// of an ObjectState that shares the map also shares the clock, so a writer
// that holds a stale struct copy still advances the version every reader
// sees. A new map always gets a new clock, so a clock identity plus a version
// names one exact content of one map.
type recordsClock struct {
	version atomic.Uint64
}

// RecordsToken names the content of an ObjectState.Records map at one moment.
// Two equal tokens mean no write went through the funnel between them. It is
// comparable and keeps its clock alive, so it is safe as a cache key.
type RecordsToken struct {
	clock   *recordsClock
	version uint64
}

// Valid reports whether the token came from a versioned object.
func (t RecordsToken) Valid() bool { return t.clock != nil }

// RecordsToken returns the current token of the record map. It reports false
// when the object has no clock, as a struct literal or a decoded snapshot does
// until it is cloned or its map is replaced through SetRecords. An unversioned
// object must not be cached.
func (o ObjectState) RecordsToken() (RecordsToken, bool) {
	if o.recordsClock == nil {
		return RecordsToken{}, false
	}
	return RecordsToken{clock: o.recordsClock, version: o.recordsClock.version.Load()}, true
}

// RecordsVersion returns the write version of the record map, or 0 when the
// object is unversioned.
func (o ObjectState) RecordsVersion() uint64 {
	if o.recordsClock == nil {
		return 0
	}
	return o.recordsClock.version.Load()
}

// The methods below are the only code that may write ObjectState.Records.
// A repoguard test rejects a direct write anywhere else. Each one advances the
// version of the map it writes.

// SetRecords replaces the record map. The new map gets its own clock, so
// tokens taken before the call never match tokens taken after it.
func (o *ObjectState) SetRecords(records map[ID]Record) {
	o.Records = records
	if records == nil {
		o.recordsClock = nil
		return
	}
	o.recordsClock = &recordsClock{}
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
func (o ObjectState) TouchRecords() {
	if o.recordsClock != nil {
		o.recordsClock.version.Add(1)
	}
}

// EnsureRecordVersions gives every object with a record map a clock.
// Unversioned objects stay uncacheable until they have one.
func (o *OrgState) EnsureRecordVersions() {
	if o == nil {
		return
	}
	for name, object := range o.Objects {
		if object.Records != nil && object.recordsClock == nil {
			object.recordsClock = &recordsClock{}
			o.Objects[name] = object
		}
	}
}
