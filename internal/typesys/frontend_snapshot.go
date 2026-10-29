package typesys

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// FrontendSnapshotVersion is local to the frontend disk representation. It is
// deliberately independent of the semantic and platform ABIs.
const FrontendSnapshotVersion = 6

// These wire types explicitly retain fields excluded from public symbol JSON.
// JSON also preserves optional false/zero scalar pointers, unlike raw Gob.
type frontendType struct {
	TypeSymbol
	ConstructorsAuthoritative bool   `json:"constructorsAuthoritative"`
	EnumHashBase              *int64 `json:"enumHashBase"`
}
type frontendField struct {
	schema.Field
	File                          string `json:"file"`
	ChildRelationshipNameInferred bool   `json:"childRelationshipNameInferred"`
}
type frontendObject struct {
	schema.Object
	GeneratedHistoryParent string          `json:"generatedHistoryParent"`
	Fields                 []frontendField `json:"fields"`
}
type frontendIndex struct {
	Index
	Types                 []frontendType                `json:"types"`
	Objects               []frontendObject              `json:"objects"`
	CustomMetadataRecords []schema.CustomMetadataRecord `json:"customMetadataRecords"`
}
type frontendOccurrence struct {
	Metadata     SourceMetadata
	Chunk        string
	ApexMetadata ApexMetadataInput
}
type frontendPayload struct {
	EmptyCollections [][]int
	Index            frontendIndex
	ProjectIdentity  string
	Sources          []frontendOccurrence
	// Physical source chunks are addressed by exact bytes and representation
	// version; logical namespace/dependency views never become physical keys.
	Chunks map[string]string
}

// Framed records bound encoding memory to one source or declaration. The
// footer authenticates every length and record, including the stream header.
type frontendStreamHeader struct {
	EmptyCollections                              [][]int
	Index                                         frontendIndex
	ProjectIdentity                               string
	Types, Triggers, Objects, Sources             int
	TypesPresent, TriggersPresent, ObjectsPresent bool
}
type frontendStreamSource struct {
	Occurrence frontendOccurrence
	Raw        *string
}

func writeFrontendRecord(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > 64<<20 {
		return errors.New("frontend record exceeds size limit")
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readFrontendRecord(r io.Reader, value any, budget *frontendDecodeBudget) error {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return err
	}
	if n == 0 || n > 64<<20 {
		return errors.New("invalid frontend record size")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	if err := budget.validate(data); err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

const maxFrontendElements = 16 << 10

// Bound aggregate decoded structure before json.Unmarshal allocates graphs.
// The conservative charges cover struct/slice/map growth as well as strings;
// unknown JSON fields consume the same budget. Counts alone are insufficient
// because a small encoded record can contain many empty composite values.
type frontendDecodeBudget struct{ remaining int64 }

func (b *frontendDecodeBudget) validate(data []byte) error {
	// Count encoded values without materializing their strings a second time.
	// json.Unmarshal still validates the complete JSON grammar before decoding.
	depth := 0
	for i := 0; i < len(data); i++ {
		cost := int64(0)
		switch data[i] {
		case ' ', '\t', '\r', '\n', ',', ':':
			continue
		case '{':
			cost = 2048
			depth++
		case '[':
			cost = 128
			depth++
		case '}', ']':
			depth--
		case '"':
			start := i
			i++
			for i < len(data) && data[i] != '"' {
				if data[i] == '\\' {
					i++
				}
				i++
			}
			if i >= len(data) {
				return errors.New("unterminated frontend string")
			}
			// Encoded escapes overestimate decoded size. Three bytes per raw byte
			// also cover the decoder's replacement of malformed UTF-8.
			cost = 64 + 3*int64(i-start-1)
		default:
			start := i
			for i+1 < len(data) && !frontendValueDelimiter(data[i+1]) {
				i++
			}
			cost = 64 + 3*int64(i-start+1)
		}
		if depth < 0 || depth > 128 {
			return errors.New("frontend record nesting exceeds limit")
		}
		b.remaining -= cost
		if b.remaining < 0 {
			return errors.New("frontend decoded graph exceeds limit")
		}
	}
	if depth != 0 {
		return errors.New("unterminated frontend record")
	}
	return nil
}

func frontendValueDelimiter(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', ',', ':', '}', ']':
		return true
	}
	return false
}

func snapshotObjects(objects []schema.Object) []frontendObject {
	if objects == nil {
		return nil
	}
	out := make([]frontendObject, len(objects))
	for i, o := range objects {
		out[i] = frontendObject{Object: o, GeneratedHistoryParent: o.GeneratedHistoryParent}
		if o.Fields != nil {
			out[i].Fields = make([]frontendField, len(o.Fields))
		}
		for j, f := range o.Fields {
			out[i].Fields[j] = frontendField{Field: f, File: f.File, ChildRelationshipNameInferred: f.ChildRelationshipNameInferred}
		}
	}
	return out
}

