package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestRunCheckPerfJSONPreservesEveryOutputFormat(t *testing.T) {
	root := writePerfCheckProject(t, true)
	for _, format := range []string{"text", "json", "sarif", "github"} {
		t.Run(format, func(t *testing.T) {
			baselineArgs := []string{"check", "--project", root, "--format", format, "--no-progress"}
			baselineCode, baselineStdout, baselineStderr := runPerfCLI(t, context.Background(), baselineArgs...)

			perfPath := filepath.Join(t.TempDir(), "check-perf.json")
			profiledArgs := append(append([]string{}, baselineArgs...), "--perf-json", perfPath)
			profiledCode, profiledStdout, profiledStderr := runPerfCLI(t, context.Background(), profiledArgs...)

			if profiledCode != baselineCode {
				t.Fatalf("profiled exit code = %d, want baseline %d; stderr=%q", profiledCode, baselineCode, profiledStderr)
			}
			if !bytes.Equal(profiledStdout, baselineStdout) {
				t.Fatalf("profiled stdout changed for %s\nbaseline: %q\nprofiled: %q", format, baselineStdout, profiledStdout)
			}
			if !bytes.Equal(profiledStderr, baselineStderr) {
				t.Fatalf("profiled stderr changed for %s\nbaseline: %q\nprofiled: %q", format, baselineStderr, profiledStderr)
			}
			readPerfJSONObject(t, perfPath)
		})
	}
}

func TestRunCheckPerfJSONCapturesLowerCamelPhaseAndSemaCounters(t *testing.T) {
	root := writePerfCheckProject(t, false)
	perfPath := filepath.Join(t.TempDir(), "check-perf.json")

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"check", "--project", root, "--format", "json", "--no-progress", "--perf-json", perfPath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	perf := readPerfJSONObject(t, perfPath)
	for _, key := range []string{
		"schemaVersion", "command", "generatedAt", "project", "status", "exitCode", "totalMs", "summary",
		"projectLoadNs", "schemaLoadNs", "indexBuildNs", "outputNs", "semaPerf",
	} {
		requireJSONKey(t, perf, key)
	}
	if perf["command"] != "check" || perf["status"] != "passed" || perf["exitCode"] != float64(0) {
		t.Fatalf("check perf identity = %#v", perf)
	}
	for _, key := range []string{"projectLoadNs", "schemaLoadNs", "indexBuildNs", "outputNs"} {
		requirePositiveJSONNumber(t, perf, key)
	}
	semaPerf := requireJSONObject(t, perf, "semaPerf")
	if semaPerf["enabled"] != true {
		t.Fatalf("semaPerf.enabled = %#v, want true", semaPerf["enabled"])
	}
	for _, key := range []string{
		"totalNs", "sourceSchemaEnrichment", "platformModel", "typeMemberModel", "methodBodies",
		"inheritance", "querySemantics", "export", "mallocs", "totalAllocBytes", "gcCount", "gcPauseNs",
	} {
		requireJSONKey(t, semaPerf, key)
	}
	requirePositiveJSONNumber(t, semaPerf, "totalNs")
	methodBodies := requireJSONObject(t, semaPerf, "methodBodies")
	requirePositiveJSONNumber(t, methodBodies, "calls")
	requirePositiveJSONNumber(t, methodBodies, "durationNs")
	assertLowerCamelJSONKeys(t, perf, "")
}

func TestRunCheckArtifactPermissionsAreRestricted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	root := writePerfCheckProject(t, false)
	directory := filepath.Join(t.TempDir(), "check-artifacts")
	outputPath := filepath.Join(directory, "check.json")
	perfPath := filepath.Join(directory, "private", "perf.json")

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"check", "--project", root, "--format", "json", "--no-progress",
		"--output", outputPath, "--perf-json", perfPath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertPathPermissions(t, directory, 0o750)
	assertPathPermissions(t, outputPath, 0o640)
	assertPathPermissions(t, filepath.Dir(perfPath), 0o700)
	assertPathPermissions(t, perfPath, 0o600)
}

