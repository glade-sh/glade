package typesys

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestFrontendSnapshotRoundTripOwnsProvenance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Sample.cls")
			writeFile(t, path, "public class Sample { public class Inner {} public String name; }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			p := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}
			s := schema.Schema{Objects: []schema.Object{{Name: "Owned__c"}}}
			index, artifacts := BuildWithArtifacts(p, s)
			data, err := EncodeFrontendSnapshot(index, &artifacts)
			if err != nil {
				t.Fatal(err)
			}
			restored, sources, err := RestoreFrontendSnapshot(p, s, data)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(index)
			b, _ := json.Marshal(restored)
			if string(a) != string(b) || !SameProjectIdentity(index, restored) {
				t.Fatal("snapshot changed index or private project identity")
			}
			if err := ValidateBuildGeneration(restored, &sources); err != nil {
				t.Fatal(err)
			}
			if sources.Sources == artifacts.Sources || sources.SourceDigests == artifacts.SourceDigests {
				t.Fatal("snapshot aliases its input")
			}
			if sources.Sources.Stats().PhysicalReadAttempts != 0 {
				t.Fatal("restoration rebuilt source arena from disk")
			}
			for i, source := range artifacts.Sources.All() {
				if got := sources.Sources.All()[i]; !reflect.DeepEqual(source, got) {
					t.Fatal("source occurrence changed")
				}
			}
			restored.Types[0].Name = "Changed"
			again, _, err := RestoreFrontendSnapshot(p, s, data)
			if err != nil || again.Types[0].Name == "Changed" {
				t.Fatal("restorations share mutable symbols")
			}
		})
	}
}

func TestFrontendSnapshotRejectsChangedInputs(t *testing.T) {
	for _, mutation := range []string{"content", "sidecar", "add_sidecar", "namespace", "api", "membership", "schema", "symlink", "corrupt", "incomplete"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Sample.cls")
			writeFile(t, path, "public class Sample { Integer left; }")
			if mutation != "add_sidecar" {
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>62.0</apiVersion></ApexClass>")
			}
			p := project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}
			s := schema.Schema{}
			index, artifacts := BuildWithArtifacts(p, s)
			data, err := EncodeFrontendSnapshot(index, &artifacts)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "content":
				before, _ := os.Stat(path)
				writeFile(t, path, "public class Sample { Integer rght; }")
				if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "sidecar":
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			case "add_sidecar":
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			case "namespace":
				p.Namespace = "owned"
			case "api":
				p.SourceAPIVersion = "62.0"
			case "membership":
				p.ApexFiles = nil
			case "schema":
				s.Objects = []schema.Object{{Name: "Other__c"}}
			case "symlink":
				other := filepath.Join(root, "other.cls")
				writeFile(t, other, "public class Sample { Integer left; }")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, path); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				data = []byte(strings.Replace(string(data), "Sample", "Broken", 1))
			case "incomplete":
				data = []byte(`{}`)
			}
			if _, _, err := RestoreFrontendSnapshot(p, s, data); err == nil {
				t.Fatal("accepted changed or incomplete frontend generation")
			}
		})
	}
}

func TestFrontendSnapshotRejectsSameCountSelectionAndRemovedSchema(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Other.cls")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Other {}")
	p := project.Project{Root: root, ApexFiles: []string{first}}
	s := schema.Schema{Objects: []schema.Object{{Name: "Owned__c"}}, CustomMetadataRecords: []schema.CustomMetadataRecord{{FullName: "Owned.Row", ObjectName: "Owned__mdt"}}}
	index, artifacts := BuildWithArtifacts(p, s)
	data, err := EncodeFrontendSnapshot(index, &artifacts)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"same_count_selection", "removed_object", "removed_record"} {
		t.Run(mutation, func(t *testing.T) {
			currentP, currentS := p, s
			switch mutation {
			case "same_count_selection":
				currentP.ApexFiles = []string{second}
			case "removed_object":
				currentS.Objects = nil
			case "removed_record":
				currentS.CustomMetadataRecords = nil
			}
			if _, _, err := RestoreFrontendSnapshot(currentP, currentS, data); err == nil {
				t.Fatal("accepted changed selection or removed schema")
			}
		})
	}
}