// EncodeFrontendSnapshot captures an already validated index and its complete
// source arena. It never synthesizes source provenance from public symbol JSON.
func EncodeFrontendSnapshot(index Index, artifacts *BuildArtifacts) ([]byte, error) {
	var buffer bytes.Buffer
	if err := WriteFrontendSnapshot(&buffer, index, artifacts); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WriteFrontendSnapshot writes one record at a time without a second encoded
// copy of the complete index or source arena. Callers publish atomically only
// after this function has written and authenticated the complete stream.
func WriteFrontendSnapshot(w io.Writer, index Index, artifacts *BuildArtifacts) error {
	if err := ValidateBuildGeneration(index, artifacts); err != nil {
		return err
	}
	hash := sha256.New()
	if err := writeFrontendRecords(io.MultiWriter(w, hash), index, artifacts); err != nil {
		return err
	}
	_, err := w.Write(hash.Sum(nil))
	return err
}

// WriteFrontendSnapshotRecords writes the same versioned records without the
// integrity footer, for a caller that authenticates its complete enclosing
// file in one pass. It encodes the captured generation only: the published
// identity is derived from those captured bytes, so a later live edit changes
// the next load's identity instead of requiring another full source reread.
func WriteFrontendSnapshotRecords(w io.Writer, index Index, artifacts *BuildArtifacts) error {
	if err := validateCapturedFrontendGeneration(index, artifacts); err != nil {
		return err
	}
	return writeFrontendRecords(w, index, artifacts)
}

// validateCapturedFrontendGeneration applies ValidateBuildGeneration's
// completeness and digest checks to the captured arena, without live reads.
func validateCapturedFrontendGeneration(index Index, artifacts *BuildArtifacts) error {
	if artifacts == nil || artifacts.Sources == nil || artifacts.SourceDigests == nil || artifacts.ApexMetadataInputs == nil {
		return incompleteSourceSnapshotError("build artifacts are incomplete")
	}
	validate := func(typ TypeSymbol) error {
		if !typ.HasSourceSnapshot() {
			return nil
		}
		source, ok := artifacts.SourceForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing type source " + typ.File)
		}
		if err := validateSourceDigest(typ.File, source.Digest(), artifacts.SourceDigests); err != nil {
			return err
		}
		if _, ok := artifacts.ApexMetadataForType(typ); !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + typ.File + "-meta.xml")
		}
		return nil
	}
	for _, typ := range index.Types {
		if err := validate(typ); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		if err := validate(TypeSymbol{File: trigger.File, SourceRoot: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, SourceNamespaceRemaps: trigger.SourceNamespaceRemaps, SourceBacked: true}); err != nil {
			return err
		}
	}
	return nil
}

func writeFrontendRecords(output io.Writer, index Index, artifacts *BuildArtifacts) error {
	for _, typ := range index.Types {
		if typ.Artifact {
			return errors.New("frontend snapshot requires captured artifact byte provenance")
		}
	}
	if len(index.CodeIntelSymbols) != 0 || len(index.CodeIntelUses) != 0 {
		return errors.New("frontend snapshot requires captured artifact byte provenance")
	}
	if index.projectIdentity == "" {
		return errors.New("frontend snapshot requires project provenance")
	}
	headerIndex := index
	headerIndex.Types, headerIndex.Triggers, headerIndex.Objects = nil, nil, nil
	sources := artifacts.Sources.All()
	if len(index.Types)+len(index.Triggers)+len(index.Objects)+len(sources) > maxFrontendElements {
		return errors.New("frontend element count exceeds limit")
	}
	header := frontendStreamHeader{Index: frontendIndex{Index: headerIndex, CustomMetadataRecords: index.CustomMetadataRecords}, ProjectIdentity: index.projectIdentity, EmptyCollections: frontendEmptyCollections(reflect.ValueOf(index), nil), Types: len(index.Types), Triggers: len(index.Triggers), Objects: len(index.Objects), Sources: len(sources), TypesPresent: index.Types != nil, TriggersPresent: index.Triggers != nil, ObjectsPresent: index.Objects != nil}
	if err := binary.Write(output, binary.LittleEndian, uint32(FrontendSnapshotVersion)); err != nil {
		return err
	}
	if err := writeFrontendRecord(output, header); err != nil {
		return err
	}
	for _, t := range index.Types {
		if err := writeFrontendRecord(output, frontendType{TypeSymbol: t, ConstructorsAuthoritative: t.ConstructorsAuthoritative, EnumHashBase: t.EnumHashBase}); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if err := writeFrontendRecord(output, trigger); err != nil {
			return err
		}
	}
	for _, o := range index.Objects {
		if err := writeFrontendRecord(output, snapshotObjects([]schema.Object{o})[0]); err != nil {
			return err
		}
	}
	seen := make(map[[sha256.Size]byte]bool)
	for _, source := range sources {
		input, ok := artifacts.Sources.apexMetadataForMetadata(source.metadata)
		if !ok {
			return errors.New("frontend snapshot lacks companion metadata provenance")
		}
		chunk := hex.EncodeToString(source.digest[:])
		record := frontendStreamSource{Occurrence: frontendOccurrence{Metadata: source.Metadata(), Chunk: chunk, ApexMetadata: input}}
		if !seen[source.digest] {
			raw := source.raw
			record.Raw = &raw
			seen[source.digest] = true
		}
		if err := writeFrontendRecord(output, record); err != nil {
			return err
		}
	}
	return nil
}

