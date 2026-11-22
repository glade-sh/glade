package dml

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

func TestNameVariantTablesLazyConcurrent(t *testing.T) {
	wantPairs := map[string]bool{}
	wantHeads := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(nameVariantTable), "\n") {
		names := strings.Fields(line)
		if len(names) < 2 {
			continue
		}
		for i := range names {
			names[i] = strings.ReplaceAll(names[i], "_", " ")
		}
		wantHeads[names[0]] = true
		for _, existing := range names[1:] {
			wantPairs[names[0]+"\x00"+existing] = true
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Go(func() {
			pairs, heads := getNameVariantTables()
			if len(pairs) != len(wantPairs) || len(heads) != len(wantHeads) {
				t.Error("table sizes differ")
				return
			}
			for pair := range wantPairs {
				if !pairs[pair] {
					t.Errorf("missing pair %q", pair)
					return
				}
			}
			for head := range wantHeads {
				if !heads[head] {
					t.Errorf("missing head %q", head)
					return
				}
			}
			if pairs["not-a-name\x00not-a-variant"] || heads["not-a-name"] {
				t.Error("unknown name matched")
			}
		})
	}
	wg.Wait()
}

func BenchmarkNameVariantLookup(b *testing.B) {
	for b.Loop() {
		_ = nameVariantOf("alex", "alexander")
	}
}

func TestNameVariantTablesStartUninitialized(t *testing.T) {
	if os.Getenv("GLADE_NAME_VARIANTS_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestNameVariantTablesStartUninitialized$")
		cmd.Env = append(os.Environ(), "GLADE_NAME_VARIANTS_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fresh process: %v\n%s", err, output)
		}
		return
	}
	if nameVariants != nil || nameVariantHeads != nil {
		t.Fatal("name tables initialized at startup")
	}
	pairs, heads := getNameVariantTables()
	if len(pairs) == 0 || len(heads) == 0 {
		t.Fatal("name tables not initialized on use")
	}
}