func TestFrontendSnapshotDependencyViewsAndOptionalMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Shared.cls")
	writeFile(t, path, "global class Shared { String value = '%%%NAMESPACE%%%Item__c'; }")
	dependency := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: "62.0"}
	shimRoot := filepath.Join(root, "shim")
	shimPath := filepath.Join(shimRoot, "Stub.cls")
	writeFile(t, shimPath, "global class Stub {}")
	shim := project.Project{Root: shimRoot, ApexFiles: []string{shimPath}, SourceAPIVersion: "67.0"}
	shim.NamespaceRemaps = []namespaceremap.Rule{{From: "Old", To: "New"}}
	p := project.Project{Root: root, SourceAPIVersion: "67.0", ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "first", SourceRoot: root, Status: "loaded", Project: &dependency}, {Namespace: "second", SourceRoot: root, Status: "loaded", Project: &dependency}}, PackageShims: []project.PackageShim{{Namespace: "shim", SourceRoot: shimRoot, Status: "loaded", Project: &shim}}}
	no := false
	zero := 0
	s := schema.Schema{Objects: []schema.Object{{Name: "Owned__c", GeneratedHistoryParent: "Parent__c", Triggerable: &no, Fields: []schema.Field{{Name: "Lookup__c", File: "owned.field-meta.xml", ChildRelationshipNameInferred: true, RelationshipOrder: &zero, Updateable: &no}}}}}
	index, artifacts := BuildWithArtifacts(p, s)
	base := int64(0)
	index.Types[0].ConstructorsAuthoritative = true
	index.Types[0].EnumHashBase = &base
	data, err := EncodeFrontendSnapshot(index, &artifacts)
	if err != nil {
		t.Fatal(err)
	}
	restored, sources, err := RestoreFrontendSnapshot(p, s, data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Objects, index.Objects) || !reflect.DeepEqual(restored.Types, index.Types) {
		t.Fatal("private symbol/schema metadata or optional zero/false state changed")
	}
	if got := sources.Sources.Stats(); got.PhysicalSources != 2 || got.LogicalViews != 3 || got.Occurrences != 3 || got.PhysicalReadAttempts != 0 {
		t.Fatalf("dependency/shim physical and logical views changed: %+v", got)
	}
	for i, source := range artifacts.Sources.All() {
		if !reflect.DeepEqual(source, sources.Sources.All()[i]) {
			t.Fatal("dependency/shim source view changed")
		}
	}
}

func TestFrontendSnapshotRetainsEmptyMetadataRecordCollection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	writeFile(t, path, "public class Owned {}")
	p := project.Project{Root: root, ApexFiles: []string{path}}
	s := schema.Schema{CustomMetadataRecords: []schema.CustomMetadataRecord{}}
	index, artifacts := BuildWithArtifacts(p, s)
	data, err := EncodeFrontendSnapshot(index, &artifacts)
	if err != nil {
		t.Fatal(err)
	}
	restored, _, err := RestoreFrontendSnapshot(p, s, data)
	if err != nil {
		t.Fatal(err)
	}
	if restored.CustomMetadataRecords == nil {
		t.Fatal("empty metadata record collection lost its presence")
	}
}

func TestFrontendSnapshotRejectsUncapturedArtifactProvenance(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	writeFile(t, path, "public class Owned {}")
	p := project.Project{Root: root, ApexFiles: []string{path}}
	index, artifacts := BuildWithArtifacts(p, schema.Schema{})
	index.Types[0].Artifact = true
	if _, err := EncodeFrontendSnapshot(index, &artifacts); err == nil {
		t.Fatal("artifact types require immutable artifact byte provenance")
	}
}

func TestFrontendSnapshotPreservesEmptyNestedCollections(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	writeFile(t, path, "public class Owned { public String value { get; set; } public static void accept(String input) {} }")
	p := project.Project{Root: root, ApexFiles: []string{path}}
	index, artifacts := BuildWithArtifacts(p, schema.Schema{})
	data, err := EncodeFrontendSnapshot(index, &artifacts)
	if err != nil {
		t.Fatal(err)
	}
	restored, _, err := RestoreFrontendSnapshot(p, schema.Schema{}, data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index.Types, restored.Types) {
		t.Fatal("empty class/member/parameter/accessor collections changed")
	}
}