func readFrontendPayload(data []byte) (frontendPayload, error) {
	if len(data) < 36 {
		return frontendPayload{}, errors.New("truncated frontend snapshot")
	}
	payload, checksum := data[:len(data)-32], data[len(data)-32:]
	digest := sha256.Sum256(payload)
	if !bytes.Equal(checksum, digest[:]) {
		return frontendPayload{}, errors.New("frontend snapshot checksum mismatch")
	}
	return readFrontendRecords(payload)
}

// readFrontendRecords decodes an authenticated record stream. Counts, framing
// and decoded graph budgets remain untrusted and are validated here.
func readFrontendRecords(payload []byte) (frontendPayload, error) {
	var wire frontendPayload
	if len(payload) < 4 {
		return wire, errors.New("truncated frontend snapshot")
	}
	reader := bytes.NewReader(payload)
	var version uint32
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil || version != FrontendSnapshotVersion {
		return wire, errors.New("invalid frontend snapshot version")
	}
	budget := &frontendDecodeBudget{remaining: 256 << 20}
	var header frontendStreamHeader
	if err := readFrontendRecord(reader, &header, budget); err != nil {
		return wire, err
	}
	if header.Types < 0 || header.Triggers < 0 || header.Objects < 0 || header.Sources < 0 || header.Types > len(payload)/4 || header.Triggers > len(payload)/4 || header.Objects > len(payload)/4 || header.Sources > len(payload)/4 {
		return wire, errors.New("invalid frontend snapshot counts")
	}
	if header.Types+header.Triggers+header.Objects+header.Sources > maxFrontendElements {
		return wire, errors.New("frontend element count exceeds limit")
	}
	if header.Index.Types != nil || header.Index.Triggers != nil || header.Index.Objects != nil {
		return wire, errors.New("frontend header contains unframed declarations")
	}
	wire.Index, wire.ProjectIdentity, wire.EmptyCollections = header.Index, header.ProjectIdentity, header.EmptyCollections
	// Counts are untrusted even after a checksum matches. Validate each
	// framed record before allocating its element; a padded header must not
	// reserve a complete type/object graph before the first record is read.
	if (!header.TypesPresent && header.Types != 0) || (!header.TriggersPresent && header.Triggers != 0) || (!header.ObjectsPresent && header.Objects != 0) {
		return wire, errors.New("invalid frontend collection presence")
	}
	if header.TypesPresent {
		wire.Index.Types = []frontendType{}
	}
	if header.TriggersPresent {
		wire.Index.Triggers = []TriggerSymbol{}
	}
	if header.ObjectsPresent {
		wire.Index.Objects = []frontendObject{}
	}
	for i := 0; i < header.Types; i++ {
		var value frontendType
		if err := readFrontendRecord(reader, &value, budget); err != nil {
			return wire, err
		}
		wire.Index.Types = append(wire.Index.Types, value)
	}
	for i := 0; i < header.Triggers; i++ {
		var value TriggerSymbol
		if err := readFrontendRecord(reader, &value, budget); err != nil {
			return wire, err
		}
		wire.Index.Triggers = append(wire.Index.Triggers, value)
	}
	for i := 0; i < header.Objects; i++ {
		var value frontendObject
		if err := readFrontendRecord(reader, &value, budget); err != nil {
			return wire, err
		}
		wire.Index.Objects = append(wire.Index.Objects, value)
	}
	wire.Chunks = make(map[string]string)
	for i := 0; i < header.Sources; i++ {
		var source frontendStreamSource
		if err := readFrontendRecord(reader, &source, budget); err != nil {
			return wire, err
		}
		if source.Raw != nil {
			if _, exists := wire.Chunks[source.Occurrence.Chunk]; exists {
				return wire, errors.New("duplicate frontend chunk")
			}
			wire.Chunks[source.Occurrence.Chunk] = *source.Raw
		}
		wire.Sources = append(wire.Sources, source.Occurrence)
	}
	if reader.Len() != 0 {
		return wire, errors.New("trailing frontend snapshot data")
	}
	return wire, nil
}

