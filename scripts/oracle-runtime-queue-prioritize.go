//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type runtimePath struct {
	Target string `json:"target"`
	Count  int    `json:"count"`
}

type runtimeInventory struct {
	RuntimePaths []runtimePath `json:"runtimePaths"`
}

type workItem struct {
	ID             string   `json:"id"`
	ProbeID        string   `json:"probeId"`
	SurfaceID      string   `json:"surfaceId"`
	Area           string   `json:"area"`
	Shard          int      `json:"shard"`
	GeneratedClass string   `json:"generatedClass"`
	MethodName     string   `json:"methodName"`
	Status         string   `json:"status"`
	Attempts       int      `json:"attempts"`
	Artifacts      []string `json:"artifacts,omitempty"`
}

type workQueue struct {
	SchemaVersion int        `json:"schemaVersion"`
	Target        string     `json:"target"`
	GeneratedAt   time.Time  `json:"generatedAt"`
	Area          string     `json:"area,omitempty"`
	Items         []workItem `json:"items"`
}

type scoredItem struct {
	item      workItem
	score     int
	matchedBy string
	original  int
}

type matchStat struct {
	ID        string `json:"id"`
	SurfaceID string `json:"surfaceId"`
	Score     int    `json:"score"`
	MatchedBy string `json:"matchedBy,omitempty"`
}

type report struct {
	RuntimeInventory string      `json:"runtimeInventory"`
	InputWorkQueue   string      `json:"inputWorkQueue"`
	OutputWorkQueue  string      `json:"outputWorkQueue"`
	TotalItems       int         `json:"totalItems"`
	MatchedItems     int         `json:"matchedItems"`
	UnmatchedItems   int         `json:"unmatchedItems"`
	DistinctTargets  int         `json:"distinctTargets"`
	TopMatches       []matchStat `json:"topMatches"`
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func candidateTokens(surfaceID string) []string {
	s := strings.TrimSpace(surfaceID)
	if s == "" {
		return nil
	}
	out := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(v string) {
		v = normalize(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	add(s)
	noParams := s
	if idx := strings.Index(noParams, "("); idx >= 0 {
		noParams = noParams[:idx]
	}
	add(noParams)

	if idx := strings.LastIndex(noParams, "::"); idx >= 0 {
		add(noParams[idx+2:])
	}

	parts := strings.Split(noParams, ".")
	if len(parts) > 0 {
		add(parts[len(parts)-1])
	}
	if len(parts) > 1 {
		add(parts[len(parts)-2])
	}
	if len(parts) > 2 {
		add(parts[len(parts)-3])
	}
	return out
}

func readJSON[T any](path string, out *T) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(strings.TrimSuffix(path, "/"+filepathBase(path)), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func filepathBase(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

func main() {
	runtimePath := flag.String("runtime-inventory", ".glade/runtime-path-inventory-with-stubs.json", "runtime inventory JSON")
	queuePath := flag.String("work-queue", "docs/generated/apex-oracle/WORK_QUEUE.json", "input work queue")
	outPath := flag.String("output", "docs/generated/apex-oracle/WORK_QUEUE.runtime-ranked.json", "output work queue")
	reportPath := flag.String("report", "docs/generated/apex-oracle/RUNTIME_QUEUE_REPORT.json", "output report")
	flag.Parse()

	inv := runtimeInventory{}
	if err := readJSON(*runtimePath, &inv); err != nil {
		fmt.Fprintf(os.Stderr, "read runtime inventory: %v\n", err)
		os.Exit(1)
	}
	queue := workQueue{}
	if err := readJSON(*queuePath, &queue); err != nil {
		fmt.Fprintf(os.Stderr, "read work queue: %v\n", err)
		os.Exit(1)
	}

	targetCounts := map[string]int{}
	for _, rp := range inv.RuntimePaths {
		t := normalize(rp.Target)
		if t == "" || rp.Count <= 0 {
			continue
		}
		targetCounts[t] += rp.Count
	}

	scored := make([]scoredItem, 0, len(queue.Items))
	for i, item := range queue.Items {
		bestScore := 0
		best := ""
		for _, c := range candidateTokens(item.SurfaceID) {
			if n := targetCounts[c]; n > bestScore {
				bestScore = n
				best = c
			}
		}
		scored = append(scored, scoredItem{item: item, score: bestScore, matchedBy: best, original: i})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].original < scored[j].original
	})

	ranked := queue
	ranked.Items = make([]workItem, 0, len(scored))
	matched := 0
	top := make([]matchStat, 0, 25)
	for i, s := range scored {
		ranked.Items = append(ranked.Items, s.item)
		if s.score > 0 {
			matched++
		}
		if i < 25 {
			top = append(top, matchStat{ID: s.item.ID, SurfaceID: s.item.SurfaceID, Score: s.score, MatchedBy: s.matchedBy})
		}
	}

	if err := writeJSON(*outPath, ranked); err != nil {
		fmt.Fprintf(os.Stderr, "write output queue: %v\n", err)
		os.Exit(1)
	}

	rep := report{
		RuntimeInventory: *runtimePath,
		InputWorkQueue:   *queuePath,
		OutputWorkQueue:  *outPath,
		TotalItems:       len(ranked.Items),
		MatchedItems:     matched,
		UnmatchedItems:   len(ranked.Items) - matched,
		DistinctTargets:  len(targetCounts),
		TopMatches:       top,
	}
	if err := writeJSON(*reportPath, rep); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ranked queue: %s\n", *outPath)
	fmt.Printf("match report: %s\n", *reportPath)
	fmt.Printf("items=%d matched=%d unmatched=%d distinctTargets=%d\n", rep.TotalItems, rep.MatchedItems, rep.UnmatchedItems, rep.DistinctTargets)
}
