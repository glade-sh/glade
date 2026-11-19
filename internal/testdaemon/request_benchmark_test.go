package testdaemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// writeDaemonBenchmarkProject creates a deterministic medium project: the
// given number of service classes with companion metadata and one test class.
// Every class body is a few kilobytes so digest validation hashes real bytes.
func writeDaemonBenchmarkProject(b *testing.B, serviceCount int) (root string, rewrite func()) {
	b.Helper()
	root = b.TempDir()
	contents := map[string]string{}
	write := func(rel, body string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			b.Fatal(err)
		}
		contents[path] = body
	}
	write("sfdx-project.json", `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	var body strings.Builder
	for i := range serviceCount {
		name := fmt.Sprintf("BenchService%03d", i)
		body.Reset()
		fmt.Fprintf(&body, "public class %s {\n", name)
		for step := range 12 {
			fmt.Fprintf(&body, "    public static Integer add%d(Integer value) { return value + %d; }\n", step, step)
			fmt.Fprintf(&body, "    public static String label%d() { return %q; }\n", step, fmt.Sprintf("bench service %03d step %02d provides a digest body of realistic size", i, step))
		}
		fmt.Fprintf(&body, "}\n")
		write(filepath.Join("force-app/main/default/classes", name+".cls"), body.String())
		write(filepath.Join("force-app/main/default/classes", name+".cls-meta.xml"),
			`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("force-app/main/default/classes/BenchReadsTest.cls",
		"@IsTest private class BenchReadsTest { @IsTest static void passes() { System.assertEquals(1, BenchService000.add1(0)); } }")
	write("force-app/main/default/classes/BenchReadsTest.cls-meta.xml",
		`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	rewrite = func() {
		for path, content := range contents {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}
	return root, rewrite
}

// BenchmarkDaemonWarmRequestAfterSave measures one warm run request against a
// medium project whose files were just rewritten with identical bytes. That
// is the request shape right after a save: every captured source is racily
// clean, so digest validation cannot shortcut on a trusted old stamp.
func BenchmarkDaemonWarmRequestAfterSave(b *testing.B) {
	root, rewrite := writeDaemonBenchmarkProject(b, 100)
	d, err := New(root)
	if err != nil {
		b.Fatal(err)
	}
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	if _, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		rewrite()
		b.StartTimer()
		if _, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil); err != nil {
			b.Fatal(err)
		}
	}
}
