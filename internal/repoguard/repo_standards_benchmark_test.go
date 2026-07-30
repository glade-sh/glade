package repoguard

import (
	"fmt"
	"testing"
)

var (
	privatePatternSliceSink []privatePackagePattern
	privateFindingCountSink int
)

func TestPrivateExamplePackagePatternSetupReducesAllocations(t *testing.T) {
	legacy := testing.AllocsPerRun(20, func() {
		privatePatternSliceSink = buildPrivateExamplePackagePatterns()
	})
	cached := testing.AllocsPerRun(20, func() {
		privatePatternSliceSink = privateExamplePackagePatterns()
	})
	if cached*5 > legacy {
		t.Fatalf("compiled pattern setup allocations = %.1f, want at least 80%% below %.1f", cached, legacy)
	}
}

func BenchmarkRepoGuardPatternSetup(b *testing.B) {
	b.Run("compile-each-time", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			privatePatternSliceSink = buildPrivateExamplePackagePatterns()
		}
	})
	b.Run("reuse-compiled-set", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			privatePatternSliceSink = privateExamplePackagePatterns()
		}
	})
}

func BenchmarkRepoGuardPatternScan7354Files(b *testing.B) {
	files := syntheticRepoGuardTexts(7354)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		findings := 0
		for _, text := range files {
			if privateExamplePackageFinding(text) != "" {
				findings++
			}
		}
		privateFindingCountSink = findings
	}
}

func syntheticRepoGuardTexts(count int) []string {
	files := make([]string, count)
	for index := range files {
		files[index] = fmt.Sprintf("docs/example-%d.md macrodata-apex refinement-local", index)
	}
	files[count/2] = hyphen("sf", "cred", "pkg", "develop")
	return files
}