// RestoreFrontendSnapshot reconstructs private provenance and an independently
// owned source arena without invoking the parser or index builder. The caller
// must supply freshly discovered project membership and a fresh schema.
func RestoreFrontendSnapshot(p project.Project, s schema.Schema, data []byte) (Index, BuildArtifacts, error) {
	return restoreFrontendSnapshot(p, s, data, true, true, nil)
}

// footer selects a self-authenticated snapshot; otherwise data is a record
// stream whose enclosing file was authenticated by the caller.
func restoreFrontendSnapshot(p project.Project, s schema.Schema, data []byte, footer bool, validateContent bool, inputs *FrontendInputs) (Index, BuildArtifacts, error) {
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project == nil && dep.ArtifactPath != "" {
			return Index{}, BuildArtifacts{}, errors.New("frontend snapshot requires captured artifact byte provenance")
		}
	}
	read := readFrontendRecords
	if footer {
		read = readFrontendPayload
	}
	wire, err := read(data)
	if err != nil {
		return Index{}, BuildArtifacts{}, err
	}

	idx := wire.Index.Index
	idx.CustomMetadataRecords = wire.Index.CustomMetadataRecords
	if wire.Index.Types != nil {
		idx.Types = make([]TypeSymbol, len(wire.Index.Types))
	}
	for i, t := range wire.Index.Types {
		idx.Types[i] = t.TypeSymbol
		idx.Types[i].ConstructorsAuthoritative = t.ConstructorsAuthoritative
		idx.Types[i].EnumHashBase = t.EnumHashBase
	}
	if wire.Index.Objects != nil {
		idx.Objects = make([]schema.Object, len(wire.Index.Objects))
	}
	for i, o := range wire.Index.Objects {
		idx.Objects[i] = o.Object
		idx.Objects[i].GeneratedHistoryParent = o.GeneratedHistoryParent
		if o.Fields != nil {
			idx.Objects[i].Fields = make([]schema.Field, len(o.Fields))
		}
		for j, f := range o.Fields {
			idx.Objects[i].Fields[j] = f.Field
			idx.Objects[i].Fields[j].File = f.File
			idx.Objects[i].Fields[j].ChildRelationshipNameInferred = f.ChildRelationshipNameInferred
		}
	}
	for _, typ := range idx.Types {
		if typ.Artifact {
			return Index{}, BuildArtifacts{}, errors.New("frontend snapshot requires captured artifact byte provenance")
		}
	}
	if len(idx.CodeIntelSymbols) != 0 || len(idx.CodeIntelUses) != 0 {
		return Index{}, BuildArtifacts{}, errors.New("frontend snapshot requires captured artifact byte provenance")
	}
	if err := restoreFrontendEmptyCollections(reflect.ValueOf(&idx).Elem(), wire.EmptyCollections); err != nil {
		return Index{}, BuildArtifacts{}, err
	}
	idx.projectIdentity = wire.ProjectIdentity
	if (inputs == nil && !MatchesProjectIdentity(idx, p)) || (inputs != nil && idx.projectIdentity != inputs.projects[p.Root]) {
		return Index{}, BuildArtifacts{}, errors.New("frontend project identity changed")
	}
	// Recreate only schema composition, never source symbols. Include removals,
	// dependency projections and metadata records in the exact comparison.
	expectedSchema := Index{Objects: slices.Clone(s.Objects), CustomMetadataRecords: slices.Clone(s.CustomMetadataRecords)}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" {
			continue
		}
		if dep.Project == nil {
			if dep.ArtifactPath != "" {
				appendArtifactDependency(&expectedSchema, dep)
			}
			continue
		}
		depSchema, err := schema.LoadProject(*dep.Project)
		if err != nil {
			continue
		} // The builder reports, and omits, this dependency.
		depSchema = namespaceDependencySchema(depSchema, dep.Namespace)
		expectedSchema.Objects = append(expectedSchema.Objects, depSchema.Objects...)
		expectedSchema.CustomMetadataRecords = append(expectedSchema.CustomMetadataRecords, depSchema.CustomMetadataRecords...)
	}
	schemaBytes := func(index Index) []byte {
		data, _ := json.Marshal(struct {
			Objects []frontendObject
			Records []schema.CustomMetadataRecord
		}{snapshotObjects(index.Objects), index.CustomMetadataRecords})
		return data
	}
	if !bytes.Equal(schemaBytes(expectedSchema), schemaBytes(idx)) {
		return Index{}, BuildArtifacts{}, errors.New("frontend schema changed")
	}

	sources := NewWorkspaceSources()
	// Hash each physical chunk once; occurrences sharing exact bytes share it.
	chunkDigests := make(map[string][sha256.Size]byte, len(wire.Chunks))
	for _, occurrence := range wire.Sources {
		raw, ok := wire.Chunks[occurrence.Chunk]
		if !ok {
			return Index{}, BuildArtifacts{}, errors.New("frontend source chunk checksum mismatch")
		}
		digest, hashed := chunkDigests[occurrence.Chunk]
		if !hashed {
			digest = sha256.Sum256([]byte(raw))
			if hex.EncodeToString(digest[:]) != occurrence.Chunk {
				return Index{}, BuildArtifacts{}, errors.New("frontend source chunk checksum mismatch")
			}
			chunkDigests[occurrence.Chunk] = digest
		}
		metadata := occurrence.Metadata
		physical := ""
		if inputs != nil {
			physical = inputs.physical[metadata.RequestedPath]
		} else {
			physical = sources.canonicalPhysicalPath(metadata.RequestedPath)
		}
		if metadata.PhysicalPath != physical {
			return Index{}, BuildArtifacts{}, errors.New("frontend physical source changed")
		}
		source := WorkspaceSource{raw: raw, digest: digest, metadata: metadata}
		logical := sources.loadLogical(logicalSourceKey{physicalPath: metadata.PhysicalPath, namespace: metadata.Namespace, remapFingerprint: sourceRemapFingerprint(metadata.NamespaceRemaps)}, raw, metadata.Namespace, metadata.NamespaceRemaps)
		source.normalized = logical.normalized
		sources.adopt(source, occurrence.ApexMetadata)
	}
	artifacts := BuildArtifacts{Sources: sources, SourceDigests: sources.sourceDigestSet(), ApexMetadataInputs: capturedApexMetadataInputs(idx, sources)}
	idx.sourceDigests = artifacts.SourceDigests
	idx.apexMetadataInputs = sources.apexMetadataInputSet()
	// Exact logical membership includes rejected parser inputs, which need not
	// have produced a TypeSymbol. Do not validate only successful declarations.
	owners, ok := frontendSourceOwners(idx, p, inputs)
	if !ok {
		return Index{}, BuildArtifacts{}, errors.New("frontend source ownership is incomplete")
	}
	expected := make(map[sourceOccurrenceKey]int)
	for _, owner := range owners {
		var remaps []namespaceremap.Rule
		if owner.metadata.dependency {
			remaps = owner.project.NamespaceRemaps
		}
		for _, path := range owner.project.SelectedApexFiles() {
			key := sourceOccurrenceKeyForMetadata(SourceMetadata{RequestedPath: path, Root: owner.project.Root, Namespace: owner.metadata.namespace, Version: owner.metadata.version, Dependency: owner.metadata.dependency, NamespaceRemaps: remaps})
			expected[key]++
		}
	}
	for _, occurrence := range wire.Sources {
		key := sourceOccurrenceKeyForMetadata(occurrence.Metadata)
		if expected[key] == 0 {
			return Index{}, BuildArtifacts{}, errors.New("frontend source membership changed")
		}
		expected[key]--
	}
	for _, count := range expected {
		if count != 0 {
			return Index{}, BuildArtifacts{}, errors.New("frontend source membership changed")
		}
	}

	if inputs != nil {
		for _, source := range sources.All() {
			path := source.metadata.RequestedPath
			input, ok := sources.apexMetadataForMetadata(source.metadata)
			if !ok || inputs.digests[path] != source.digest || inputs.metadata[path] != input {
				return Index{}, BuildArtifacts{}, errors.New("frontend captured input changed")
			}
		}
	} else if validateContent {
		if err := ValidateBuildGeneration(idx, &artifacts); err != nil {
			return Index{}, BuildArtifacts{}, err
		}
		for _, source := range sources.All() {
			if _, represented := artifacts.ApexMetadataInputs[source.metadata.RequestedPath]; represented {
				continue
			}
			if err := validateLiveSourceAndMetadata(source.metadata.RequestedPath, source.digest, sources.apexMetadata[sourceOccurrenceKeyForMetadata(source.metadata)]); err != nil {
				return Index{}, BuildArtifacts{}, err
			}
		}
	}
	return idx, artifacts, nil
}