// A snapshot write must not hand the writer a second project-sized buffer.
// Individual records are bounded by one declaration or one physical source.
func TestFrontendSnapshotStreamingRoundTrip(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i := 0; i < 40; i++ {
		path := filepath.Join(root, fmt.Sprintf("Owned%d.cls", i))
		writeFile(t, path, fmt.Sprintf("public class Owned%d { String value; } //", i)+strings.Repeat("x", 8192))
		paths = append(paths, path)
	}
	p := project.Project{Root: root, ApexFiles: paths, SourceAPIVersion: "67.0"}
	index, artifacts := BuildWithArtifacts(p, schema.Schema{})
	var output boundedSnapshotWriter
	if err := WriteFrontendSnapshot(&output, index, &artifacts); err != nil {
		t.Fatal(err)
	}
	if output.largest > output.Len()/4 {
		t.Fatalf("project-sized write: largest=%d total=%d", output.largest, output.Len())
	}
	restored, captured, err := RestoreFrontendSnapshot(p, schema.Schema{}, output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index, restored) || captured.Sources.Stats().PhysicalReadAttempts != 0 {
		t.Fatal("streaming snapshot changed index or reconstructed sources from disk")
	}
	broken := append([]byte(nil), output.Bytes()...)
	broken[len(broken)/2] ^= 1
	if _, _, err := RestoreFrontendSnapshot(p, schema.Schema{}, broken); err == nil {
		t.Fatal("accepted corrupted stream")
	}
}

type boundedSnapshotWriter struct {
	bytes.Buffer
	largest int
}

func (w *boundedSnapshotWriter) Write(p []byte) (int, error) {
	if len(p) > w.largest {
		w.largest = len(p)
	}
	return w.Buffer.Write(p)
}

