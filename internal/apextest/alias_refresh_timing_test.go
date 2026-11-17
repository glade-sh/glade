package apextest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Trigger-heavy shape for the alias refresh cost: a large caller list and many
// nested lists live while each DML statement runs triggers whose handler
// mutates a static list. GLADE_ALIAS_REFRESH_PERF=1 prints the best of 3 runs.
func TestAliasRefreshTriggerHeavyTiming(t *testing.T) {
	if os.Getenv("GLADE_ALIAS_REFRESH_PERF") == "" {
		t.Skip("set GLADE_ALIAS_REFRESH_PERF=1 to time the shape")
	}
	api := "62.0"
	root := gapNeighborProject(t, api, map[string]string{
		"classes/ArH.cls":                 `public class ArH { public static List<String> log = new List<String>(); public static void note(String s) { log.add(s); } }`,
		"classes/ArH.cls-meta.xml":        gapClassMeta(api),
		"triggers/ArTrg.trigger":          `trigger ArTrg on Account (before insert, after insert, before update, after update) { ArH.note(String.valueOf(Trigger.new.size())); }`,
		"triggers/ArTrg.trigger-meta.xml": gapTriggerMeta(api),
		"classes/ArTest.cls": `@IsTest private class ArTest { @IsTest static void heavy() {
  List<Integer> big = new List<Integer>(); for (Integer i = 0; i < 20000; i++) { big.add(i); }
  Map<String, List<String>> nested = new Map<String, List<String>>(); for (Integer i = 0; i < 3000; i++) { nested.put('k' + i, new List<String>{'a'}); }
  for (Integer i = 0; i < 150; i++) { Account a = new Account(Name = 'a' + i); insert a; a.Name = 'b' + i; update a; }
  System.assertEquals(600, ArH.log.size());
} }`,
		"classes/ArTest.cls-meta.xml": gapClassMeta(api),
	})
	index := loadTestIndex(t, root)
	best := time.Hour
	for i := 0; i < 3; i++ {
		start := time.Now()
		run := Run(index, Options{SelectedClasses: []string{"ArTest"}, NoDiskCache: true, Parallelism: 1})
		if summary := run.Summary(); summary.Passed != 1 {
			t.Fatalf("summary = %+v", summary)
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	t.Logf("best=%dms", best.Milliseconds())
	_ = filepath.Join
}
