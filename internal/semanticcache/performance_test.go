package semanticcache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func performanceResult(n int) sema.Result {
	r := smallTestResult()
	r.Diagnostics = append(r.Diagnostics, diagnostic.Diagnostic{Message: "second <&> diagnostic", File: "second.cls"})
	r.Types = make(map[string]sema.TypeReference, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("class%05d", i)
		r.Types[name] = sema.TypeReference{Name: name, Kind: sema.TypeApex, Source: name + ".cls"}
	}
	return r
}

func TestPerformanceEnvelopeIdentity(t *testing.T) {
	for _, result := range []sema.Result{{}, {Diagnostics: []diagnostic.Diagnostic{}, Types: map[string]sema.TypeReference{}}, smallTestResult(), performanceResult(20000)} {
		root := t.TempDir()
		id := testIdentity()
		snapshot := sema.SnapshotResult(result)
		payload := checksumPayload{Version: EnvelopeVersion, Identity: id, Result: snapshot}
		checksum, err := payloadChecksum(payload)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := json.Marshal(diskEnvelope{Version: EnvelopeVersion, Identity: id, Result: snapshot, ChecksumSHA256: checksum})
		if err != nil {
			t.Fatal(err)
		}
		if err := Store(root, "cache/result.json", id, result); err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(root, "cache/result.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(expected, actual) {
			t.Fatal("persisted bytes differ from canonical envelope")
		}
		got, err := Load(root, "cache/result.json", id)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, result) {
			t.Fatal("result or diagnostic order changed")
		}
		wantOutput, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		gotOutput, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(wantOutput, gotOutput) {
			t.Fatal("result output changed")
		}
	}
}

func BenchmarkSemanticDiskHit(b *testing.B) {
	root := b.TempDir()
	id := testIdentity()
	result := performanceResult(20000)
	if err := Store(root, "cache/result.json", id, result); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewManager(Limits{})
		got, _, err := m.GetOrCompute(b.Context(), Request{Identity: id, ProjectRoot: root, RelativePath: "cache/result.json"}, func() (sema.Result, error) { return sema.Result{}, fmt.Errorf("unexpected miss") })
		if err != nil || len(got.Types) != len(result.Types) {
			b.Fatal(err)
		}
	}
}

func BenchmarkSemanticIdentity(b *testing.B) {
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: b.TempDir()}, schema.Schema{})
	for i := 0; i < 3000; i++ {
		index.Types = append(index.Types, typesys.TypeSymbol{Name: fmt.Sprintf("Class%05d", i), Members: []typesys.MemberSymbol{{Name: "field", Type: "String"}}})
	}
	options := sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true}
	want, err := IdentityForBuild(index, &artifacts, options)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := IdentityForBuild(index, &artifacts, options)
		if err != nil || got != want {
			b.Fatal("identity changed", err)
		}
	}
}

func TestPerformanceManagerExactEncodedSize(t *testing.T) {
	result := performanceResult(100)
	encoded, err := json.Marshal(sema.SnapshotResult(result))
	if err != nil {
		t.Fatal(err)
	}
	want := int64(len(encoded)) + sema.EstimateResultRetainedBytes(result)
	id := testIdentity()
	root := t.TempDir()
	const path = "cache/result.json"
	if err := Store(root, path, id, result); err != nil {
		t.Fatal(err)
	}
	for _, disk := range []bool{false, true} {
		manager := NewManager(Limits{})
		request := Request{Identity: id, ProjectRoot: root, RelativePath: path, NoDisk: !disk}
		got, access, err := manager.GetOrCompute(context.Background(), request, func() (sema.Result, error) { return result, nil })
		if err != nil || !reflect.DeepEqual(got, result) {
			t.Fatalf("disk=%t: result changed: %v", disk, err)
		}
		if disk && access.Source != SourceDisk || !disk && access.Source != SourceBuild {
			t.Fatalf("disk=%t: source=%s", disk, access.Source)
		}
		if stats := manager.Stats(); stats.RetainedBytes != want {
			t.Fatalf("disk=%t: retained=%d want=%d", disk, stats.RetainedBytes, want)
		}
	}
}