func TestFrontendSnapshotCapturedInputsRejectPreservedMetadataEdits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	writeFile(t, path, "public class Owned { Integer left; }")
	p := project.Project{Root: root, ApexFiles: []string{path}}
	index, artifacts := BuildWithArtifacts(p, schema.Schema{})
	data, err := EncodeFrontendSnapshot(index, &artifacts)
	if err != nil {
		t.Fatal(err)
	}
	before, err := CaptureFrontendInputs(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	writeFile(t, path, "public class Owned { Integer rght; }")
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := CaptureFrontendInputs(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	if before.Identity() == after.Identity() {
		t.Fatal("same-size/same-mtime edit escaped content proof")
	}
	if _, _, err := RestoreFrontendSnapshotWithInputs(p, schema.Schema{}, data, after); err == nil {
		t.Fatal("accepted snapshot from another captured generation")
	}
}

func TestFrontendSnapshotMalformedCountsBoundAllocation(t *testing.T) {
	for _, section := range []string{"types", "objects", "both", "sources"} {
		t.Run(section, func(t *testing.T) {
			const count = 1 << 16
			header := frontendStreamHeader{}
			if section == "types" || section == "both" {
				header.Types, header.TypesPresent = count, true
			}
			if section == "objects" || section == "both" {
				header.Objects, header.ObjectsPresent = count, true
			}
			if section == "sources" {
				header.Sources = count
			}
			var stream bytes.Buffer
			if err := binary.Write(&stream, binary.LittleEndian, uint32(FrontendSnapshotVersion)); err != nil {
				t.Fatal(err)
			}
			if err := writeFrontendRecord(&stream, header); err != nil {
				t.Fatal(err)
			}
			// Authenticate a count-sized body whose first record is malformed.
			// Reject framing before reserving memory for unobserved elements.
			stream.Write(make([]byte, count*4))
			digest := sha256.Sum256(stream.Bytes())
			stream.Write(digest[:])
			data := stream.Bytes()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err := readFrontendPayload(data)
			runtime.ReadMemStats(&after)
			if err == nil {
				t.Fatal("accepted malformed count-sized stream")
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 4<<20 {
				t.Fatalf("malformed header allocated %d bytes before validating its first record", allocated)
			}
		})
	}
}

func TestFrontendSnapshotStreamingTriggers(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i := 0; i < 40; i++ {
		path := filepath.Join(root, fmt.Sprintf("Owned%d.trigger", i))
		writeFile(t, path, fmt.Sprintf("trigger Owned%d on Account (before insert) {}", i))
		paths = append(paths, path)
	}
	p := project.Project{Root: root, ApexFiles: paths, SourceAPIVersion: "67.0"}
	index, artifacts := BuildWithArtifacts(p, schema.Schema{})
	if len(index.Triggers) != len(paths) {
		t.Fatal("trigger fixture did not build")
	}
	var output boundedSnapshotWriter
	if err := WriteFrontendSnapshot(&output, index, &artifacts); err != nil {
		t.Fatal(err)
	}
	if output.largest > output.Len()/4 {
		t.Fatalf("trigger set was encoded as one project-sized record: largest=%d total=%d", output.largest, output.Len())
	}
	restored, _, err := RestoreFrontendSnapshot(p, schema.Schema{}, output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index, restored) {
		t.Fatal("trigger stream changed the complete index")
	}
}

func TestFrontendSnapshotRejectsExcessDecodedElements(t *testing.T) {
	for _, section := range []string{"types", "triggers", "objects", "sources"} {
		t.Run(section, func(t *testing.T) {
			const count = 1 << 15
			header := frontendStreamHeader{}
			switch section {
			case "types":
				header.Types, header.TypesPresent = count, true
			case "triggers":
				header.Triggers, header.TriggersPresent = count, true
			case "objects":
				header.Objects, header.ObjectsPresent = count, true
			case "sources":
				header.Sources = count
			}
			var stream bytes.Buffer
			if err := binary.Write(&stream, binary.LittleEndian, uint32(FrontendSnapshotVersion)); err != nil {
				t.Fatal(err)
			}
			if err := writeFrontendRecord(&stream, header); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < count; i++ {
				if err := writeFrontendRecord(&stream, struct{}{}); err != nil {
					t.Fatal(err)
				}
			}
			digest := sha256.Sum256(stream.Bytes())
			stream.Write(digest[:])
			if _, err := readFrontendPayload(stream.Bytes()); err == nil {
				t.Fatal("accepted excessive valid framed elements")
			}
		})
	}
}

func TestFrontendSnapshotRejectsUnframedDeclarations(t *testing.T) {
	for _, section := range []string{"types", "triggers", "objects"} {
		t.Run(section, func(t *testing.T) {
			header := frontendStreamHeader{}
			switch section {
			case "types":
				header.Index.Types = []frontendType{{}}
			case "triggers":
				header.Index.Triggers = []TriggerSymbol{{}}
			case "objects":
				header.Index.Objects = []frontendObject{{}}
			}
			var stream bytes.Buffer
			if err := binary.Write(&stream, binary.LittleEndian, uint32(FrontendSnapshotVersion)); err != nil {
				t.Fatal(err)
			}
			if err := writeFrontendRecord(&stream, header); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(stream.Bytes())
			stream.Write(digest[:])
			if _, err := readFrontendPayload(stream.Bytes()); err == nil {
				t.Fatal("accepted declarations outside counted frames")
			}
		})
	}
}

func TestFrontendSnapshotHeaderGraphBounded(t *testing.T) {
	for _, section := range []string{"types", "triggers", "objects"} {
		t.Run(section, func(t *testing.T) {
			// A checksum authenticates bytes, not the size of their decoded graph.
			header := json.RawMessage(`{"Index":{"` + section + `": [` + strings.Repeat("{},", 150000) + `{}]}}`)
			var stream bytes.Buffer
			if err := binary.Write(&stream, binary.LittleEndian, uint32(FrontendSnapshotVersion)); err != nil {
				t.Fatal(err)
			}
			if err := writeFrontendRecord(&stream, header); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(stream.Bytes())
			stream.Write(digest[:])
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err := readFrontendPayload(stream.Bytes())
			runtime.ReadMemStats(&after)
			if err == nil {
				t.Fatal("accepted unbounded header graph")
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
				t.Fatalf("header graph allocated %d bytes before rejection", allocated)
			}
		})
	}
}

func TestFrontendSnapshotBudgetDoesNotCopyStrings(t *testing.T) {
	for _, value := range []string{strings.Repeat("{}[]", 20000), strings.Repeat("\\\"{}[]", 20000), strings.Repeat("\x00\"\uFFFD[]", 20000)} {
		data, err := json.Marshal(map[string]string{"raw": value})
		if err != nil {
			t.Fatal(err)
		}
		allocations := testing.AllocsPerRun(3, func() {
			budget := frontendDecodeBudget{remaining: 8 << 20}
			if err := budget.validate(data); err != nil {
				t.Fatal(err)
			}
		})
		if allocations != 0 {
			t.Fatalf("budget validation copied a source string: %g allocations", allocations)
		}
	}
	// Go's decoder replaces malformed UTF-8 with three-byte replacement runes.
	// Charge raw string bytes conservatively before decoding them.
	data := append([]byte(`{"raw":"`), bytes.Repeat([]byte{0xff}, 20000)...)
	data = append(data, []byte(`"}`)...)
	budget := frontendDecodeBudget{remaining: 8 << 20}
	if err := budget.validate(data); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if (8<<20)-budget.remaining < int64(len(decoded["raw"])) {
		t.Fatal("budget undercharged UTF-8 replacement")
	}
}

func TestFrontendSnapshotPresenceTraversalDoesNotAllocatePerScalar(t *testing.T) {
	type scalar struct {
		A, B, C, D, E, F string
		Count            int
		Enabled          bool
	}
	values := make([]scalar, 10000)
	pointers := make([]*scalar, len(values))
	type nested struct {
		Value  scalar
		Labels []string
	}
	nestedValues := make([]nested, len(values))
	for i := range values {
		pointers[i] = &values[i]
		nestedValues[i].Labels = []string{"first", "second", "third"}
	}
	for _, value := range []any{values, pointers, nestedValues} {
		allocations := testing.AllocsPerRun(3, func() {
			if paths := frontendEmptyCollections(reflect.ValueOf(value), nil); len(paths) != 0 {
				t.Fatal("scalar traversal reported an empty collection")
			}
		})
		if allocations > 8 {
			t.Fatalf("presence traversal allocated per scalar: %g allocations", allocations)
		}
	}
}

func TestFrontendSnapshotPresencePathsOwnTheirPositions(t *testing.T) {
	type child struct {
		Map   map[string]string
		Slice []string
	}
	value := struct{ Left, Right []child }{
		Left:  []child{{Map: map[string]string{}, Slice: []string{}}},
		Right: []child{{Map: map[string]string{}, Slice: []string{}}},
	}
	paths := frontendEmptyCollections(reflect.ValueOf(value), nil)
	want := [][]int{{0, 0, 0}, {0, 0, 1}, {1, 0, 0}, {1, 0, 1}}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("presence positions changed: %v", paths)
	}
	paths[0][2] = 9
	if paths[1][2] != 1 || paths[2][2] != 0 || paths[3][2] != 1 {
		t.Fatal("presence positions share mutable backing")
	}
}

func TestFrontendSnapshotPresenceTraversalAllocatesOnlyRetainedPaths(t *testing.T) {
	type leaf struct{ Empty []string }
	type branch struct{ Leaves []leaf }
	values := make([]branch, 10000)
	for i := range values {
		values[i].Leaves = []leaf{{Empty: []string{}}}
	}
	value := reflect.ValueOf(values)
	allocations := testing.AllocsPerRun(3, func() {
		paths := frontendEmptyCollections(value, nil)
		if len(paths) != len(values) {
			t.Fatalf("lost empty collection positions: %d", len(paths))
		}
		for i, path := range paths {
			if len(path) != 4 || path[0] != i || path[1] != 0 || path[2] != 0 || path[3] != 0 {
				t.Fatalf("changed empty collection position: %v", path)
			}
		}
	})
	// One owned position per retained empty collection, plus amortized growth
	// of the traversal stack/result. Intermediate containers need no lists.
	if allocations > float64(len(values)+32) {
		t.Fatalf("presence traversal allocated intermediate lists: %g allocations", allocations)
	}
}

// An enclosing cache authenticates the complete file once. Its record stream
// is the footer stream without the footer, and is encoded from the captured
// generation alone: publication never rereads every source and sidecar.
func TestFrontendSnapshotRecordsUseCapturedGenerationWithoutFooter(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Owned.cls")
			original := "public class Owned { Integer left; }"
			writeFile(t, path, original)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			p := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: api}
			index, artifacts := BuildWithArtifacts(p, schema.Schema{})
			var full bytes.Buffer
			if err := WriteFrontendSnapshot(&full, index, &artifacts); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, "public class Owned { Integer right; }")
			var records bytes.Buffer
			if err := WriteFrontendSnapshotRecords(&records, index, &artifacts); err != nil {
				t.Fatalf("captured generation was revalidated against live files: %v", err)
			}
			digest := sha256.Sum256(records.Bytes())
			if !bytes.Equal(full.Bytes(), append(records.Bytes(), digest[:]...)) {
				t.Fatal("record stream is not the footer stream without its single footer")
			}
			if _, _, err := RestoreFrontendSnapshotRecordsWithInputs(p, schema.Schema{}, full.Bytes(), mustCaptureFrontendInputs(t, p)); err == nil {
				t.Fatal("record restore accepted trailing footer bytes")
			}
			// The edited live file is not the captured generation.
			if _, _, err := RestoreFrontendSnapshotRecordsWithInputs(p, schema.Schema{}, records.Bytes(), mustCaptureFrontendInputs(t, p)); err == nil {
				t.Fatal("record restore accepted a changed live source")
			}
			writeFile(t, path, original)
			restored, captured, err := RestoreFrontendSnapshotRecordsWithInputs(p, schema.Schema{}, records.Bytes(), mustCaptureFrontendInputs(t, p))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(index, restored) || captured.Sources.Stats().PhysicalReadAttempts != 0 {
				t.Fatal("record restore changed the index or reread sources")
			}
			if err := ValidateBuildGeneration(restored, &captured); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func mustCaptureFrontendInputs(t *testing.T, p project.Project) *FrontendInputs {
	t.Helper()
	inputs, err := CaptureFrontendInputs(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	return inputs
}