// FrontendInputIdentity validates relevant current bytes, inventory, resolved
// configuration and dependency views before a persisted frontend can be used.
// Filesystem timestamps are never an acceptance proof.
// FrontendInputs is a short-lived content proof captured from the freshly
// discovered project. Its maps are private and never persisted or accepted
// from timestamps. A restoration consumes the same reads and canonical paths.
type FrontendInputs struct {
	identity string
	projects map[string]string
	physical map[string]string
	digests  map[string][sha256.Size]byte
	metadata map[string]ApexMetadataInput
}

func (inputs *FrontendInputs) Identity() string { return inputs.identity }

func CaptureFrontendInputs(p project.Project, s schema.Schema) (*FrontendInputs, error) {
	return captureFrontendInputs(p, s, nil, nil)
}

// FrontendInputsForBuild derives publication identity from already captured
// bytes. A live edit after capture changes the next load's identity, so the
// published generation is never accepted for different current content.
func FrontendInputsForBuild(p project.Project, s schema.Schema, index Index, artifacts *BuildArtifacts) (*FrontendInputs, error) {
	inputs, err := captureFrontendInputs(p, s, artifacts, nil)
	if err != nil {
		return nil, err
	}
	if inputs.projects[p.Root] != index.projectIdentity {
		return nil, errors.New("frontend project changed during construction")
	}
	return inputs, nil
}