func TestRunCheckPerfProfilesCloseOnSuccessDiagnosticsAndError(t *testing.T) {
	successRoot := writePerfCheckProject(t, false)
	diagnosticRoot := writePerfCheckProject(t, true)
	invalidRoot := t.TempDir()
	writeTestFile(t, filepath.Join(invalidRoot, "sfdx-project.json"), "{not-json\n")
	tests := []struct {
		name     string
		root     string
		wantCode int
		wantPerf bool
	}{
		{name: "success", root: successRoot, wantCode: 0, wantPerf: true},
		{name: "diagnostics", root: diagnosticRoot, wantCode: 1, wantPerf: true},
		{name: "project load error", root: invalidRoot, wantCode: 1, wantPerf: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cpuPath := filepath.Join(dir, "cpu.pprof")
			memPath := filepath.Join(dir, "mem.pprof")
			perfPath := filepath.Join(dir, "check-perf.json")
			code, stdout, stderr := runPerfCLI(t, context.Background(),
				"check", "--project", tc.root, "--no-progress",
				"--cpu-profile", cpuPath, "--mem-profile", memPath, "--perf-json", perfPath,
			)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, tc.wantCode, stdout, stderr)
			}
			requireNonEmptyFile(t, cpuPath)
			requireNonEmptyFile(t, memPath)
			if tc.wantPerf {
				readPerfJSONObject(t, perfPath)
			} else if _, err := os.Stat(perfPath); !os.IsNotExist(err) {
				t.Fatalf("perf JSON should not be written for project-load errors: %v", err)
			}
		})
	}

	// Starting another CPU profile proves the error path stopped the process-global profiler.
	probePath := filepath.Join(t.TempDir(), "probe.pprof")
	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"check", "--project", successRoot, "--no-progress", "--cpu-profile", probePath,
	)
	if code != 0 {
		t.Fatalf("profile closure probe exit code = %d; stdout=%q stderr=%q", code, stdout, stderr)
	}
	requireNonEmptyFile(t, probePath)
}

func TestRunCheckPerfRejectsPairwiseArtifactAliasesBeforeWrites(t *testing.T) {
	destinations := []struct {
		name string
		flag string
	}{
		{name: "--output", flag: "--output"},
		{name: "--perf-json", flag: "--perf-json"},
		{name: "--cpu-profile", flag: "--cpu-profile"},
		{name: "--mem-profile", flag: "--mem-profile"},
	}
	for first := 0; first < len(destinations); first++ {
		for second := first + 1; second < len(destinations); second++ {
			firstDestination := destinations[first]
			secondDestination := destinations[second]
			name := strings.TrimPrefix(firstDestination.name, "--") + "_and_" + strings.TrimPrefix(secondDestination.name, "--")
			t.Run(name, func(t *testing.T) {
				root := writePerfCheckProject(t, false)
				directory := t.TempDir()
				paths := []string{
					filepath.Join(directory, "check-output.txt"),
					filepath.Join(directory, "check-perf.json"),
					filepath.Join(directory, "cpu.pprof"),
					filepath.Join(directory, "mem.pprof"),
				}
				sharedPath := filepath.Join(directory, "shared-artifact")
				paths[first] = sharedPath
				paths[second] = lexicalArtifactAlias(t, sharedPath)
				args := []string{"check", "--project", root, "--no-progress"}
				for index, destination := range destinations {
					args = append(args, destination.flag, paths[index])
				}

				code, stdout, stderr := runPerfCLI(t, context.Background(), args...)

				if code != 1 {
					t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
				}
				for _, want := range []string{firstDestination.name, secondDestination.name, "same path"} {
					if !strings.Contains(string(stderr), want) {
						t.Fatalf("stderr = %q, want %q", stderr, want)
					}
				}
				assertArtifactFilesAbsent(t, paths...)
			})
		}
	}
}

