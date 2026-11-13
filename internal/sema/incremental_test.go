package sema

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func incrementalCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func incrementalWrite(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// incrementalSyntheticProject writes a small clean project with inheritance,
// an interface, inner types, SOQL, DML, a trigger and a test class.
func incrementalSyntheticProject(t *testing.T, root string) {
	t.Helper()
	incrementalWrite(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	const dir = "force-app/main/default/classes/"
	meta := `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`
	add := func(name, body string) {
		incrementalWrite(t, root, dir+name+".cls", body)
		incrementalWrite(t, root, dir+name+".cls-meta.xml", meta)
	}
	add("Shape", `public interface Shape {
    Decimal area();
}
`)
	add("BaseRepo", `public virtual class BaseRepo {
    protected String label = 'base';
    public virtual String describe() {
        String out = label;
        return out;
    }
    public static Integer twice(Integer v) {
        return v * 2;
    }
}
`)
	add("MidRepo", `public virtual class MidRepo extends BaseRepo {
    public override String describe() {
        return 'mid:' + label + BaseRepo.twice(2);
    }
    public Integer size(List<Account> rows) {
        return rows.size();
    }
}
`)
	add("LeafRepo", `public class LeafRepo extends MidRepo {
    public Integer count() {
        List<Account> rows = [SELECT Id, Name FROM Account LIMIT 10];
        return size(rows);
    }
    public override String describe() {
        return super.describe() + '/leaf';
    }
}
`)
	add("Circle", `public class Circle implements Shape {
    private Decimal radius;
    public Circle(Decimal r) {
        radius = r;
    }
    public Decimal area() {
        return 3.14 * radius * radius;
    }
    public class Marker {
        public String name = 'circle';
        public String show() {
            return name;
        }
    }
}
`)
	add("ShapeService", `public with sharing class ShapeService {
    public static Decimal total(List<Shape> shapes) {
        Decimal sum = 0;
        for (Shape s : shapes) {
            sum += s.area();
        }
        return sum;
    }
    public static String label(Circle c) {
        Circle.Marker m = new Circle.Marker();
        return m.show() + c.area();
    }
}
`)
	add("AccountWorker", `public with sharing class AccountWorker {
    public static void rename(List<Account> rows) {
        for (Account a : rows) {
            a.Name = a.Name + '!';
        }
        update rows;
    }
    public static Integer countAll() {
        return [SELECT COUNT() FROM Account];
    }
}
`)
	add("Orchestrator", `public class Orchestrator {
    public String run() {
        LeafRepo repo = new LeafRepo();
        List<Shape> shapes = new List<Shape>{ new Circle(2) };
        return repo.describe() + ShapeService.total(shapes) + AccountWorker.countAll();
    }
}
`)
	add("OrchestratorTest", `@IsTest
private class OrchestratorTest {
    @IsTest
    static void runs() {
        Test.startTest();
        String s = new Orchestrator().run();
        Test.stopTest();
        System.assertNotEquals(null, s);
    }
}
`)
	incrementalWrite(t, root, "force-app/main/default/triggers/AccountTrigger.trigger", `trigger AccountTrigger on Account (before insert) {
    for (Account a : Trigger.new) {
        a.Name = a.Name + '?';
    }
}
`)
	incrementalWrite(t, root, "force-app/main/default/triggers/AccountTrigger.trigger-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexTrigger>`)
}

type incrementalGeneration struct {
	index     typesys.Index
	artifacts typesys.BuildArtifacts
}

func incrementalLoad(t *testing.T, root string) incrementalGeneration {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	return incrementalGeneration{index: index, artifacts: artifacts}
}

func incrementalOptions(artifacts *typesys.BuildArtifacts) AnalyzeOptions {
	return AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true, BuildArtifacts: artifacts}
}

func incrementalClassFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".cls") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// incrementalBodyOpen finds an offset just after the first method-body brace
// that follows a closing parenthesis, or -1.
func incrementalBodyOpen(source string, rng *rand.Rand) int {
	var opens []int
	for i := 1; i < len(source); i++ {
		if source[i] != '{' {
			continue
		}
		j := i - 1
		for j >= 0 && (source[j] == ' ' || source[j] == '\n' || source[j] == '\t' || source[j] == '\r') {
			j--
		}
		if j >= 0 && source[j] == ')' {
			opens = append(opens, i+1)
		}
	}
	if len(opens) == 0 {
		return -1
	}
	return opens[rng.Intn(len(opens))]
}

type incrementalEdit struct {
	name  string
	apply func(source string, rng *rand.Rand) (string, bool)
}

var incrementalEdits = []incrementalEdit{
	{"comment-at-end", func(s string, _ *rand.Rand) (string, bool) { return s + "\n// trailing edit\n", true }},
	{"comment-at-top", func(s string, _ *rand.Rand) (string, bool) { return "// leading edit\n" + s, true }},
	{"blank-lines-top", func(s string, _ *rand.Rand) (string, bool) { return "\n\n\n" + s, true }},
	{"local-in-body", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        Integer zzEdit = 1; zzEdit++;\n" + s[at:], true
	}},
	{"unknown-type-in-body", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        ZzMissingType zzBad = new ZzMissingType();\n" + s[at:], true
	}},
	{"undefined-call-in-body", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        zzNoSuchMethod(1);\n" + s[at:], true
	}},
	{"unknown-field-in-body", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        Account zzAcc = new Account(); zzAcc.ZzNoField__c = 'x';\n" + s[at:], true
	}},
	{"duplicate-local-in-body", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        Integer zzDup = 1; Integer zzDup = 2;\n" + s[at:], true
	}},
	{"call-other-type-in-body", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        Object zzO = BaseRepo.twice(3); ShapeService.total(new List<Shape>());\n" + s[at:], true
	}},
	{"add-method", func(s string, _ *rand.Rand) (string, bool) {
		at := strings.LastIndex(s, "}")
		if at < 0 {
			return s, false
		}
		return s[:at] + "    public static void zzAdded() {}\n" + s[at:], true
	}},
	{"add-field", func(s string, _ *rand.Rand) (string, bool) {
		at := strings.Index(s, "{")
		if at < 0 {
			return s, false
		}
		return s[:at+1] + "\n    public static Integer zzField = 4;" + s[at+1:], true
	}},
	{"break-syntax", func(s string, r *rand.Rand) (string, bool) {
		at := incrementalBodyOpen(s, r)
		if at < 0 {
			return s, false
		}
		return s[:at] + "\n        Integer = ;\n" + s[at:], true
	}},
}

