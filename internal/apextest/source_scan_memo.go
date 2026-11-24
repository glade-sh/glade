package apextest

import (
	"crypto/sha256"
	"sync"
	"sync/atomic"
)

// sourceScanMemoEnabled turns on the per-source scan memos. A long-lived
// caller (the test daemon) re-runs the project-wide source scans after every
// edit; one-shot runs scan each source once and keep no memo.
var sourceScanMemoEnabled atomic.Bool

// EnableSourceScanMemo makes the project-wide Apex source scans reuse their
// per-source results across runs. Results are keyed by the source text, so an
// edit, delete or rename only changes which keys are asked for, and every
// org- or configuration-dependent step still runs on each call.
func EnableSourceScanMemo() { sourceScanMemoEnabled.Store(true) }

// backgroundPublications serializes startup-cache writes that runs with
// Options.BackgroundDiskPublication hand off: the memory cache already serves
// the runtime, and the disk copy only speeds a later process start. Each
// write takes a sequence number when its run hands it off; a write older than
// one already written for the same project root is dropped, so a slow write
// cannot replace a newer entry.
var (
	backgroundPublications     sync.Mutex
	backgroundPublicationsDone sync.WaitGroup
	backgroundPublicationSeq   atomic.Uint64
	backgroundPublished        = map[string]uint64{}
)

func publishInBackground(root string, publish func()) {
	write := orderedPublication(root, publish)
	backgroundPublicationsDone.Add(1)
	go func() {
		defer backgroundPublicationsDone.Done()
		write()
	}()
}

func orderedPublication(root string, publish func()) func() {
	seq := backgroundPublicationSeq.Add(1)
	return func() {
		backgroundPublications.Lock()
		defer backgroundPublications.Unlock()
		if backgroundPublished[root] > seq {
			return
		}
		backgroundPublished[root] = seq
		publish()
	}
}

// sourceScanMemoLimit and sourceScanMemoByteLimit bound a memo by entries
// and by the source bytes its entries were computed from (each entry holds
// data proportional to its source); past either the memo starts over. Every
// edit of a file adds an entry, so a long-lived daemon needs both bounds.
const (
	sourceScanMemoLimit     = 1 << 15
	sourceScanMemoByteLimit = 256 << 20
)

// sourceScanMemo memoizes a pure function of one source text.
type sourceScanMemo[T any] struct {
	mu      sync.Mutex
	entries map[[sha256.Size]byte]T
	bytes   int
}

func (m *sourceScanMemo[T]) get(source string, compute func(string) T) T {
	if !sourceScanMemoEnabled.Load() {
		return compute(source)
	}
	key := sha256.Sum256([]byte(source))
	m.mu.Lock()
	value, ok := m.entries[key]
	m.mu.Unlock()
	if ok {
		return value
	}
	value = compute(source)
	m.mu.Lock()
	if m.entries == nil || len(m.entries) >= sourceScanMemoLimit || m.bytes+len(source) > sourceScanMemoByteLimit {
		m.entries = make(map[[sha256.Size]byte]T)
		m.bytes = 0
	}
	if _, present := m.entries[key]; !present {
		m.bytes += len(source)
	}
	m.entries[key] = value
	m.mu.Unlock()
	return value
}

func (m *sourceScanMemo[T]) len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// standardFieldSourceMatches holds every pattern match the standard-field
// scan takes from one source. It depends on the source alone; the scan
// replays it against the org on each call.
type standardFieldSourceMatches struct {
	childRefs     []projectChildRelationshipSourceReference
	scanSource    string
	typed         [][]string
	booleanRight  [][]int
	booleanLeft   [][]int
	booleanNot    [][]int
	numericRight  [][]int
	numericLeft   [][]int
	getAll        [][]int
	schemaFields  [][]int
	sobjectFields [][]int
	newLiterals   [][]int
	literals      [][]int
	staticFields  [][]int
	variableField [][]int
}

var (
	standardFieldMatchMemo sourceScanMemo[*standardFieldSourceMatches]
	recordTypeScanMemo     sourceScanMemo[[]projectRecordTypeReference]
)

func standardFieldMatchesFromSource(source string) *standardFieldSourceMatches {
	scanSource := apexReferenceScanSource(source)
	return &standardFieldSourceMatches{
		childRefs:     projectChildRelationshipSourceReferences(source),
		scanSource:    scanSource,
		typed:         apexTypedVariablePattern.FindAllStringSubmatch(scanSource, -1),
		booleanRight:  apexVariableFieldBooleanRightLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1),
		booleanLeft:   apexVariableFieldBooleanLeftLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1),
		booleanNot:    apexVariableFieldBooleanNegationPattern.FindAllStringSubmatchIndex(scanSource, -1),
		numericRight:  apexVariableFieldNumericRightLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1),
		numericLeft:   apexVariableFieldNumericLeftLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1),
		getAll:        apexCustomSettingGetAllPattern.FindAllStringSubmatchIndex(scanSource, -1),
		schemaFields:  apexSchemaSObjectTypeFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1),
		sobjectFields: apexSObjectTypeFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1),
		newLiterals:   apexNewSObjectLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1),
		literals:      apexSObjectLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1),
		staticFields:  apexStaticFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1),
		variableField: apexVariableFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1),
	}
}

// fieldSetConstantMemo holds the field set constant matches of one source;
// most sources have none, and the org-dependent steps run per call.
var fieldSetConstantMemo sourceScanMemo[[][]string]

func fieldSetConstantMatches(source string) [][]string {
	return apexFieldSetConstantPattern.FindAllStringSubmatch(source, -1)
}

func scanRecordTypesFromApexSourceMemo(source string) []projectRecordTypeReference {
	return recordTypeScanMemo.get(source, scanRecordTypesFromApexSource)
}
