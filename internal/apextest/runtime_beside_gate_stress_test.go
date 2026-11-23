package apextest

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// besideGateStressProject is one sfdx project for the stress test. Each
// worker owns its project, so a snapshot mutation in one worker never
// changes another worker's expected outcome.
type besideGateStressProject struct {
	root   string
	helper string
}

func newBesideGateStressProject(t *testing.T, worker int) besideGateStressProject {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	helper := filepath.Join(classes, "StressHelper"+strconv.Itoa(worker)+".cls")
	return besideGateStressProject{root: root, helper: helper}
}

// load writes the helper and test classes for one iteration and returns the
// index. invalid adds a compile error the semantic gate rejects.
func (p besideGateStressProject) load(t *testing.T, worker, iteration int, invalid bool) (typesys.Index, typesys.BuildArtifacts, []TestCase) {
	t.Helper()
	name := "StressHelper" + strconv.Itoa(worker)
	body := "return " + strconv.Itoa(iteration) + ";"
	if invalid {
		body = "Object missing = new MissingStressType" + strconv.Itoa(worker) + "(); return 0;"
	}
	writeFile(t, p.helper, "public class "+name+" { public static Integer value() { "+body+" } }")
	testClass := "StressTest" + strconv.Itoa(worker)
	writeFile(t, filepath.Join(filepath.Dir(p.helper), testClass+".cls"),
		"@IsTest private class "+testClass+" { @IsTest static void runs() { System.assert("+name+".value() >= 0); } }")
	loaded, err := project.Load(p.root)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(loaded, gladeschema.Schema{})
	cases := Discover(index, Options{})
	return index, artifacts, cases
}

// TestRuntimeBesideGateStress runs the beside-gate path from several
// goroutines with randomized gate outcomes: accept (memory and disk cache),
// reject (compile error), gate panic, and abandon (a source snapshot that
// no longer matches). Abandoned builds keep running while later runs and
// cache invalidations start, as they do across sequential tests. Run it
// under -race. GLADE_BESIDE_GATE_STRESS_ITERATIONS raises the iteration
// count for a soak.
func TestRuntimeBesideGateStress(t *testing.T) {
	restoreDisk := EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(WaitForRuntimeCacheWork)
	previous := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	iterations := 4
	if text := os.Getenv("GLADE_BESIDE_GATE_STRESS_ITERATIONS"); text != "" {
		value, err := strconv.Atoi(text)
		if err != nil || value < 1 {
			t.Fatalf("GLADE_BESIDE_GATE_STRESS_ITERATIONS=%q", text)
		}
		iterations = value
	}
	seed := time.Now().UnixNano()
	t.Logf("seed %d", seed)
	const workers = 4
	projects := make([]besideGateStressProject, workers)
	for worker := range projects {
		projects[worker] = newBesideGateStressProject(t, worker)
	}
	var wg sync.WaitGroup
	failures := make(chan string, workers*iterations)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			random := rand.New(rand.NewSource(seed + int64(worker)))
			for iteration := 0; iteration < iterations; iteration++ {
				if problem := besideGateStressIteration(t, projects[worker], worker, iteration, random); problem != "" {
					failures <- fmt.Sprintf("worker %d iteration %d: %s", worker, iteration, problem)
				}
			}
		}(worker)
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

func besideGateStressIteration(t *testing.T, p besideGateStressProject, worker, iteration int, random *rand.Rand) (problem string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if text := toString(recovered); text == "stress gate panic" {
				return
			}
			problem = fmt.Sprintf("unexpected panic: %v", recovered)
		}
	}()
	switch random.Intn(8) {
	case 0:
		InvalidateRuntimeCaches()
	case 1:
		ResetPerfCounters()
	}
	noDisk := random.Intn(2) == 0
	switch mode := random.Intn(4); mode {
	case 0: // accept
		index, artifacts, cases := p.load(t, worker, iteration, false)
		var opts Options
		if random.Intn(2) == 0 {
			opts.BuildArtifacts = &artifacts
		}
		opts.NoDiskCache = noDisk
		run := RunCasesContext(context.Background(), index, opts, cases)
		if summary := run.Summary(); summary.Passed != 1 {
			return fmt.Sprintf("accept summary = %#v problem %q", summary, firstRunProblem(run))
		}
	case 1: // reject
		index, artifacts, cases := p.load(t, worker, iteration, true)
		run := RunCasesContext(context.Background(), index, Options{BuildArtifacts: &artifacts, NoDiskCache: noDisk}, cases)
		if summary := run.Summary(); summary.Passed != 0 {
			return fmt.Sprintf("reject summary = %#v", summary)
		}
	case 2: // gate panic at a random point
		index, artifacts, cases := p.load(t, worker, iteration, false)
		delay := time.Duration(random.Intn(2000)) * time.Microsecond
		// afterIdentity runs on the run's goroutine; afterAnalysis can run on
		// a detached semantic-cache leader, where a panic is not the run's.
		hooks := semanticGateHooks{afterIdentity: func() { time.Sleep(delay); panic("stress gate panic") }}
		runCasesContextWithSemanticGateHooks(context.Background(), index, Options{BuildArtifacts: &artifacts, NoDiskCache: noDisk}, cases, hooks)
		return "gate panic did not propagate"
	case 3: // abandon: the snapshot no longer matches the files
		index, artifacts, cases := p.load(t, worker, iteration, false)
		writeFile(t, p.helper, "public class StressHelper"+strconv.Itoa(worker)+" { public static Integer value() { return -1; } }")
		run := RunCasesContext(context.Background(), index, Options{BuildArtifacts: &artifacts, NoDiskCache: noDisk}, cases)
		if summary := run.Summary(); summary.Passed != 0 {
			return fmt.Sprintf("abandon summary = %#v", summary)
		}
	}
	return ""
}
