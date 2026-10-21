package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestLazyCollectionPropertyAcrossSourceUnits(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, units := range []int{1, 2} {
			for _, access := range []struct{ name, self, result string }{
				{"implicit", "", "entries"},
				{"explicit", "this.", "this.entries"},
				{"alias", "alias.", "alias.entries"},
				{"helper", "this.", "this.readBacking()"},
			} {
				t.Run(fmt.Sprintf("%s/units-%d/self-%s", api, units, access.name), func(t *testing.T) {
					root := t.TempDir()
					var paths []string
					for unit := 0; unit < units; unit++ {
						base := filepath.Join(root, fmt.Sprintf("unit%d", unit))
						writeFile(t, filepath.Join(base, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						harness := fmt.Sprintf("ShelfReplay%dTest", unit)
						for _, source := range []struct{ name, body string }{
							{"ShelfCatalog", fmt.Sprintf(`public class ShelfCatalog {
 private static List<Integer> source = new List<Integer>{%d, %d};
 public interface Reader { List<Integer> read(); void replace(List<Integer> items); void reset(); }
 public static Reader open() { return new Shelf(); }
 private class Tray { public List<Integer> contents; }
 private class Shelf implements Reader {
  private List<Integer> scratch { get; set; }
  private Tray selectedTray { get; set; }
  private List<Integer> entries {
   get {
    Shelf alias = this;
    if (%sentries == null) { %sentries = source; }
    return %s;
   }
   private set;
  }
  private List<Integer> readBacking() { return this.entries; }
  public void replace(List<Integer> items) {
   scratch = items;
   selectedTray = new Tray();
   selectedTray.contents = scratch;
   Shelf alias = this;
   alias.entries = selectedTray.contents;
  }
  public void reset() { scratch = null; entries = scratch; }
  public List<Integer> read() {
   List<Integer> result = new List<Integer>();
   for (Integer item : this.entries) { result.add(item); }
   return result;
  }
 }
}`, 4+unit, 7+unit, access.self, access.self, access.result)},
							{harness, fmt.Sprintf(`@IsTest private class %s {
 @IsTest static void replay() {
  ShelfCatalog.Reader subject = ShelfCatalog.open();
  System.assertEquals(new List<Integer>{%d,%d},subject.read());
  System.assertEquals(new List<Integer>{%d,%d},subject.read());
  subject.replace(new List<Integer>{31});
  System.assertEquals(new List<Integer>{31},subject.read());
  System.assertEquals(new List<Integer>{%d,%d},ShelfCatalog.open().read());
  subject.reset();
  System.assertEquals(new List<Integer>{%d,%d},subject.read());
 }
}`, harness, 4+unit, 7+unit, 4+unit, 7+unit, 4+unit, 7+unit, 4+unit, 7+unit)},
						} {
							path := filepath.Join(base, "force-app/classes", source.name+".cls")
							writeFile(t, path, source.body)
							paths = append(paths, path)
						}
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != units || got.Passed != units {
						t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
					}
				})
			}
		}
	}
}
