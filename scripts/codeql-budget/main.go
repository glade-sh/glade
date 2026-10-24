// Command codeql-budget checks the checked-in waiver limits and a completed
// full-profile run exported with gh run view --json jobs and --log --job ID.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type budget struct {
	FullWaivers    int `json:"full_waivers"`
	PRWaivers      int `json:"pr_waivers"`
	AnalyzeSeconds int `json:"analyze_seconds"`
	QuerySeconds   int `json:"query_seconds"`
}

type step struct{ Name, Conclusion, StartedAt, CompletedAt string }
type runReport struct {
	Jobs []struct {
		Name, Conclusion string
		Steps            []step
	}
}

func main() {
	workflow := flag.String("workflow", ".github/workflows/codeql.yml", "workflow containing inline profiles")
	limits := flag.String("budget", ".github/codeql-budget.json", "recorded release budgets")
	report := flag.String("run-json", "", "gh run view --json jobs output (requires -query-log)")
	log := flag.String("query-log", "", "gh run view --log --job ID output (requires -run-json)")
	flag.Parse()
	if err := check(*workflow, *limits, *report, *log); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(workflowPath, budgetPath, reportPath, logPath string) error {
	body, err := os.ReadFile(budgetPath)
	if err != nil {
		return err
	}
	var limits budget
	if err := json.Unmarshal(body, &limits); err != nil {
		return err
	}
	if limits.FullWaivers <= 0 || limits.PRWaivers <= 0 || limits.AnalyzeSeconds <= 0 || limits.QuerySeconds <= 0 {
		return fmt.Errorf("budgets must be positive")
	}
	body, err = os.ReadFile(workflowPath)
	if err != nil {
		return err
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string
				With struct{ Config string }
			}
		}
	}
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		return err
	}
	counts := make([]int, 2)
	for i, route := range []struct {
		name, label string
		limit       int
	}{
		{"Initialize full-branch CodeQL", "full", limits.FullWaivers},
		{"Initialize pull request CodeQL", "PR", limits.PRWaivers},
	} {
		configs := 0
		for _, s := range workflow.Jobs["codeql"].Steps {
			if s.Name == route.name {
				configs++
				var config struct {
					Filters []map[string]map[string][]string `yaml:"query-filters"`
				}
				if err := yaml.Unmarshal([]byte(s.With.Config), &config); err != nil {
					return err
				}
				ids := map[string]bool{}
				for _, filter := range config.Filters {
					exclude, ok := filter["exclude"]
					if !ok || len(filter) != 1 || len(exclude) != 1 || len(exclude["id"]) == 0 {
						return fmt.Errorf("exclusions must use explicit query IDs")
					}
					for _, id := range exclude["id"] {
						if !strings.HasPrefix(id, "go/") || strings.ContainsAny(id, "*?[]{}") {
							return fmt.Errorf("exclusions must use explicit query IDs")
						}
						ids[id] = true
					}
				}
				counts[i] = len(ids)
			}
		}
		if configs != 1 {
			return fmt.Errorf("expected one %s config", route.name)
		}
		if counts[i] > route.limit {
			return fmt.Errorf("%s waivers %d exceed budget %d", route.label, counts[i], route.limit)
		}
	}
	summary := fmt.Sprintf("full waivers=%d/%d; PR waivers=%d/%d", counts[0], limits.FullWaivers, counts[1], limits.PRWaivers)
	if reportPath == "" && logPath == "" {
		fmt.Println(summary + "; timing not checked (supply -run-json and -query-log for a release)")
		return nil
	}
	if reportPath == "" || logPath == "" {
		return fmt.Errorf("release timing requires both -run-json and -query-log")
	}
	body, err = os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	var report runReport
	if err := json.Unmarshal(body, &report); err != nil {
		return err
	}
	jobs, initializations, analyses := 0, 0, 0
	var analyze step
	for _, job := range report.Jobs {
		if job.Name == "CodeQL" || job.Name == "CodeQL / CodeQL (full)" {
			jobs++
			if job.Conclusion != "success" {
				return fmt.Errorf("CodeQL job did not succeed")
			}
			uploaded, checked := 0, 0
			for _, s := range job.Steps {
				if s.Name == "Upload completed CodeQL results" && s.Conclusion == "success" {
					uploaded++
				}
				if s.Name == "Fail on CodeQL alerts" && s.Conclusion == "success" {
					checked++
				}
				if s.Name == "Initialize full-branch CodeQL" && s.Conclusion == "success" {
					initializations++
				}
				if s.Name == "Initialize pull request CodeQL" && s.Conclusion == "success" {
					return fmt.Errorf("small-PR initialization cannot prove the full profile")
				}
				if s.Name == "Analyze CodeQL" && s.Conclusion == "success" {
					analyses++
					analyze = s
				}
			}
			if job.Name != "CodeQL" && (uploaded != 1 || checked != 1) {
				return fmt.Errorf("full-profile result upload and alert check did not succeed")
			}
		}
	}
	if jobs != 1 {
		return fmt.Errorf("expected one CodeQL job")
	}
	if initializations != 1 {
		return fmt.Errorf("full-profile initialization did not succeed")
	}
	if analyses != 1 {
		return fmt.Errorf("expected one successful Analyze CodeQL step")
	}
	start, e1 := time.Parse(time.RFC3339Nano, analyze.StartedAt)
	end, e2 := time.Parse(time.RFC3339Nano, analyze.CompletedAt)
	if e1 != nil || e2 != nil || !end.After(start) {
		return fmt.Errorf("invalid Analyze CodeQL timestamps")
	}
	elapsed := end.Sub(start).Seconds()
	if elapsed > float64(limits.AnalyzeSeconds) {
		return fmt.Errorf("analyze time %.2fs exceeds budget %ds", elapsed, limits.AnalyzeSeconds)
	}
	body, err = os.ReadFile(logPath)
	if err != nil {
		return err
	}
	queryStart, queryEnd, err := queryInterval(string(body))
	if err != nil {
		return err
	}
	querySeconds := queryEnd.Sub(queryStart).Seconds()
	if querySeconds > float64(limits.QuerySeconds) {
		return fmt.Errorf("query time %.2fs exceeds budget %ds", querySeconds, limits.QuerySeconds)
	}
	if queryStart.Before(start) || queryEnd.After(end) {
		return fmt.Errorf("query log timestamps fall outside Analyze CodeQL")
	}
	fmt.Printf("%s; analyze=%.2f/%ds; queries=%.2f/%ds\n", summary, elapsed, limits.AnalyzeSeconds, querySeconds, limits.QuerySeconds)
	return nil
}

