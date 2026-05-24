package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BenchmarkRunTestSuite(b *testing.B) {
	for _, tests := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("methods=%d/setup=false", tests), func(b *testing.B) {
			benchmarkRunTestSuite(b, tests, false)
		})
		b.Run(fmt.Sprintf("methods=%d/setup=true", tests), func(b *testing.B) {
			benchmarkRunTestSuite(b, tests, true)
		})
	}
}

func BenchmarkRunTestSuiteWithClassSetup(b *testing.B) {
	benchmarkRunTestSuite(b, 100, true)
}

func benchmarkRunTestSuite(b *testing.B, tests int, withSetup bool) {
	root := b.TempDir()
	writeBenchmarkApexTestProject(b, root, tests, withSetup)
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := Run(index, Options{})
		summary := run.Summary()
		if summary.Total != tests || summary.Passed != tests {
			b.Fatalf("summary = %#v", summary)
		}
	}
}

func writeBenchmarkApexTestProject(b *testing.B, root string, tests int, withSetup bool) {
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
		setupBlock := ""
		if withSetup {
			setupBlock = `
  @TestSetup
  static void buildFixture() {
    insert new Account(Name = 'Fixture');
  }
`
		}
		writeBenchmarkTestFile(b, filepath.Join(root, fmt.Sprintf("force-app/main/classes/MathUtil%03dTest.cls", i)), fmt.Sprintf(`
@isTest
private class MathUtil%03dTest {
%s
  @isTest static void adds() {
    System.assertEquals(%d, MathUtil.add(%d, %d));
  }
}
`, i, setupBlock, i+i+1, i, i+1))
	}
}

func benchmarkLoadTestIndex(b *testing.B, root string) typesys.Index {
	b.Helper()
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
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
