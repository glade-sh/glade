package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/startupcache"
)

// A FIFO makes an otherwise invisible status read observable without relying
// on timing or allocation budgets. --no-cache disables execution cache reads.
func TestRedirectedTestSkipsStartupCacheStatus(t *testing.T) {
	if root := os.Getenv("GLADE_TEST_CACHE_FIFO_ROOT"); root != "" {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"test", "--project", root, "--filter", "ExampleTest", "--no-serve", "--no-cache"}, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
		}
		return
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls"), `@IsTest private class ExampleTest { @IsTest static void passes() { System.assert(true); } }`)
	cacheDir := filepath.Join(root, ".glade", "test")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(cacheDir, "startup.meta.json"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRedirectedTestSkipsStartupCacheStatus$")
	cmd.Env = append(os.Environ(), "GLADE_TEST_CACHE_FIFO_ROOT="+root)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("redirected progress blocked reading the startup cache for a discarded warning")
	}
	if err != nil {
		t.Fatalf("child: %v: %s", err, output)
	}
}

func TestVisibleStartupCacheWarningUnchanged(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls"), `@IsTest private class ExampleTest { @IsTest static void passes() { System.assert(true); } }`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--filter", "ExampleTest", "--no-serve", "--no-cache", "--progress-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	const want = "startup cache: missing; next full run will build .glade/test/startup.meta.json"
	for _, event := range decodeSetupProgress(t, stderr.String()) {
		if event.Kind == "warn" && event.Label == want {
			return
		}
	}
	t.Fatalf("missing exact warning %q in %s", want, stderr.String())
}

// The status optimization must not bypass execution cache validation.
func TestRedirectedTestValidatesExecutionCache(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	defer restore()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Example.cls"), `public class Example { public static Boolean value() { return true; } }`)
	class := filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls")
	writeTestFile(t, class, `@IsTest private class ExampleTest { @IsTest static void passes() { System.assert(true); } }`)
	run := func() (int, string, string) {
		var out, errs bytes.Buffer
		code := Run(context.Background(), []string{"test", "--project", root, "--no-serve", "--parallelism", "1"}, &out, &errs)
		return code, out.String(), errs.String()
	}
	code, out, errs := run()
	if code != 0 || errs != "" || !strings.Contains(out, "1 passed, 0 failed") {
		t.Fatalf("initial exit=%d stdout=%q stderr=%q", code, out, errs)
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || entry == nil || len(entry.Runtime.Methods) == 0 || len(entry.Runtime.Classes) == 0 {
		t.Fatalf("missing populated runtime cache: entry present=%t err=%v", entry != nil, err)
	}
	cacheFiles := func() map[string][]byte {
		files := make(map[string][]byte)
		paths, err := filepath.Glob(filepath.Join(root, startupcache.SubdirTest, "startup.*"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files[filepath.Base(path)] = data
		}
		return files
	}
	before := cacheFiles()
	if len(before) < 2 {
		t.Fatalf("missing cache files: %d", len(before))
	}
	code, out, errs = run()
	if code != 0 || errs != "" || !strings.Contains(out, "1 passed, 0 failed") {
		t.Fatalf("warm exit=%d stdout=%q stderr=%q", code, out, errs)
	}
	if after := cacheFiles(); !reflect.DeepEqual(before, after) {
		t.Fatal("warm execution changed startup cache bytes")
	}
	writeTestFile(t, class, `@IsTest private class ExampleTest { @IsTest static void passes() { System.assert(false, 'changed'); } }`)
	code, out, errs = run()
	if code == 0 || !strings.Contains(out+errs, "changed") {
		t.Fatalf("stale cache hid changed source: exit=%d stdout=%q stderr=%q", code, out, errs)
	}
}

func BenchmarkRedirectedTestWithStartupCache(b *testing.B) {
	root := b.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0700); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0600); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1500; i++ {
		name := fmt.Sprintf("Synthetic%04d", i)
		source := fmt.Sprintf("public class %s { public static Integer value() { return %d; } }", name, i)
		if err := os.WriteFile(filepath.Join(classes, name+".cls"), []byte(source), 0600); err != nil {
			b.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(classes, "ExampleTest.cls"), []byte(`@IsTest private class ExampleTest { @IsTest static void passes() { System.assert(true); } }`), 0600); err != nil {
		b.Fatal(err)
	}
	restore := apextest.EnableDiskCacheForTesting()
	defer restore()
	args := []string{"test", "--project", root, "--no-serve", "--parallelism", "1"}
	var warmOut, warmErr bytes.Buffer
	if code := Run(context.Background(), args, &warmOut, &warmErr); code != 0 || warmErr.Len() != 0 || !strings.Contains(warmOut.String(), "1 passed, 0 failed") {
		b.Fatalf("warmup exit=%d stderr=%q stdout=%q", code, warmErr.String(), warmOut.String())
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || entry == nil || len(entry.Runtime.Classes) < 1500 || len(entry.Runtime.Methods) < 1500 {
		b.Fatalf("expected populated execution cache: entry present=%t err=%v", entry != nil, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out, errs bytes.Buffer
		code := Run(context.Background(), args, &out, &errs)
		if code != 0 || errs.Len() != 0 || !strings.Contains(out.String(), "1 passed, 0 failed") {
			b.Fatalf("exit=%d stderr=%q stdout=%q", code, errs.String(), out.String())
		}
	}
}
