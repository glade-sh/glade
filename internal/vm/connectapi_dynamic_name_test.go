package vm

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Each case starts cold because the captured platform view is process-wide.
func TestConnectAPIDynamicNameCold(t *testing.T) {
	if scenario := os.Getenv("GLADE_CONNECTAPI_DYNAMIC_CASE"); scenario != "" {
		if scenario == "json" || scenario == "jsonStrict" {
			machine := New(nil)
			method := "JSON.deserialize"
			if scenario == "jsonStrict" {
				method = "JSON.deserializeStrict"
			}
			// The native output-type rejection is tracked separately; this checks
			// that either deserializer requests the captured type view.
			_, _ = machine.call(method, []Value{String("{}"), platformScalar("Type", "cOnNeCtApI.FeedBody")}, nil, &Result{})
			if !typesys.ConnectAPIShapeRequested() {
				t.Fatal("deserialization did not load the captured type shape")
			}
			return
		}
		source := "Type t = Type.forName('Connect'+'Api'+'.FeedBody'); Object value = t.newInstance(); String text = String.valueOf(value);"
		if scenario == "namespace" {
			source = "Type t = Type.forName('connect'+'api', 'QuerySqlRow'); Object value = t.newInstance(); String text = String.valueOf(value);"
		}
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		machine := New(nil)
		if scenario != "outside" {
			machine.EnableTestContext()
		}
		result, err := machine.Execute(program)
		if scenario == "outside" {
			if err == nil || !strings.Contains(err.Error(), "ConnectApi.AbstractMessageBody cannot be constructed directly outside of a test") {
				t.Fatalf("outside error = %v", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		want := "isRichText=null"
		if scenario == "namespace" {
			want = "row=null"
		}
		if got := result.Vars["text"].Text; !strings.Contains(got, want) {
			t.Fatalf("captured shape missing: %q", got)
		}
		return
	}
	for _, scenario := range []string{"inside", "outside", "namespace", "json", "jsonStrict"} {
		t.Run(scenario, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestConnectAPIDynamicNameCold$")
			cmd.Env = append(os.Environ(), "GLADE_CONNECTAPI_DYNAMIC_CASE="+scenario)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v\n%s", scenario, err, output)
			}
		})
	}
}
