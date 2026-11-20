package soql

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/collate/build"
)

func TestLocaleCollationColdQuery(t *testing.T) {
	locale := os.Getenv("GLADE_COLD_LOCALE")
	if locale == "" {
		t.Skip("per-process locale required")
	}
	raw, err := os.ReadFile("testdata/locale_collation/maps/V-values.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatal("invalid frozen locale input")
		}
		values = append(values, fields[2])
	}
	org := collationOrg(values, "Account", map[string]bool{"Name": true})
	query, err := Parse("SELECT BillingCity FROM Account ORDER BY Name, BillingCity")
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"first", "warm"} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{Locale: locale})
		duration := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(collationLabels(result, "BillingCity")))
		output, _ := json.Marshal(map[string]any{"locale": locale, "phase": phase, "rows": len(values), "elapsed_ns": duration.Nanoseconds(), "allocated_bytes": after.TotalAlloc - before.TotalAlloc, "allocations": after.Mallocs - before.Mallocs, "digest": hex.EncodeToString(digest[:])})
		fmt.Printf("COLDQUERY %s\n", output)
	}
}

// The generated table must track both the pinned weights and the x/text builder.
func TestLocaleCollationPreparedRootMatchesSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "root.txt")
	referencePath := filepath.Join(t.TempDir(), "reference.json")
	cmd := exec.Command("go", "run", "collation_root_generate.go", path, referencePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate root: %v\n%s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := collationDataBytes("cldr_root_table.b64")
	if !bytes.Equal(got, want) {
		t.Fatal("generated root is stale; run go generate in internal/soql")
	}
	table, err := build.NewBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(want)))
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(table); err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, reference) {
		t.Fatal("decoded table differs from the builder output")
	}
}
