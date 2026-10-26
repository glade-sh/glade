//go:build darwin || linux

package apextest

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/vm"
)

func TestIPv4ConstructorProjectAnonymousAndIsTestAfterSuspension(t *testing.T) {
	t.Logf("GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	const childFlag = "GLADE_IPV4_PROJECT_CLOCK_CHILD"
	if os.Getenv(childFlag) != "1" {
		ipv4ProjectSuspendedChild(t, "^TestIPv4ConstructorProjectAnonymousAndIsTestAfterSuspension$", childFlag)
		return
	}
	type execution struct {
		label, class string
		machine      *vm.VM
		program      ir.Program
	}
	var executions []execution
	body := `for (Integer sample=0;sample<200;sample++) {
 String address=String.join(new List<String>{'0','0','0','0'},'.');
 System.assertEquals(0L,IPv4BoundaryProbe.create(address).numberValue());
 }`
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/ipv4_validation/project")); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":"netprobe","sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
		classes := filepath.Join(root, "force-app/main/default/classes")
		writeFile(t, filepath.Join(classes, "DelimiterConstructorRouteTest.cls"), `@IsTest private class DelimiterConstructorRouteTest { @IsTest static void verify() {`+body+`} }`)
		writeFile(t, filepath.Join(classes, "DelimiterConstructorRouteTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
		for _, name := range []string{"IPv4BoundaryProbe", "IPv4BoundaryProbeTest"} {
			path := filepath.Join(classes, name+".cls-meta.xml")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, strings.ReplaceAll(string(raw), "62.0", api))
		}
		index := loadTestIndex(t, root)
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		anonymous, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		machine := runner.newMachine()
		machine.SetCurrentNamespace("netprobe")
		executions = append(executions, execution{label: api + "/anonymous/inner-constructor-interface", machine: machine, program: anonymous})
		cases := Discover(index, Options{SelectedClasses: []string{"DelimiterConstructorRouteTest"}, SelectedMethod: "verify"})
		if len(cases) != 1 {
			t.Fatalf("test discovery=%d", len(cases))
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		invokes, invokeErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || invokeErrors[key] != nil {
			t.Fatal(methodErrors[key], invokeErrors[key])
		}
		machine = runner.newMachine()
		machine.SetCurrentNamespace("netprobe")
		machine.EnableTestContext()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		executions = append(executions, execution{label: api + "/isTest/inner-constructor-interface", class: cases[0].ClassName, machine: machine, program: invokes[key]})
	}
	clock := regexp2.MustCompile(`\.`, 0)
	clock.MatchTimeout = 2 * time.Second
	for cycle := 0; cycle < 12; cycle++ {
		if _, err := clock.FindStringMatch("0.0.0.0"); err != nil {
			t.Fatal(err)
		}
		fmt.Println("IPV4_PROJECT_PAUSE")
		if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(150 * time.Millisecond)
		for time.Now().Before(until) {
			for _, run := range executions {
				var err error
				if run.class == "" {
					_, err = run.machine.Execute(run.program)
				} else {
					_, err = run.machine.ExecuteTestInClass(run.program, run.class)
				}
				if err != nil {
					t.Fatalf("cycle=%d route=%s: %v", cycle, run.label, err)
				}
			}
		}
	}
}

func ipv4ProjectSuspendedChild(t *testing.T, selection, flag string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run="+selection, "-test.v", "-test.timeout=100s")
	cmd.Env = append(os.Environ(), flag+"=1", "GOMAXPROCS=2")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	scanner := bufio.NewScanner(stdout)
	var output strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line + "\n")
		if line == "IPV4_PROJECT_PAUSE" {
			time.Sleep(2400 * time.Millisecond)
			if err := cmd.Process.Signal(syscall.SIGCONT); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("suspended project child: %v\n%s", err, output.String())
	}
}
