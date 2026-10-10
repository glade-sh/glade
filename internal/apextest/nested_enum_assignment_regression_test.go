package apextest

import (
	"path/filepath"
	"testing"
)

func TestRunNestedEnumArgumentDoesNotResolveOuterStaticField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RepositorySortOrder.cls"), `
public class RepositorySortOrder {
  private final SortOrder sortOrder;
  public enum SortOrder { ASCENDING, DESCENDING }
  public static final RepositorySortOrder ASCENDING {
    get {
      if (ASCENDING == null) {
        ASCENDING = new RepositorySortOrder(SortOrder.ASCENDING);
      }
      return ASCENDING;
    }
    set;
  }
  public RepositorySortOrder(SortOrder sortOrder) { this.sortOrder = sortOrder; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RepositorySortOrderTest.cls"), `
@isTest
private class RepositorySortOrderTest {
  @isTest static void nestedEnumArgumentResolvesToEnum() {
    RepositorySortOrder value = RepositorySortOrder.ASCENDING;
    System.assertNotEquals(null, value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}
