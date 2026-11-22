package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// BenchmarkPersistedQueryAndWrite includes the full command open and query path;
// fixture creation is outside the timed region for both sub-benchmarks.
func BenchmarkPersistedQueryAndWrite(b *testing.B) {
	root := b.TempDir()
	path := filepath.Join(root, "org.db")
	store, err := storage.OpenSQLite(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	org := storage.NewOrgState()
	object := storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: make(map[storage.ID]storage.Record, 100000),
	}
	for i := 0; i < 100000; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i))
		object.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Account %d", i))}}
	}
	org.Objects["Account"] = object
	if err := store.Save(org); err != nil {
		b.Fatal(err)
	}
	query := "SELECT Id, Name FROM Account WHERE Id = '001000000050000'"
	args := []string{"query", "--db", path, "--project", ".", "--json", query}
	var expected bytes.Buffer
	if err := runDB(context.Background(), args, &expected, io.Discard); err != nil {
		b.Fatal(err)
	}
	if !strings.Contains(expected.String(), "Account 50000") {
		b.Fatalf("query did not return the expected row: %s", expected.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		b.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(expected.Bytes()))
	const baseDigest = "dc281775d253eb70ffd5792502a91b37dfb6aedfacff4fca0dfcf45aad54eff4"
	if digest != baseDigest {
		b.Fatalf("query output SHA-256 = %s, want base %s", digest, baseDigest)
	}
	b.Logf("query output SHA-256: %s; database bytes: %d", digest, info.Size())
	b.Run("one_id_query", func(b *testing.B) {
		b.ReportMetric(float64(info.Size()), "db-bytes")
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var out bytes.Buffer
			if err := runDB(context.Background(), args, &out, io.Discard); err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), expected.Bytes()) {
				b.Fatal("query output changed")
			}
		}
	})
	b.Run("one_record_save", func(b *testing.B) {
		id := storage.ID("001000000050000")
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			record := object.Records[id]
			record.Fields["Name"] = storage.StringValue(fmt.Sprintf("Updated %d", i))
			object.Records[id] = record
			if err := store.Save(org); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		loaded, err := store.Load()
		if err != nil {
			b.Fatal(err)
		}
		got := loaded.Objects["Account"].Records[id].Fields["Name"].String
		if got != fmt.Sprintf("Updated %d", b.N-1) || len(loaded.Objects["Account"].Records) != 100000 {
			b.Fatalf("saved name %q or record count changed", got)
		}
	})
}