func FrontendInputIdentity(p project.Project, s schema.Schema) (string, error) {
	inputs, err := CaptureFrontendInputs(p, s)
	if err != nil {
		return "", err
	}
	return inputs.identity, nil
}
func frontendInputIdentity(p project.Project, s schema.Schema, previous *SourceDigestSet) (string, error) {
	inputs, err := captureFrontendInputs(p, s, nil, previous)
	if err != nil {
		return "", err
	}
	return inputs.identity, nil
}

func captureFrontendInputs(p project.Project, s schema.Schema, artifacts *BuildArtifacts, previous *SourceDigestSet) (*FrontendInputs, error) {
	inputs := &FrontendInputs{projects: map[string]string{}, physical: map[string]string{}, digests: map[string][sha256.Size]byte{}, metadata: map[string]ApexMetadataInput{}}
	if artifacts != nil {
		if artifacts.Sources == nil {
			return nil, errors.New("missing frontend sources")
		}
		for _, source := range artifacts.Sources.All() {
			path := source.metadata.RequestedPath
			input, ok := artifacts.Sources.apexMetadataForMetadata(source.metadata)
			if !ok {
				return nil, errors.New("missing frontend sidecar")
			}
			inputs.digests[path], inputs.physical[path], inputs.metadata[path] = source.digest, source.metadata.PhysicalPath, input
		}
	}
	h := sha256.New()
	encoder := json.NewEncoder(h)
	encode := encoder.Encode
	if err := encode(p); err != nil {
		return nil, err
	}
	if err := encode(struct {
		Objects []frontendObject
		Records []schema.CustomMetadataRecord
	}{snapshotObjects(s.Objects), s.CustomMetadataRecords}); err != nil {
		return nil, err
	}
	arena := NewWorkspaceSources()
	var visit func(project.Project) error
	visit = func(current project.Project) error {
		identity := incrementalProjectIdentity(current)
		inputs.projects[current.Root] = identity
		if err := encode(identity); err != nil {
			return err
		}
		for _, path := range current.SelectedApexFiles() {
			digest, ok := inputs.digests[path]
			if !ok {
				if artifacts != nil {
					return errors.New("frontend source inventory changed")
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				digest = sha256.Sum256(data)
				inputs.digests[path], inputs.physical[path] = digest, arena.canonicalPhysicalPath(path)
				sidecar, err := os.ReadFile(path + "-meta.xml")
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				if err == nil {
					inputs.metadata[path] = ApexMetadataInput{Present: true, Digest: sha256.Sum256(sidecar)}
				} else {
					inputs.metadata[path] = ApexMetadataInput{}
				}
			}
			if prior, ok := previous.Digest(path); ok {
				digest = prior
			}
			if err := encode(struct {
				Path, Physical string
				SHA256         [32]byte
			}{path, inputs.physical[path], digest}); err != nil {
				return err
			}
			if err := encode(inputs.metadata[path]); err != nil {
				return err
			}
		}
		for _, path := range current.FlowFiles {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := encode(struct {
				Path   string
				SHA256 [32]byte
			}{path, sha256.Sum256(data)}); err != nil {
				return err
			}
		}
		for _, dep := range current.ManagedPackageDependencies {
			if dep.Project != nil {
				sch, err := schema.LoadProject(*dep.Project)
				if err != nil {
					return err
				}
				if err := encode(snapshotObjects(sch.Objects)); err != nil {
					return err
				}
				if err := encode(sch.CustomMetadataRecords); err != nil {
					return err
				}
				if err := visit(*dep.Project); err != nil {
					return err
				}
			}
			if dep.ArtifactPath != "" {
				data, err := os.ReadFile(dep.ArtifactPath)
				if err != nil {
					return err
				}
				if err := encode(sha256.Sum256(data)); err != nil {
					return err
				}
			}
		}
		for _, shim := range current.PackageShims {
			if shim.Project != nil {
				if err := visit(*shim.Project); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(p); err != nil {
		return nil, err
	}
	inputs.identity = fmt.Sprintf("%x", h.Sum(nil))
	return inputs, nil
}

// RestoreFrontendSnapshotWithInputs reuses the live content proof from this
// load. It never reopens each file to check an already checked byte digest.
// Semantic/runtime consumers retain their later live-generation barriers.
func RestoreFrontendSnapshotWithInputs(p project.Project, s schema.Schema, data []byte, inputs *FrontendInputs) (Index, BuildArtifacts, error) {
	if inputs == nil {
		return Index{}, BuildArtifacts{}, errors.New("missing frontend input proof")
	}
	return restoreFrontendSnapshot(p, s, data, true, false, inputs)
}

// RestoreFrontendSnapshotRecordsWithInputs restores a footerless record stream
// from WriteFrontendSnapshotRecords. The caller has authenticated the stream.
func RestoreFrontendSnapshotRecordsWithInputs(p project.Project, s schema.Schema, records []byte, inputs *FrontendInputs) (Index, BuildArtifacts, error) {
	if inputs == nil {
		return Index{}, BuildArtifacts{}, errors.New("missing frontend input proof")
	}
	return restoreFrontendSnapshot(p, s, records, false, false, inputs)
}

func frontendSourceOwners(index Index, p project.Project, inputs *FrontendInputs) ([]incrementalSourceOwner, bool) {
	if inputs == nil {
		return incrementalSourceOwners(index, p)
	}
	if index.projectIdentity == "" || index.projectIdentity != inputs.projects[p.Root] || p.Root != index.Project.Root || p.Namespace != index.Project.Namespace || p.SourceAPIVersion != index.Project.SourceAPIVersion {
		return nil, false
	}
	owners := []incrementalSourceOwner{{project: p, metadata: incrementalSourceMetadata{namespace: p.Namespace, root: p.Root, apiVersion: p.SourceAPIVersion}}}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			owners = append(owners, incrementalSourceOwner{project: *dep.Project, metadata: incrementalSourceMetadata{namespace: dep.Namespace, root: dep.Project.Root, version: dep.Version, apiVersion: dep.Project.SourceAPIVersion, dependency: true, namespaceRemaps: dep.Project.NamespaceRemaps}})
		}
	}
	for _, shim := range p.PackageShims {
		if shim.Status == "loaded" && shim.Project != nil {
			owners = append(owners, incrementalSourceOwner{project: *shim.Project, metadata: incrementalSourceMetadata{namespace: shim.Namespace, root: shim.Project.Root, apiVersion: shim.Project.SourceAPIVersion, dependency: true}})
		}
	}
	return owners, true
}

// Public JSON intentionally omits empty collections. Snapshot presence records
// preserve those distinctions without changing the public representation. Field
// ordinals are safe only within this versioned, build-fingerprinted format.
func frontendEmptyCollections(value reflect.Value, path []int) [][]int {
	return collectFrontendEmptyCollections(value, path, nil)
}

func collectFrontendEmptyCollections(value reflect.Value, path []int, result [][]int) [][]int {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return result
		}
		return collectFrontendEmptyCollections(value.Elem(), path, result)
	}
	switch value.Kind() {
	case reflect.Struct:
		if path == nil {
			path = make([]int, 0, 16)
		}
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.CanInterface() && frontendCollectionContainer(field.Kind()) {
				result = collectFrontendEmptyCollections(field, append(path, i), result)
			}
		}
	case reflect.Slice:
		if !value.IsNil() && value.Len() == 0 {
			return append(result, slices.Clone(path))
		}
		if value.Len() == 0 || !frontendCollectionContainer(value.Type().Elem().Kind()) {
			return result
		}
		if path == nil {
			path = make([]int, 0, 16)
		}
		for i := 0; i < value.Len(); i++ {
			result = collectFrontendEmptyCollections(value.Index(i), append(path, i), result)
		}
	case reflect.Map:
		if !value.IsNil() && value.Len() == 0 {
			return append(result, slices.Clone(path))
		}
	}
	return result
}

