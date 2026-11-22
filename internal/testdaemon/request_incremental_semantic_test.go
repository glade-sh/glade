package testdaemon

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestRunRequestBodyEditReusesSemanticState(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Warm(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return v + v; } }`)
	if err := d.UpdateChanges([]watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}}); err != nil {
		t.Fatal(err)
	}
	apextest.InvalidateRuntimeCaches()
	request := NewRunRequestV1(apextest.Options{Parallelism: 1}, "", 0, 0, false)
	run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("warm request summary: %+v; %s", got, runProblems(run))
	}
	stats := d.semaState.LastStats()
	if !stats.Incremental || stats.Reused == 0 || stats.Rechecked == 0 || stats.Rechecked >= stats.Types {
		t.Fatalf("request semantic stats = %+v, want a partial recheck", stats)
	}
	request.ReturnClassShards = true
	request.ShardCount = 2
	_, _, plan, err := d.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	apextest.InvalidateRuntimeCaches()
	cold, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, coldPlan, err := cold.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, coldPlan) {
		t.Fatalf("warm and cold plans differ: %+v / %+v", plan, coldPlan)
	}
	request.ReturnClassShards = false
	request.ShardCount = 0
	coldRun, _, _, err := cold.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, report := range []*testreport.Run{&run, &coldRun} {
		report.DurationMS = 0
		for i := range report.Suites {
			report.Suites[i].DurationMS = 0
			for j := range report.Suites[i].Cases {
				report.Suites[i].Cases[j].DurationMS = 0
			}
		}
	}
	if !reflect.DeepEqual(run, coldRun) {
		t.Fatal("warm and cold reports differ beyond durations")
	}
}

// The no-cache option is a cache-independent run, including semantic state.
func TestRunRequestNoCacheBypassesSemanticState(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Warm(); err != nil {
		t.Fatal(err)
	}
	before := d.semaState.LastStats()
	writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return v + v; } }`)
	if err := d.UpdateChanges([]watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}}); err != nil {
		t.Fatal(err)
	}
	apextest.InvalidateRuntimeCaches()
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("no-cache request summary: %+v; %s", got, runProblems(run))
	}
	if after := d.semaState.LastStats(); after != before {
		t.Fatalf("no-cache request changed semantic state: before %+v after %+v", before, after)
	}
}

func TestRunRequestBodyErrorMatchesCold(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := incrementalProject(t)
	path := filepath.Join(root, "force-app/main/default/classes/Util.cls")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Warm(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `public class Util { public static Integer twice(Integer v) { return missing; } }`)
	if err := d.UpdateChanges([]watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Util"}}); err != nil {
		t.Fatal(err)
	}
	apextest.InvalidateRuntimeCaches()
	request := NewRunRequestV1(apextest.Options{Parallelism: 1}, "", 0, 0, false)
	run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Summary(); got.CompileErrors == 0 {
		t.Fatalf("want compile error: %+v", got)
	}
	apextest.InvalidateRuntimeCaches()
	cold, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	coldRun, _, _, err := cold.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, report := range []*testreport.Run{&run, &coldRun} {
		report.DurationMS = 0
		for i := range report.Suites {
			report.Suites[i].DurationMS = 0
			for j := range report.Suites[i].Cases {
				report.Suites[i].Cases[j].DurationMS = 0
			}
		}
	}
	if !reflect.DeepEqual(run, coldRun) {
		t.Fatal("warm and cold error reports differ beyond durations")
	}
}
