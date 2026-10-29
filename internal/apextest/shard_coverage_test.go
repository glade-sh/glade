package apextest

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestShardFallbackDurationRemainsPositive(t *testing.T) {
	for _, tc := range []struct {
		name      string
		durations map[string]int64
		want      int64
	}{
		{"empty", nil, 1},
		{"single zero", map[string]int64{"TestA": 0}, 1},
		{"all zero", map[string]int64{"TestA": 0, "TestB": 0, "TestC": 0}, 1},
		{"zero median", map[string]int64{"TestA": 0, "TestB": 0, "TestC": 40}, 1},
		{"positive median", map[string]int64{"TestA": 1, "TestB": 5, "TestC": 9}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shardMedianDuration(tc.durations); got != tc.want {
				t.Fatalf("fallback=%d ms; want %d ms", got, tc.want)
			}
		})
	}
}

func TestShardZeroMeasurementsDoNotCollapseUnmeasuredFamilies(t *testing.T) {
	for _, measured := range []int{1, 3, 8} {
		t.Run(fmt.Sprint(measured), func(t *testing.T) {
			durations := make(map[string]int64)
			var names []string
			for i := 0; i < measured; i++ {
				name := fmt.Sprintf("TestMeasured%d", i)
				durations[name] = 0
				names = append(names, name)
			}
			for i := 0; i < 12; i++ {
				names = append(names, fmt.Sprintf("TestUnmeasured%d", i))
			}
			for index, shard := range shardAssignment(names, durations, 3) {
				unmeasured := 0
				for _, name := range shard {
					if strings.HasPrefix(name, "TestUnmeasured") {
						unmeasured++
					}
				}
				if unmeasured != 4 {
					t.Errorf("shard %d owns %d unmeasured families; want 4", index, unmeasured)
				}
			}
		})
	}
}

func TestShardMeasuredTableCoversCompletedMeasurements(t *testing.T) {
	durations, err := loadShardDurationTable(shardDurationTablePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(durations) == 0 {
		t.Fatal("shard balancing needs measured durations")
	}
	for _, name := range []string{
		"TestAssignmentSaveNativeConformanceAPIRoutes",
		"TestAssignmentSaveUserModeNativeConformanceAPIRoutes",
		"TestAbstractHierarchyOrgConformance",
	} {
		if durations[name] <= 0 {
			t.Errorf("completed measurement missing for %s", name)
		}
	}
}

func TestShardRunPatternsPartitionLiveRegistry(t *testing.T) {
	names, err := discoverShardTestNames()
	if err != nil {
		t.Fatal(err)
	}
	durations, err := loadShardDurationTable(shardDurationTablePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{1, 2, 5, 16} {
		seen := make(map[string]int)
		for _, selected := range shardAssignment(names, durations, count) {
			pattern, err := shardRunPattern(selected, "")
			if err != nil {
				t.Fatal(err)
			}
			match, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				if match.MatchString(name) {
					seen[name]++
				}
			}
		}
		for _, name := range names {
			if seen[name] != 1 {
				t.Errorf("%d shards select %s %d times", count, name, seen[name])
			}
		}
	}
}

func TestShardChildOutcomesMatchUnsharded(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(spec string) map[string]int {
		t.Helper()
		cmd := exec.Command(executable, "-test.v", "-test.run", "^TestShardFilterFixture[AB]$")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, shardSelectorEnv+"=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, shardSelectorEnv+"="+spec)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("shard %q: %v\n%s", spec, err, output)
		}
		outcomes := make(map[string]int)
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[0] == "---" {
				outcomes[fields[1]+" "+fields[2]]++
			}
		}
		return outcomes
	}
	want := run("")
	if len(want) != 6 {
		t.Fatalf("unsharded fixture outcomes: %v", want)
	}
	got := make(map[string]int)
	for index := 0; index < 3; index++ {
		for outcome, count := range run(fmt.Sprintf("%d/3", index)) {
			got[outcome] += count
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sharded outcomes=%v; unsharded=%v", got, want)
	}
}