// Only retained positions own a path copy. Traversal uses a reusable stack and
// skips scalar leaves; CanInterface excludes the same unexported fields without
// allocating reflect.StructField.Index metadata for every visited field.
func frontendCollectionContainer(kind reflect.Kind) bool {
	return kind == reflect.Struct || kind == reflect.Slice || kind == reflect.Map || kind == reflect.Pointer
}

func restoreFrontendEmptyCollections(root reflect.Value, paths [][]int) error {
	for _, path := range paths {
		value := root
		for _, step := range path {
			for value.Kind() == reflect.Pointer && !value.IsNil() {
				value = value.Elem()
			}
			switch value.Kind() {
			case reflect.Struct:
				if step < 0 || step >= value.NumField() || value.Type().Field(step).PkgPath != "" {
					return errors.New("invalid frontend collection field")
				}
				value = value.Field(step)
			case reflect.Slice:
				if step < 0 || step >= value.Len() {
					return errors.New("invalid frontend collection index")
				}
				value = value.Index(step)
			default:
				return errors.New("invalid frontend collection path")
			}
		}
		if !value.CanSet() || (value.Kind() != reflect.Slice && value.Kind() != reflect.Map) || value.Len() != 0 {
			return errors.New("invalid frontend collection presence")
		}
		if value.Kind() == reflect.Slice {
			value.Set(reflect.MakeSlice(value.Type(), 0, 0))
		} else {
			value.Set(reflect.MakeMap(value.Type()))
		}
	}
	return nil
}

