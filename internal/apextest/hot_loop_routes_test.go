package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned scanner, XML document, and pixel-buffer shapes pin the runtime routes.
// These preserve baseline behavior; they do not assert new native semantics.
func TestHotLoopSyntheticProjectRoutes(t *testing.T) {
	const body = `String text='abcd'.repeat(256); Integer chars=0; for(Integer i=0;i<100;i++){chars+=text.mid(i,1).length()+text.left(1).length()+text.right(1).length();} System.assertEquals(300,chars);
 Dom.Document doc=new Dom.Document(); doc.load('<root>'+('<item key="v">abc</item>').repeat(100)+'</root>'); System.assertEquals(100,doc.getRootElement().getChildElements().size()); System.assertEquals('abc',doc.getRootElement().getChildElements().get(99).getText());
 XmlStreamReader reader=new XmlStreamReader('<root>Ω<x/>tail</root>'); Integer events=0; String locations=''; while(reader.hasNext()){reader.next();events++;locations+=reader.getLocation()+';';} System.assertEquals(7,events); System.assertEquals('Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;',locations);
 List<Integer> pixels=new List<Integer>{1,2,3,4}; for(Integer i=0;i<4000;i++){Integer at=i&3; pixels.set(at,(pixels.get(at)+i)&255);} System.assertEquals(177,pixels.get(0)); System.assertEquals(154,pixels.get(1)); System.assertEquals(131,pixels.get(2)); System.assertEquals(108,pixels.get(3));`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{"LoopShapes": `public class LoopShapes {public static void run(){` + body + `}}`, "LoopShapesTest": `@IsTest private class LoopShapesTest {@IsTest static void run(){LoopShapes.run();}}`}, nil)
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			for _, route := range []struct{ name, source string }{{"anonymous", body}, {"named", "LoopShapes.run();"}} {
				t.Run(route.name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(route.source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					if _, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program); err != nil {
						t.Fatal(err)
					}
					t.Log("ROW " + api + "/" + route.name + " scanner=300 children=100 pixels=177,154,131,108")
				})
			}
			for _, tracing := range []bool{false, true} {
				route := "isTest_runner_untraced"
				if tracing {
					route = "isTest_runner_traced"
				}
				t.Run(route, func(t *testing.T) {
					report := Run(index, Options{SelectedClasses: []string{"LoopShapesTest"}, NoDiskCache: true, Parallelism: 1, TraceAll: tracing})
					if report.Summary().Failed != 0 || report.Summary().Passed != 1 {
						t.Fatalf("%+v", report.Summary())
					}
					t.Log("ROW " + api + "/" + route + " scanner=300 children=100 pixels=177,154,131,108")
				})
			}
		})
	}
}

// These cache routes preserve the same synthetic outcomes as the uncached runner.
func TestHotLoopWarmProjectCacheRoutes(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "hot_loop_routes", "LoopShapes.cls"))
	if err != nil {
		t.Fatal(err)
	}
	restore := EnableDiskCacheForTesting()
	defer restore()
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"LoopShapes": string(source),
				"LoopShapesTest": `@IsTest private class LoopShapesTest {
     @IsTest static void first(){System.assertEquals('300:100:7:177,154,131,108:Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;',LoopShapes.run());}
     @IsTest static void second(){System.assertEquals('300:100:7:177,154,131,108:Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;',LoopShapes.run());}
    }`,
			}, nil)
			InvalidateRuntimeCaches()
			for _, route := range []string{"cold_project_cache", "warm_memory_parallel", "warm_disk_parallel"} {
				t.Run(route, func(t *testing.T) {
					if route == "warm_disk_parallel" {
						InvalidateRuntimeCaches()
					}
					report := Run(index, Options{SelectedClasses: []string{"LoopShapesTest"}, Parallelism: 2, ParallelMethods: true, RestoredRuntimeMultiWorker: true, PerfCounters: true})
					if summary := report.Summary(); summary.Total != 2 || summary.Passed != 2 {
						t.Fatalf("%+v", report)
					}
					phases := SnapshotPerfCounters().Phases
					switch route {
					case "cold_project_cache":
						if phases.CacheMisses != 1 {
							t.Fatalf("cold cache: %+v", phases)
						}
					case "warm_memory_parallel":
						if phases.MemoryCacheHits != 1 {
							t.Fatalf("memory cache not exercised: %+v", phases)
						}
					case "warm_disk_parallel":
						if phases.DiskCacheHits != 1 {
							t.Fatalf("disk cache not exercised: %+v", phases)
						}
					}
					t.Log("ROW " + api + "/" + route + " outcome=300:100:7:177,154,131,108:Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;; methods=2")
				})
			}
		})
	}
}