func queryInterval(log string) (time.Time, time.Time, error) {
	timestamp := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z`)
	completion := regexp.MustCompile(`\[(\d+)/(\d+) eval [^]]+\] Evaluation done;`)
	var start, end time.Time
	launches, total := 0, 0
	seen := map[int]bool{}
	for _, line := range strings.Split(log, "\n") {
		launch := strings.Contains(line, "[command]") && strings.Contains(line, "codeql database run-queries ")
		match := completion.FindStringSubmatch(line)
		if !launch && match == nil {
			continue
		}
		stamp, err := time.Parse(time.RFC3339Nano, timestamp.FindString(line))
		if err != nil {
			return start, end, fmt.Errorf("invalid query log timestamp")
		}
		if launch {
			launches++
			start = stamp
			continue
		}
		index, _ := strconv.Atoi(match[1])
		n, _ := strconv.Atoi(match[2])
		if launches != 1 || n <= 0 || index <= 0 || index > n || seen[index] || (total != 0 && total != n) || !stamp.After(start) {
			return start, end, fmt.Errorf("incomplete query evaluation")
		}
		total = n
		seen[index] = true
		if stamp.After(end) {
			end = stamp
		}
	}
	if launches != 1 {
		return start, end, fmt.Errorf("expected one query evaluation")
	}
	if total == 0 || len(seen) != total {
		return start, end, fmt.Errorf("incomplete query evaluation")
	}
	return start, end, nil
}