// UpdateFrontendSnapshot reuses unchanged parsed declarations only when the
// complete current non-source input identity still matches the old generation.
// Semantic results remain invalidated for every changed source digest.
func UpdateFrontendSnapshot(p project.Project, s schema.Schema, data []byte, expectedInput string) (Index, BuildArtifacts, error) {
	return updateFrontendSnapshot(p, s, data, true, expectedInput)
}

// UpdateFrontendSnapshotRecords is UpdateFrontendSnapshot for a footerless
// record stream that its enclosing cache file already authenticated.
func UpdateFrontendSnapshotRecords(p project.Project, s schema.Schema, records []byte, expectedInput string) (Index, BuildArtifacts, error) {
	return updateFrontendSnapshot(p, s, records, false, expectedInput)
}

func updateFrontendSnapshot(p project.Project, s schema.Schema, data []byte, footer bool, expectedInput string) (Index, BuildArtifacts, error) {
	if len(p.PackageDirectories) == 0 {
		return Index{}, BuildArtifacts{}, errors.New("frontend incremental reuse requires a discovered project")
	}
	previous, artifacts, err := restoreFrontendSnapshot(p, s, data, footer, false, nil)
	if err != nil {
		return Index{}, BuildArtifacts{}, err
	}
	represented := make(map[sourceOccurrenceKey]bool)
	for _, typ := range previous.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		represented[sourceOccurrenceKeyForMetadata(SourceMetadata{RequestedPath: typ.File, Root: typ.SourceRoot, Namespace: typ.Namespace, Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps})] = true
	}
	for _, trigger := range previous.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		represented[sourceOccurrenceKeyForMetadata(SourceMetadata{RequestedPath: trigger.File, Root: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, NamespaceRemaps: trigger.SourceNamespaceRemaps})] = true
	}
	for _, source := range artifacts.Sources.All() {
		if !represented[sourceOccurrenceKeyForMetadata(source.metadata)] {
			return Index{}, BuildArtifacts{}, errors.New("frontend incremental refresh requires every source occurrence")
		}
	}
	proof, err := frontendInputIdentity(p, s, artifacts.SourceDigests)
	if err != nil || proof != expectedInput {
		return Index{}, BuildArtifacts{}, errors.New("frontend non-source inputs changed")
	}
	var changed []string
	seen := map[string]bool{}
	for _, source := range artifacts.Sources.All() {
		path := source.metadata.RequestedPath
		if seen[path] {
			continue
		}
		seen[path] = true
		raw, err := os.ReadFile(path)
		if err != nil {
			return Index{}, BuildArtifacts{}, err
		}
		if sha256.Sum256(raw) != source.digest {
			changed = append(changed, path)
		}
	}
	if len(changed) == 0 || RequiresAuthoritativeApexRebuild(previous, changed, nil) {
		return Index{}, BuildArtifacts{}, errors.New("frontend edit requires authoritative rebuild")
	}
	updated, exact, err := TryUpdateApexFilesCheckedWithLoadedProject(previous, changed, nil, p)
	if err != nil {
		return Index{}, BuildArtifacts{}, err
	}
	if !exact {
		return Index{}, BuildArtifacts{}, errors.New("frontend edit requires authoritative rebuild")
	}
	refreshed, err := RefreshBuildArtifacts(updated, &artifacts)
	if err != nil {
		return Index{}, BuildArtifacts{}, err
	}
	if err := ValidateBuildGeneration(updated, &refreshed); err != nil {
		return Index{}, BuildArtifacts{}, err
	}
	return updated, refreshed, nil
}
