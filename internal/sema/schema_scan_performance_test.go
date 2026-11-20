package sema

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

// baselineSchemaScan retains the original scanner as an outcome oracle.
func baselineSchemaScan(ctx *semaProjectReferencedSchemaContext, source string, vars map[string]string) ([]semaProjectReferencedSchemaFieldReference, []semaProjectReferencedSchemaFieldReference) {
	literal := make([]semaProjectReferencedSchemaFieldReference, 0, 16)
	member := make([]semaProjectReferencedSchemaFieldReference, 0, 16)
	ls := make(map[semaProjectReferencedSchemaFieldReference]struct{}, 16)
	ms := make(map[semaProjectReferencedSchemaFieldReference]struct{}, 16)
	lookup := make(map[semaProjectReferencedLookupFieldReference]struct{}, 16)
	for i := 0; i < len(source); {
		start, end, ok := semaProjectReferencedScanIdentifier(source, i)
		if !ok {
			break
		}
		next := end
		if n := semaCollectProjectReferencedSObjectLiteralAt(ctx, source, vars, start, end, &literal, ls, lookup); n > next {
			next = n
		}
		if n := semaCollectProjectReferencedMemberFieldAt(ctx, source, vars, start, end, &member, ms); n > next {
			next = n
		}
		i = next
	}
	return literal, member
}

func TestSchemaScanAndDigestIdentity(t *testing.T) {
	sources := []string{"", "public class Plain { void run() { String s = 'hello'; } }", "Account a = new Account(Name='x', Name='x'); String x = a.Name;", "Parent__c p = new Parent__c(); Child__c c = new Child__c(Parent__c=p.Id, Value__c=42); c.Value__c;", strings.Repeat("Account a = new Account(Name='x'); a.Name; ", 40), "Account a = new Account(Name=", "// no field\nString x = 'é';"}
	for _, source := range sources {
		objects := []schema.Object{{Name: "Account"}, {Name: "Parent__c"}, {Name: "Child__c"}}
		a := newSemaProjectReferencedSchemaContext(append([]schema.Object(nil), objects...), "")
		b := newSemaProjectReferencedSchemaContext(append([]schema.Object(nil), objects...), "")
		va := semaProjectReferencedSchemaVariableTypes(a, source)
		vb := semaProjectReferencedSchemaVariableTypes(b, source)
		al, am := baselineSchemaScan(a, source, va)
		bl, bm := semaProjectReferencedSchemaFieldReferences(b, source, vb)
		if !reflect.DeepEqual(al, bl) || !reflect.DeepEqual(am, bm) || !reflect.DeepEqual(a, b) {
			t.Fatalf("scanner mismatch for %q", source)
		}
		before, err := json.Marshal([]any{al, am, a.objects})
		if err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal([]any{bl, bm, b.objects})
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("serialized scanner mismatch for %q", source)
		}
	}
	inputs := []string{"", "é"}
	for _, n := range []int{1, 127, 128, 129, 255, 256, 257, 4097} {
		inputs = append(inputs, strings.Repeat("a", n), strings.Repeat("é", n))
	}
	for _, s := range inputs {
		h := &blindHasher{h: sha256.New()}
		h.str(s)
		expected := sha256.New()
		var size [8]byte
		binary.LittleEndian.PutUint64(size[:], uint64(len(s)))
		expected.Write(size[:])
		expected.Write([]byte(s))
		if !reflect.DeepEqual(h.h.Sum(nil), expected.Sum(nil)) {
			t.Fatalf("digest differs at length %d", len(s))
		}
	}
}

func BenchmarkSchemaScanProject(b *testing.B) {
	sources := make([]string, 1531)
	for i := range sources {
		sources[i] = fmt.Sprintf("public class Service%d { public Integer calculate(Integer a) { return a + %d; } }", i, i)
	}
	ctx := newSemaProjectReferencedSchemaContext([]schema.Object{{Name: "Account"}}, "")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range sources {
			semaProjectReferencedSchemaFieldReferences(ctx, s, nil)
		}
	}
}

func BenchmarkSchemaDigest(b *testing.B) {
	values := make([]string, 15000)
	for i := range values {
		values[i] = fmt.Sprintf("Service%d.Field%d", i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		positionBlindDigest(values)
	}
}

func TestSchemaAnalysisSerializedIdentity(t *testing.T) {
	// Other tests can alter process-wide semantic state. Keep the baseline in a
	// fresh process so the serialized oracle uses the same initial state.
	if os.Getenv("GLADE_SCHEMA_IDENTITY_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSchemaAnalysisSerializedIdentity$")
		cmd.Env = append(os.Environ(), "GLADE_SCHEMA_IDENTITY_CHILD=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated identity check: %v\n%s", err, out)
		}
		return
	}
	root := t.TempDir()
	incrementalSyntheticProject(t, root)
	wants := [2][2]string{{"a66583322e0074992e6123980ae17621207c9838179c26c68fce6dfeb1663738", "0a99afbcdfe3fefc4ad4318f143532654395609079818da0837da5b367675f2e"}, {"8bce0731b35f6706731e74cdbc5ef251773ce9f1b1d851bb8731449e863975ef", "0514293e4b0623fcf550328ff5d2c8f5701820cfd9e97e39904c454a9d1f3c66"}}
	for i, broken := range []bool{false, true} {
		if broken {
			incrementalWrite(t, root, "force-app/main/default/classes/LeafRepo.cls", "public class LeafRepo { void run() { MissingType value; } }")
		}
		generation := incrementalLoad(t, root)
		result := AnalyzeWithOptions(generation.index, incrementalOptions(&generation.artifacts))
		if broken && !result.HasErrors() {
			t.Fatal("expected diagnostics")
		}
		for j, value := range []any{result, SnapshotResult(result)} {
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.ReplaceAll(data, []byte(root), []byte("<root>"))
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != wants[i][j] {
				t.Fatalf("broken=%v value=%T digest=%s want=%s", broken, value, got, wants[i][j])
			}
		}
	}
}
