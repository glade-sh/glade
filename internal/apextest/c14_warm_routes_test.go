package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testreport"
)

// This compares the selected project-runner outcome before and after runtime
// cache reuse. Native matching and explicit carries remain in each family test.
func TestC14WarmProjectRoutes(t *testing.T) {
	restore := EnableDiskCacheForTesting()
	defer restore()
	base := filepath.Join("testdata", "conformance")
	families, err := filepath.Glob(filepath.Join(base, "*_c14", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	families = append(families, filepath.Join(base, "scheduled_limit_c09", "org.tsv"), filepath.Join(base, "batch_limit_c09", "org.tsv"))
	for _, path := range families {
		name := filepath.Base(filepath.Dir(path))
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		reader.LazyQuotes = true
		rows, err := reader.ReadAll()
		file.Close()
		if err != nil || len(rows) < 2 {
			t.Fatalf("%s rows: %v", name, err)
		}
		if name != "stdlib_core_c14" && !strings.HasSuffix(name, "_c09") && (len(rows[0]) != 5 || rows[0][1] != "class") {
			// Row-shaped families (anonymous probes, per-expression steps) have
			// no project-runner route to warm; their family tests cover them.
			continue
		}
		for _, row := range rows[1:] {
			row := row
			api, class, method, capture := "", "", "", ""
			project := filepath.Join(filepath.Dir(path), "project")
			switch name {
			case "scheduled_limit_c09":
				api, method = row[0], row[1]
				code := strings.TrimSuffix(api, ".0")
				class = "DootGT" + code + "C6C09"
				capture = "C09_" + code + "_6_0"
				project = filepath.Join(filepath.Dir(path), "api"+code)
			case "batch_limit_c09":
				api, method = row[0], row[2]
				code := strings.TrimSuffix(api, ".0")
				phase := strings.TrimPrefix(row[1], "Batch.")
				phaseCode := map[string]string{"start": "3", "execute": "4", "finish": "5"}[phase]
				if phaseCode == "" {
					t.Fatal("unknown batch phase")
				}
				class = "DootGT" + code + "C" + phaseCode + "C09"
				capture = "C09_" + code + "_" + phaseCode + "_0"
				project = filepath.Join(filepath.Dir(path), code+"-"+phase)
			case "stdlib_core_c14":
				api, class, method, capture = row[0], "DootStdlibFollow67A1031", row[1], row[2]
			default:
				if len(row) != 5 {
					t.Fatalf("%s malformed row", name)
				}
				api, class, method, capture = row[0], row[1], row[2], row[3]
			}
			t.Run(name+"/"+api+"/"+class+"/"+method, func(t *testing.T) {
				root := t.TempDir()
				if name == "scheduled_limit_c09" || name == "batch_limit_c09" {
					entries, err := os.ReadDir(project)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range entries {
						body, err := os.ReadFile(filepath.Join(project, entry.Name()))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", entry.Name()), string(body))
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
				} else {
					err := filepath.WalkDir(project, func(p string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						if name == "stdlib_locale_crypto_c14" && strings.Contains(p, string(filepath.Separator)+"classes"+string(filepath.Separator)) && filepath.Base(p) != class+".cls" && filepath.Base(p) != class+".cls-meta.xml" {
							return nil
						}
						body, err := os.ReadFile(p)
						if err != nil {
							return err
						}
						rel, err := filepath.Rel(project, p)
						if err != nil {
							return err
						}
						if strings.HasSuffix(p, ".xml") || strings.HasSuffix(p, "sfdx-project.json") {
							body = []byte(strings.ReplaceAll(string(body), "62.0", api))
						}
						writeFile(t, filepath.Join(root, rel), string(body))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				if name == "soql_clock_held_c14" {
					var want map[string]any
					if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
						t.Fatal(err)
					}
					clock := "2026-10-31T12:00:00Z"
					if method == "futureToday" {
						before, err := time.Parse(time.RFC3339Nano, want["before"].(string))
						if err != nil {
							t.Fatal(err)
						}
						clock = before.Add(-time.Minute).Format(time.RFC3339Nano)
					}
					if method == "formatFunction" {
						zone, err := time.LoadLocation("America/Los_Angeles")
						if err != nil {
							t.Fatal(err)
						}
						entries := want["rows"].([]any)
						first := entries[0].(map[string]any)
						instant, err := time.ParseInLocation("1/2/2006, 3:04 PM", first["CreatedDate"].(string), zone)
						if err != nil {
							t.Fatal(err)
						}
						clock = instant.UTC().Format(time.RFC3339Nano)
					}
					writeFile(t, filepath.Join(root, "glade.yml"), fmt.Sprintf("test:\n  clock:\n    start: '%s'\n  user:\n    timeZone: America/Los_Angeles\n    locale: en_US\n    language: en_US\n", clock))
				}
				index := loadTestIndex(t, root)
				InvalidateRuntimeCaches()
				opts := Options{SelectedClasses: []string{class}, SelectedMethod: method, Parallelism: 1, PerfCounters: true}
				cold := Run(index, opts)
				ResetPerfCounters()
				warm := Run(index, opts)
				if len(cold.Suites) != 1 || len(cold.Suites[0].Cases) != 1 || len(warm.Suites) != 1 || len(warm.Suites[0].Cases) != 1 {
					t.Fatalf("selected route missing: cold=%+v warm=%+v", cold.Summary(), warm.Summary())
				}
				c, w := cold.Suites[0].Cases[0], warm.Suites[0].Cases[0]
				if c.Status != w.Status || strings.EqualFold(string(c.Status), "skipped") {
					t.Fatalf("cold/warm statuses: %q/%q", c.Status, w.Status)
				}
				if len(cold.ExcludedClasses) != 0 || len(warm.ExcludedClasses) != 0 {
					t.Fatalf("unexpected compile exclusions: cold=%+v warm=%+v", cold.ExcludedClasses, warm.ExcludedClasses)
				}
				if SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
					t.Fatal("second project run did not hit the runtime memory cache")
				}
				if capture == "UNCAUGHT" {
					// Native rejection: no capture marker; cold and warm reject alike.
					if c.Problem == nil || w.Problem == nil || c.Problem.Type != w.Problem.Type || c.Problem.Message != w.Problem.Message {
						t.Fatalf("cold/warm rejections differ: cold=%+v warm=%+v", c.Problem, w.Problem)
					}
					return
				}
				cv, err := c14WarmProblem(c.Problem, capture)
				if err != nil {
					t.Fatal(err)
				}
				wv, err := c14WarmProblem(w.Problem, capture)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cv, wv) {
					t.Fatalf("cold/warm outcomes differ: cold=%v warm=%v", cv, wv)
				}
			})
		}
	}
}

func c14WarmProblem(problem *testreport.Problem, capture string) (any, error) {
	if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
		return nil, fmt.Errorf("missing capture assertion")
	}
	payload, err := parseCaptureObservation(problem.Message, capture)
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": problem.Type, "payload": normalizeCaptureIdentities(payload)}, nil
}
