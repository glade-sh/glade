package apextest

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/exception_anonymous_frames/org.tsv rows 2–15.
func TestExceptionAnonymousFramesNative(t *testing.T) {
	dir := filepath.Join("testdata", "exception_anonymous_frames")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 15 || lines[0] != "api\troute\tcase\ttype\tmessage64\tcauseType\tcauseMessage64\tline\tstack64" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		for _, prefix := range []string{"TraceLeaf", "TraceProbe"} {
			name := prefix + suffix
			b, err := os.ReadFile(filepath.Join(dir, "classes", name+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			classes[name] = string(b)
		}
		index := nativeBoundaryIndex(t, api, classes, nil)
		if index.HasErrors() {
			t.Fatalf("index: %v", index.Diagnostics)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, line := range lines[1:] {
			row := strings.Split(line, "\t")
			if len(row) != 9 {
				t.Fatalf("invalid row %q", line)
			}
			if row[0] != api {
				continue
			}
			if row[1] != "anonymous" {
				t.Fatalf("invalid route %q", row[1])
			}
			t.Run(api+"/"+row[2], func(t *testing.T) {
				machine := runner.newMachine()
				source := fmt.Sprintf("String observed=TraceProbe%s.capture('%s');", suffix, row[2])
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				parts := strings.Split(machine.Globals["observed"].Text, "|")
				got := map[string]string{}
				for _, part := range parts {
					k, v, ok := strings.Cut(part, ":")
					if ok {
						got[k] = v
					}
				}
				for _, tc := range []struct{ key, want string }{{"type", row[3]}, {"message", row[4]}, {"causeType", row[5]}, {"causeMessage", row[6]}, {"line", row[7]}, {"stack", row[8]}} {
					value := got[tc.key]
					if tc.key == "message" || tc.key == "causeMessage" || tc.key == "stack" {
						if value != "N" {
							value = strings.TrimPrefix(value, "B:")
						}
						if tc.want != "N" {
							decoded, err := base64.StdEncoding.DecodeString(value)
							if err != nil {
								t.Fatalf("%s base64: %v", tc.key, err)
							}
							value = string(decoded)
							b, err := base64.StdEncoding.DecodeString(tc.want)
							if err != nil {
								t.Fatal(err)
							}
							tc.want = string(b)
						}
					}
					if value != tc.want {
						t.Errorf("%s=%q, want %q", tc.key, value, tc.want)
					}
				}
			})
		}
	}
}