// TestIncrementalAnalysisEqualsFullAnalysis edits one file at a time, in
// random sequences, and requires the incremental result to equal a full
// analysis exactly, at one and at four CPUs.
func TestIncrementalAnalysisEqualsFullAnalysis(t *testing.T) {
	projects := []struct {
		name  string
		build func(t *testing.T, root string)
	}{
		{"synthetic", incrementalSyntheticProject},
	}
	for _, rel := range []string{
		"testdata/local-tests/enterprise-composed",
		"testdata/local-tests/org-like-runner",
		"testdata/local-tests/platform-apis",
		"testdata/local-tests/visualforce-pages",
		"testdata/conformance/person_account_followups",
		"testdata/conformance/case349_trigger_capture",
		"testdata/conformance/type_name_resolution_pins/project",
	} {
		rel := rel
		projects = append(projects, struct {
			name  string
			build func(t *testing.T, root string)
		}{rel, func(t *testing.T, root string) { incrementalCopyTree(t, filepath.Join("..", "..", rel), root) }})
	}
	for _, procs := range []int{1, 4} {
		procs := procs
		t.Run(fmt.Sprintf("gomaxprocs%d", procs), func(t *testing.T) {
			previous := runtime.GOMAXPROCS(procs)
			defer runtime.GOMAXPROCS(previous)
			var edits, incremental, reused int
			for pi, proj := range projects {
				root := t.TempDir()
				proj.build(t, root)
				files := incrementalClassFiles(t, root)
				if len(files) == 0 {
					continue
				}
				original := map[string]string{}
				for _, file := range files {
					data, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					original[file] = string(data)
				}
				rng := rand.New(rand.NewSource(int64(1000*procs + pi)))
				state := NewIncrementalState()
				check := func(label string) IncrementalStats {
					t.Helper()
					generation := incrementalLoad(t, root)
					got, stats := state.Analyze(generation.index, incrementalOptions(&generation.artifacts))
					want := AnalyzeWithOptions(generation.index, incrementalOptions(&generation.artifacts))
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s %s: incremental result differs from full analysis (stats %+v)\n got diagnostics: %v\nwant diagnostics: %v", proj.name, label, stats, got.Diagnostics, want.Diagnostics)
					}
					return stats
				}
				check("baseline")
				rounds := incrementalRounds
				for round := 0; round < rounds; round++ {
					file := files[rng.Intn(len(files))]
					label := fmt.Sprintf("round %d %s", round, filepath.Base(file))
					current, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					source := string(current)
					if rng.Intn(4) == 0 {
						source = original[file]
						label += " revert"
					} else {
						edit := incrementalEdits[rng.Intn(len(incrementalEdits))]
						next, ok := edit.apply(source, rng)
						if !ok {
							continue
						}
						source = next
						label += " " + edit.name
					}
					if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
						t.Fatal(err)
					}
					stats := check(label)
					edits++
					if stats.Incremental {
						incremental++
						reused += stats.Reused
					}
				}
			}
			t.Logf("gomaxprocs=%d edits=%d incremental=%d reusedTypes=%d", procs, edits, incremental, reused)
			if incremental == 0 || reused == 0 {
				t.Fatalf("no edit ran incrementally (edits=%d); the equality check proved nothing", edits)
			}
		})
	}
}

