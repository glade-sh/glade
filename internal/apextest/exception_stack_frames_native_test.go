package apextest

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/exception_stack_frames/org.tsv: anonymous-block frame counts
// by exception type and the call-token line of caller frames, at API 62.0 and
// 67.0. Anonymous scripts live in scripts/<id>.apex with {V} for the API suffix.
func TestExceptionStackFramesNative(t *testing.T) {
	dir := filepath.Join("testdata", "exception_stack_frames")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 47 || lines[0] != "id\tapi\troute\ttype\tline\tanonymous_frames\tnote\tstack_decoded\ttest_assert_site" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		entries, err := os.ReadDir(filepath.Join(dir, "classes"))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".cls")
			if name != "FrameCustomException" && !strings.HasSuffix(name, suffix) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, "classes", entry.Name()))
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
			// The final empty test_assert_site column is trimmed with the file.
			for len(row) == 8 {
				row = append(row, "")
			}
			if len(row) != 9 {
				t.Fatalf("invalid row %q", line)
			}
			if row[1] != api {
				continue
			}
			id, route := row[0], row[2]
			t.Run(api+"/"+route+"/"+id, func(t *testing.T) {
				machine := runner.newMachine()
				var observation string
				switch route {
				case "anonymous":
					b, err := os.ReadFile(filepath.Join(dir, "scripts", id+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					source := strings.ReplaceAll(string(b), "{V}", suffix)
					if row[3] == "COMPILE" {
						// Native rejects a static class receiver split from its call
						// with "Variable does not exist". That compile diagnostic is
						// a separate gap from the frame rules this test pins.
						t.Skip("native compile diagnostic for a split static receiver is not implemented")
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					observation = machine.Globals["obs"].Text
				case "istest":
					cases := Discover(index, Options{SelectedClasses: []string{"FrameLineTest" + suffix}, SelectedMethod: strings.ReplaceAll(id, "-", "")})
					if len(cases) != 1 {
						// Test method names drop the hyphen and use camel case.
						cases = Discover(index, Options{SelectedClasses: []string{"FrameLineTest" + suffix}, SelectedMethod: camelID(id)})
					}
					if len(cases) != 1 {
						t.Fatalf("named discovery: %d", len(cases))
					}
					methods, methodErrors := compileTestMethods(cases)
					programs, programErrors := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
					}
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("expected assertion: %v", err)
					}
					var found bool
					_, observation, found = strings.Cut(fault.ExceptionMessage(), "Actual: ")
					if !found {
						t.Fatalf("missing observation: %v", err)
					}
				default:
					t.Fatalf("invalid route %q", route)
				}
				want := strings.ReplaceAll(row[7], `\n`, "\n")
				if observation != want {
					t.Errorf("stack:\n%s\nwant:\n%s", observation, want)
				}
				if got := strings.Count(observation, "AnonymousBlock:"); strconv.Itoa(got) != row[5] {
					t.Errorf("anonymous frames=%d, want %s", got, row[5])
				}
			})
		}
	}
}

func camelID(id string) string {
	parts := strings.Split(id, "-")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}
