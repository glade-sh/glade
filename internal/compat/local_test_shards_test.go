package compat

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestPlanLocalTestClassShardsSeparatesSlowClasses(t *testing.T) {
	cases := []apextest.TestCase{
		{ClassName: "SlowestTest", MethodName: "one"},
		{ClassName: "SecondSlowestTest", MethodName: "one"},
		{ClassName: "SmallTest", MethodName: "one"},
	}
	shards := planLocalTestClassShards(cases, map[string]int64{
		"SlowestTest":       1000,
		"SecondSlowestTest": 900,
		"SmallTest":         10,
	}, 2)
	if len(shards) != 2 {
		t.Fatalf("shards = %#v", shards)
	}
	if reflect.DeepEqual(shards[0].Classes, []string{"SlowestTest", "SecondSlowestTest"}) ||
		reflect.DeepEqual(shards[1].Classes, []string{"SlowestTest", "SecondSlowestTest"}) {
		t.Fatalf("slow classes landed together: %#v", shards)
	}
}

func TestPlanLocalTestClassShardsFallsBackToMethodCount(t *testing.T) {
	cases := []apextest.TestCase{
		{ClassName: "ManyMethodsTest", MethodName: "one"},
		{ClassName: "ManyMethodsTest", MethodName: "two"},
		{ClassName: "ManyMethodsTest", MethodName: "three"},
		{ClassName: "OneMethodTest", MethodName: "one"},
	}
	shards := planLocalTestClassShards(cases, nil, 2)
	if len(shards) != 2 {
		t.Fatalf("shards = %#v", shards)
	}
	if got := shards[0].Classes[0]; got != "ManyMethodsTest" {
		t.Fatalf("first shard starts with %q, want ManyMethodsTest: %#v", got, shards)
	}
	if shards[0].TotalDurationMS <= shards[1].TotalDurationMS {
		t.Fatalf("method-count fallback did not weight the larger class: %#v", shards)
	}
}
