package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
)

func BenchmarkRunTestSuite(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkApexTestProject(b, root, 20)
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := Run(index, Options{})
		summary := run.Summary()
		if summary.Total != 20 || summary.Passed != 20 {
			b.Fatalf("summary = %#v", summary)
		}
	}
}

func writeBenchmarkApexTestProject(b *testing.B, root string, tests int) {
	b.Helper()
	writeBenchmarkTestFile(b, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeBenchmarkTestFile(b, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	for i := 0; i < tests; i++ {
		writeBenchmarkTestFile(b, filepath.Join(root, fmt.Sprintf("force-app/main/classes/MathUtil%03dTest.cls", i)), fmt.Sprintf(`
@isTest
private class MathUtil%03dTest {
  @isTest static void adds() {
    System.assertEquals(%d, MathUtil.add(%d, %d));
  }
}
`, i, i+i+1, i, i+1))
	}
}

func benchmarkLoadTestIndex(b *testing.B, root string) typesys.Index {
	b.Helper()
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	return typesys.Build(p, s)
}

func writeBenchmarkTestFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
