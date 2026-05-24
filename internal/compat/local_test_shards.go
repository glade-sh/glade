package compat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apextest"
)

type localTestClassShard struct {
	Index           int      `json:"index"`
	TotalDurationMS int64    `json:"totalDurationMs"`
	Classes         []string `json:"classes"`
}

type localTestClassWeight struct {
	Class      string
	Methods    int
	DurationMS int64
	Weight     int64
}

func planLocalTestClassShards(cases []apextest.TestCase, durations map[string]int64, shardCount int) []localTestClassShard {
	if shardCount <= 0 {
		return nil
	}
	weights := localTestClassWeights(cases, durations)
	shards := make([]localTestClassShard, shardCount)
	for i := range shards {
		shards[i].Index = i
	}
	for _, weight := range weights {
		target := 0
		for i := 1; i < len(shards); i++ {
			if shards[i].TotalDurationMS < shards[target].TotalDurationMS {
				target = i
			}
		}
		shards[target].Classes = append(shards[target].Classes, weight.Class)
		shards[target].TotalDurationMS += weight.Weight
	}
	return shards
}

func localTestClassWeights(cases []apextest.TestCase, durations map[string]int64) []localTestClassWeight {
	methods := map[string]int{}
	for _, testCase := range cases {
		if testCase.ClassName == "" {
			continue
		}
		methods[testCase.ClassName]++
	}
	weights := make([]localTestClassWeight, 0, len(methods))
	for className, methodCount := range methods {
		duration := durations[className]
		weight := duration
		if weight <= 0 {
			weight = int64(methodCount)
		}
		weights = append(weights, localTestClassWeight{
			Class:      className,
			Methods:    methodCount,
			DurationMS: duration,
			Weight:     weight,
		})
	}
	sort.Slice(weights, func(i, j int) bool {
		if weights[i].Weight == weights[j].Weight {
			return weights[i].Class < weights[j].Class
		}
		return weights[i].Weight > weights[j].Weight
	})
	return weights
}

func selectLocalTestShard(cases []apextest.TestCase, durations map[string]int64, shardCount, shardIndex int) ([]apextest.TestCase, error) {
	if shardCount <= 0 {
		return cases, nil
	}
	if shardIndex < 0 || shardIndex >= shardCount {
		return nil, fmt.Errorf("--shard-index must be between 0 and %d", shardCount-1)
	}
	shards := planLocalTestClassShards(cases, durations, shardCount)
	selected := map[string]bool{}
	for _, className := range shards[shardIndex].Classes {
		selected[className] = true
	}
	out := make([]apextest.TestCase, 0, len(cases))
	for _, testCase := range cases {
		if selected[testCase.ClassName] {
			out = append(out, testCase)
		}
	}
	return out, nil
}

func writeLocalTestClassShardFiles(dir string, shards []localTestClassShard) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	width := len(fmt.Sprintf("%d", len(shards)-1))
	if width < 3 {
		width = 3
	}
	for _, shard := range shards {
		path := filepath.Join(dir, fmt.Sprintf("shard-%0*d.txt", width, shard.Index))
		data := strings.Join(shard.Classes, "\n")
		if data != "" {
			data += "\n"
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadLocalTestDurationHistory(path string) (map[string]int64, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var perf struct {
		TopSlowClasses []LocalTestPerfClass `json:"topSlowClasses"`
	}
	if err := json.Unmarshal(data, &perf); err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, class := range perf.TopSlowClasses {
		if strings.TrimSpace(class.Class) == "" || class.DurationMS <= 0 {
			continue
		}
		out[class.Class] = class.DurationMS
	}
	return out, nil
}
