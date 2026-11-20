package soql

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
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