// TestIncrementalAnalysisReuseAndFallback pins which edits reuse cached body
// diagnostics and which fall back to a full analysis.
func TestIncrementalAnalysisReuseAndFallback(t *testing.T) {
	root := t.TempDir()
	incrementalSyntheticProject(t, root)
	state := NewIncrementalState()
	analyze := func() IncrementalStats {
		t.Helper()
		generation := incrementalLoad(t, root)
		got, stats := state.Analyze(generation.index, incrementalOptions(&generation.artifacts))
		if want := AnalyzeWithOptions(generation.index, incrementalOptions(&generation.artifacts)); !reflect.DeepEqual(got, want) {
			t.Fatalf("incremental result differs from full analysis: %v vs %v", got.Diagnostics, want.Diagnostics)
		}
		return stats
	}
	edit := func(file string, change func(string) string) {
		t.Helper()
		path := filepath.Join(root, "force-app/main/default/classes", file)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(change(string(data))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if stats := analyze(); stats.Incremental || stats.Reason == "" {
		t.Fatalf("first analysis stats = %+v, want a full run with a reason", stats)
	}
	if stats := analyze(); !stats.Incremental || stats.Rechecked != 0 || stats.Reused == 0 {
		t.Fatalf("unchanged generation stats = %+v, want every type reused", stats)
	}
	edit("AccountWorker.cls", func(s string) string { return strings.Replace(s, "a.Name + '!'", "a.Name + '!!'", 1) })
	stats := analyze()
	if !stats.Incremental || stats.Rechecked == 0 || stats.Reused == 0 || stats.Rechecked >= stats.Types {
		t.Fatalf("body edit stats = %+v, want a partial recheck", stats)
	}
	edit("AccountWorker.cls", func(s string) string {
		return strings.Replace(s, "public static Integer countAll()", "public static Integer countAny()", 1)
	})
	if stats := analyze(); stats.Incremental || !strings.Contains(stats.Reason, "signature") {
		t.Fatalf("signature edit stats = %+v, want a full run for a signature change", stats)
	}
	edit("Orchestrator.cls", func(s string) string {
		return strings.Replace(s, "AccountWorker.countAll()", "AccountWorker.countAny()", 1)
	})
	if stats := analyze(); !stats.Incremental {
		t.Fatalf("body edit after a signature change stats = %+v, want an incremental run", stats)
	}
	edit("Orchestrator.cls", func(s string) string {
		return strings.Replace(s, "AccountWorker.countAny()", "AccountWorker.countGone()", 1)
	})
	if stats := analyze(); !stats.Incremental {
		t.Fatalf("body edit that introduces an error stats = %+v, want an incremental run", stats)
	}
	edit("Orchestrator.cls", func(s string) string {
		return strings.Replace(s, "AccountWorker.countGone()", "AccountWorker.countAny()", 1)
	})
	if stats := analyze(); !stats.Incremental {
		t.Fatalf("body edit that clears an error stats = %+v, want an incremental run", stats)
	}
}
