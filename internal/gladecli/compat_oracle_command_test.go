package gladecli

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/oracle"
)

func TestOracleLocalFiltersForQueueUsesExactClassMethod(t *testing.T) {
	queue := oracle.WorkQueue{
		Items: []oracle.WorkItem{
			{GeneratedClass: "GLADE_Oracle_stdlib_misc_1113", MethodName: "probe_1113"},
			{GeneratedClass: "GLADE_Oracle_stdlib_misc_11130", MethodName: "probe_11130"},
			{GeneratedClass: "GLADE_Oracle_stdlib_misc_11131"},
		},
	}

	got := oracleLocalFiltersForQueue(queue)
	want := []string{
		"GLADE_Oracle_stdlib_misc_1113.probe_1113",
		"GLADE_Oracle_stdlib_misc_11130.probe_11130",
		"GLADE_Oracle_stdlib_misc_11131",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filters = %#v", got)
	}
}