func TestRunCheckPerfRejectsSymlinkAncestorAliasWithMissingParentsBeforeWrites(t *testing.T) {
	root := writePerfCheckProject(t, false)
	directory := t.TempDir()
	realDirectory := filepath.Join(directory, "real")
	aliasDirectory := filepath.Join(directory, "alias")
	if err := os.Mkdir(realDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDirectory, aliasDirectory); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	outputPath := filepath.Join(realDirectory, "missing", "nested", "shared-artifact")
	perfPath := filepath.Join(aliasDirectory, "missing", "nested", "shared-artifact")

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"check", "--project", root, "--no-progress", "--output", outputPath, "--perf-json", perfPath,
	)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"--output", "--perf-json", "same path"} {
		if !strings.Contains(string(stderr), want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	assertArtifactFilesAbsent(t, outputPath, perfPath)
}

func TestRunCheckPerfRejectsExistingHardLinkedDestinationsBeforeWrites(t *testing.T) {
	root := writePerfCheckProject(t, false)
	directory := t.TempDir()
	outputPath := filepath.Join(directory, "check-output.json")
	perfPath := filepath.Join(directory, "check-perf.json")
	const sentinel = "artifact sentinel\n"
	writeTestFile(t, outputPath, sentinel)
	if err := os.Link(outputPath, perfPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"check", "--project", root, "--format", "json", "--no-progress", "--output", outputPath, "--perf-json", perfPath,
	)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"--output", "--perf-json", "same path"} {
		if !strings.Contains(string(stderr), want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	for _, path := range []string{outputPath, perfPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != sentinel {
			t.Fatalf("%s changed before alias rejection: %q", path, data)
		}
	}
}

func TestValidateDistinctCLIArtifactDestinationsFastPathSkipsResolutionAndAllocation(t *testing.T) {
	invalidPath := string([]byte{'a', 0, 'b'})
	for _, tc := range []struct {
		name         string
		destinations []cliArtifactDestination
	}{
		{name: "no destinations"},
		{
			name: "automatic history only",
			destinations: []cliArtifactDestination{
				{Name: "--perf-json"},
				{Name: "automatic duration history", Path: invalidPath},
				{Name: "--junit"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotErr error
			allocations := testing.AllocsPerRun(100, func() {
				gotErr = validateDistinctCLIArtifactDestinations(tc.destinations...)
			})
			if gotErr != nil {
				t.Fatalf("validateDistinctCLIArtifactDestinations error = %v, want fast-path nil", gotErr)
			}
			if allocations != 0 {
				t.Fatalf("validateDistinctCLIArtifactDestinations allocations = %v, want 0", allocations)
			}
		})
	}
}

func TestRunTestPerfJSONWiresPreRunAndRunnerPhasesAndPreservesResultIdentity(t *testing.T) {
	root := writePerfTestProject(t)
	baseArgs := []string{"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache"}
	baselineCode, baselineStdout, baselineStderr := runPerfCLI(t, context.Background(), baseArgs...)
	if baselineCode != 0 {
		t.Fatalf("baseline exit code = %d; stdout=%q stderr=%q", baselineCode, baselineStdout, baselineStderr)
	}

	perfPath := filepath.Join(t.TempDir(), "test-perf.json")
	profiledArgs := append(append([]string{}, baseArgs...), "--perf-json", perfPath)
	profiledCode, profiledStdout, profiledStderr := runPerfCLI(t, context.Background(), profiledArgs...)
	if profiledCode != baselineCode {
		t.Fatalf("profiled exit code = %d, want %d; stdout=%q stderr=%q", profiledCode, baselineCode, profiledStdout, profiledStderr)
	}
	if !bytes.Equal(profiledStderr, baselineStderr) {
		t.Fatalf("profiled stderr changed\nbaseline: %q\nprofiled: %q", baselineStderr, profiledStderr)
	}
	if got, want := perfTestResultIdentity(t, profiledStdout), perfTestResultIdentity(t, baselineStdout); !reflect.DeepEqual(got, want) {
		t.Fatalf("profiled result identity changed\nbaseline: %#v\nprofiled: %#v", want, got)
	}

	perf := readPerfJSONObject(t, perfPath)
	apexPerf := requireJSONObject(t, perf, "apexPerf")
	if apexPerf["enabled"] != true {
		t.Fatalf("apexPerf.enabled = %#v, want true", apexPerf["enabled"])
	}
	phases := requireJSONObject(t, apexPerf, "phases")
	for _, key := range []string{
		"projectLoadNs", "schemaLoadNs", "indexBuildNs", "discoverNs", "runtimeKeyNs",
		"projectCompileNs", "testCompileNs", "classSetupNs", "methodRunNs", "reportAssemblyNs",
	} {
		requirePositiveJSONNumber(t, phases, key)
	}
}

func TestRunTestPerfJSONCreatesPrivateArtifact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	root := writePerfTestProject(t)
	directory := filepath.Join(t.TempDir(), "private-perf")
	perfPath := filepath.Join(directory, "test-perf.json")

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--perf-json", perfPath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertPathPermissions(t, directory, 0o700)
	assertPathPermissions(t, perfPath, 0o600)
}

func TestRunTestPerfJSONPreservesExistingArtifactPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	root := writePerfTestProject(t)
	perfPath := filepath.Join(t.TempDir(), "caller-owned-perf.json")
	if err := os.WriteFile(perfPath, []byte("sentinel\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--perf-json", perfPath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	readPerfJSONObject(t, perfPath)
	assertPathPermissions(t, perfPath, 0o640)
}

func TestRunTestPerfJSONPreservesLegacyShapeAndDerivesDurationsFromPhases(t *testing.T) {
	root := writePerfTestProject(t)
	dir := t.TempDir()
	perfPath := filepath.Join(dir, "test-perf.json")
	cpuPath := filepath.Join(dir, "cpu.pprof")
	memPath := filepath.Join(dir, "mem.pprof")

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--perf-json", perfPath, "--cpu-profile", cpuPath, "--mem-profile", memPath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	perf := readPerfJSONObject(t, perfPath)
	for _, key := range []string{
		"generatedAt", "project", "durationMs", "discoverMs", "compileMs", "totalMs", "summary", "apexPerf",
		"classDurations", "methodDurations", "topSlowClasses", "cpuProfilePath", "memProfilePath",
		"schemaVersion", "command", "status", "exitCode", "startupMs", "historyWriteNs",
	} {
		requireJSONKey(t, perf, key)
	}
	if perf["command"] != "test" || perf["status"] != "passed" || perf["exitCode"] != float64(0) {
		t.Fatalf("test perf identity = %#v", perf)
	}
	phases := requireJSONObject(t, requireJSONObject(t, perf, "apexPerf"), "phases")
	discoverMS := int64(requireJSONNumber(t, phases, "discoverNs")) / int64(time.Millisecond)
	if got := int64(requireJSONNumber(t, perf, "discoverMs")); got != discoverMS {
		t.Fatalf("discoverMs = %d, want discoverNs/1ms = %d", got, discoverMS)
	}
	compileNS := int64(requireJSONNumber(t, phases, "projectCompileNs")) + int64(requireJSONNumber(t, phases, "testCompileNs"))
	if got, want := int64(requireJSONNumber(t, perf, "compileMs")), compileNS/int64(time.Millisecond); got != want {
		t.Fatalf("compileMs = %d, want (projectCompileNs+testCompileNs)/1ms = %d", got, want)
	}
	startupPhaseKeys := []string{
		"projectLoadNs", "schemaLoadNs", "indexBuildNs", "discoverNs", "runtimeKeyNs",
		"cacheValidateNs", "cacheDecodeNs", "cacheEncodeNs", "orgBuildNs", "projectCompileNs", "testCompileNs",
	}
	var startupNS int64
	for _, key := range startupPhaseKeys {
		startupNS += int64(optionalJSONNumber(t, phases, key))
	}
	if got, want := int64(requireJSONNumber(t, perf, "startupMs")), startupNS/int64(time.Millisecond); got != want {
		t.Fatalf("startupMs = %d, want phase sum / 1ms = %d", got, want)
	}
	if got, startup := int64(requireJSONNumber(t, perf, "totalMs")), int64(requireJSONNumber(t, perf, "startupMs")); got < startup {
		t.Fatalf("totalMs = %d, want at least startupMs %d", got, startup)
	}
	requirePositiveJSONNumber(t, perf, "historyWriteNs")
	requireNonEmptyFile(t, defaultCLIDurationHistoryPath(root))
	requireNonEmptyFile(t, cpuPath)
	requireNonEmptyFile(t, memPath)
}

func TestRunTestPerfJSONRejectsNonlocalModes(t *testing.T) {
	for _, mode := range []string{"--connect", "--daemon", "--watch", "--watch-once"} {
		t.Run(strings.TrimPrefix(mode, "--"), func(t *testing.T) {
			root := writePerfTestProject(t)
			perfPath := filepath.Join(t.TempDir(), "test-perf.json")
			ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
			defer cancel()
			code, stdout, stderr := runPerfCLI(t, ctx,
				"test", "--project", root, "--json", "--no-progress", mode, "--perf-json", perfPath,
			)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
			}
			want := "--perf-json cannot be combined with " + mode
			if !strings.Contains(string(stderr), want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if _, err := os.Stat(perfPath); !os.IsNotExist(err) {
				t.Fatalf("perf JSON should not be written for %s: %v", mode, err)
			}
		})
	}
}

func TestRunTestPerfJSONRejectsDurationHistoryPathConflict(t *testing.T) {
	root := writePerfTestProject(t)
	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "history.json")
	writeTestFile(t, sharedPath, "{}\n")
	conflictingPerfPath := dir + string(os.PathSeparator) + "." + string(os.PathSeparator) + "history.json"

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--duration-history", sharedPath, "--perf-json", conflictingPerfPath,
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if want := "--perf-json and --duration-history must use different paths"; !strings.Contains(string(stderr), want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestRunTestPerfRejectsPairwiseArtifactAliasesBeforeWrites(t *testing.T) {
	destinations := []struct {
		name string
		flag string
	}{
		{name: "automatic duration history"},
		{name: "--perf-json", flag: "--perf-json"},
		{name: "--cpu-profile", flag: "--cpu-profile"},
		{name: "--mem-profile", flag: "--mem-profile"},
		{name: "--trace", flag: "--trace"},
		{name: "--junit", flag: "--junit"},
	}
	for first := 0; first < len(destinations); first++ {
		for second := first + 1; second < len(destinations); second++ {
			firstDestination := destinations[first]
			secondDestination := destinations[second]
			name := strings.NewReplacer("--", "", " ", "_").Replace(firstDestination.name) + "_and_" + strings.NewReplacer("--", "", " ", "_").Replace(secondDestination.name)
			t.Run(name, func(t *testing.T) {
				root := writePerfTestProject(t)
				directory := t.TempDir()
				paths := []string{
					defaultCLIDurationHistoryPath(root),
					filepath.Join(directory, "test-perf.json"),
					filepath.Join(directory, "cpu.pprof"),
					filepath.Join(directory, "mem.pprof"),
					filepath.Join(directory, "trace.json"),
					filepath.Join(directory, "junit.xml"),
				}
				sharedPath := filepath.Join(directory, "shared-artifact")
				if first == 0 || second == 0 {
					sharedPath = defaultCLIDurationHistoryPath(root)
				}
				paths[first] = sharedPath
				paths[second] = lexicalArtifactAlias(t, sharedPath)
				args := []string{"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache"}
				for index, destination := range destinations {
					if destination.flag != "" {
						args = append(args, destination.flag, paths[index])
					}
				}

				code, stdout, stderr := runPerfCLI(t, context.Background(), args...)

				if code != 1 {
					t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
				}
				for _, want := range []string{firstDestination.name, secondDestination.name, "same path"} {
					if !strings.Contains(string(stderr), want) {
						t.Fatalf("stderr = %q, want %q", stderr, want)
					}
				}
				assertArtifactFilesAbsent(t, paths...)
			})
		}
	}
}

func TestRunTestPerfJSONExactSelectorKeepsCurrentPreRunSnapshotWithoutStaleRunnerData(t *testing.T) {
	root := writePerfTestProject(t)
	seedPath := filepath.Join(t.TempDir(), "seed-perf.json")
	seedCode, seedStdout, seedStderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--class", "PerfWiringTest", "--perf-json", seedPath,
	)
	if seedCode != 0 {
		t.Fatalf("seed exit code = %d, want 0; stdout=%q stderr=%q", seedCode, seedStdout, seedStderr)
	}
	seedPhases := requireJSONObject(t, requireJSONObject(t, readPerfJSONObject(t, seedPath), "apexPerf"), "phases")
	requirePositiveJSONNumber(t, seedPhases, "methodRunNs")

	selectorPath := filepath.Join(t.TempDir(), "selector-perf.json")
	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--class", "MissingPerfWiringTest", "--perf-json", selectorPath,
	)
	if code != 1 {
		t.Fatalf("selector exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
	}
	perf := readPerfJSONObject(t, selectorPath)
	if perf["status"] != "failed" || perf["exitCode"] != float64(1) {
		t.Fatalf("selector perf identity = %#v", perf)
	}
	apexPerf := requireJSONObject(t, perf, "apexPerf")
	if apexPerf["enabled"] != true {
		t.Fatalf("selector apexPerf.enabled = %#v, want true", apexPerf["enabled"])
	}
	phases := requireJSONObject(t, apexPerf, "phases")
	for _, key := range []string{"projectLoadNs", "schemaLoadNs", "indexBuildNs", "discoverNs"} {
		requirePositiveJSONNumber(t, phases, key)
	}
	for _, key := range []string{"runtimeKeyNs", "projectCompileNs", "testCompileNs", "classSetupNs", "methodRunNs", "reportAssemblyNs"} {
		if got := optionalJSONNumber(t, phases, key); got != 0 {
			t.Fatalf("selector %s = %v, want 0 without stale runner data", key, got)
		}
	}
}

func TestRunTestPerfJSONRejectsEarlySuccessAndArtifactOnlyModes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flag      string
		extraArgs func(t *testing.T) ([]string, []string)
	}{
		{name: "wizard", flag: "--wizard"},
		{name: "last failed", flag: "--last-failed"},
		{
			name: "write class shards",
			flag: "--write-class-shards",
			extraArgs: func(t *testing.T) ([]string, []string) {
				shardsPath := filepath.Join(t.TempDir(), "shards")
				return []string{"--shard-count", "2", "--write-class-shards", shardsPath}, []string{shardsPath}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writePerfTestProject(t)
			perfPath := filepath.Join(t.TempDir(), "test-perf.json")
			args := []string{"test", "--project", root, "--json", "--no-progress", "--perf-json", perfPath}
			forbiddenPaths := []string{perfPath}
			if tc.extraArgs == nil {
				args = append(args, tc.flag)
			} else {
				extraArgs, extraForbiddenPaths := tc.extraArgs(t)
				args = append(args, extraArgs...)
				forbiddenPaths = append(forbiddenPaths, extraForbiddenPaths...)
			}

			code, stdout, stderr := runPerfCLI(t, context.Background(), args...)

			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
			}
			want := "--perf-json cannot be combined with " + tc.flag
			if !strings.Contains(string(stderr), want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			assertArtifactFilesAbsent(t, forbiddenPaths...)
		})
	}
}

func TestRunTestPerfProfilesAloneDoNotEnablePerfCounters(t *testing.T) {
	root := writePerfTestProject(t)
	memPath := filepath.Join(t.TempDir(), "mem.pprof")
	apextest.ResetPerfCounters()

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache", "--mem-profile", memPath,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := apextest.SnapshotPerfCounters(); got.Enabled {
		t.Fatalf("profile-only run enabled perf counters: %#v", got)
	}
	requireNonEmptyFile(t, memPath)
}

func TestRunTestAutomaticHistoryOnlyPreservesNormalRun(t *testing.T) {
	root := writePerfTestProject(t)

	code, stdout, stderr := runPerfCLI(t, context.Background(),
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	requireNonEmptyFile(t, defaultCLIDurationHistoryPath(root))
}

func writePerfCheckProject(t *testing.T, diagnostics bool) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	body := `public class PerfCheck { public Integer value() { return 42; } }`
	if diagnostics {
		body = `public class PerfCheck { public MissingType value() { return null; } }`
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/PerfCheck.cls"), body)
	return root
}

func writePerfTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/PerfWiringTest.cls"), `
@isTest
private class PerfWiringTest {
  @testSetup static void setupData() {
    Integer seed = 41;
    System.assertEquals(41, seed);
  }

  @isTest static void passes() {
    System.assertEquals(42, 40 + 2);
  }
}
`)
	return root
}

func runPerfCLI(t *testing.T, ctx context.Context, args ...string) (int, []byte, []byte) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, args, &stdout, &stderr)
	return code, append([]byte(nil), stdout.Bytes()...), append([]byte(nil), stderr.Bytes()...)
}

func readPerfJSONObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, data)
	}
	return got
}

func requireNonEmptyFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s is empty", path)
	}
}

func assertPathPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s permissions = %04o, want %04o", path, got, want)
	}
}

func lexicalArtifactAlias(t *testing.T, path string) string {
	t.Helper()
	directory := filepath.Dir(path)
	aliasDirectory := filepath.Join(directory, ".artifact-alias")
	if err := os.MkdirAll(aliasDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	return aliasDirectory + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(path)
}

func assertArtifactFilesAbsent(t *testing.T, paths ...string) {
	t.Helper()
	checked := map[string]bool{}
	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		if checked[cleanPath] {
			continue
		}
		checked[cleanPath] = true
		if _, err := os.Stat(cleanPath); !os.IsNotExist(err) {
			t.Fatalf("artifact %s should not exist before destination validation: %v", cleanPath, err)
		}
	}
}

func requireJSONKey(t *testing.T, object map[string]any, key string) {
	t.Helper()
	if _, ok := object[key]; !ok {
		t.Fatalf("JSON object missing %q: %#v", key, object)
	}
}

func requireJSONObject(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := object[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", key, object[key])
	}
	return value
}

func requireJSONNumber(t *testing.T, object map[string]any, key string) float64 {
	t.Helper()
	value, ok := object[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", key, object[key])
	}
	return value
}

func optionalJSONNumber(t *testing.T, object map[string]any, key string) float64 {
	t.Helper()
	value, ok := object[key]
	if !ok {
		return 0
	}
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", key, value)
	}
	return number
}

func requirePositiveJSONNumber(t *testing.T, object map[string]any, key string) {
	t.Helper()
	if got := requireJSONNumber(t, object, key); got <= 0 {
		t.Fatalf("%s = %v, want > 0", key, got)
	}
}

func assertLowerCamelJSONKeys(t *testing.T, value any, path string) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			keyPath := key
			if path != "" {
				keyPath = path + "." + key
			}
			first, _ := utf8.DecodeRuneInString(key)
			if key == "" || unicode.IsUpper(first) || strings.Contains(key, "_") {
				t.Fatalf("JSON key %q is not lower-camel", keyPath)
			}
			assertLowerCamelJSONKeys(t, child, keyPath)
		}
	case []any:
		for _, child := range value {
			assertLowerCamelJSONKeys(t, child, path)
		}
	}
}

func perfTestResultIdentity(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode test JSON: %v\n%s", err, data)
	}
	items, ok := envelope["tests"].([]any)
	if !ok {
		t.Fatalf("test JSON tests = %#v", envelope["tests"])
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		test, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("test JSON item = %#v", item)
		}
		identity := map[string]any{}
		for _, key := range []string{"className", "methodName", "name", "status", "message"} {
			if value, ok := test[key]; ok {
				identity[key] = value
			}
		}
		out = append(out, identity)
	}
	return out
}
