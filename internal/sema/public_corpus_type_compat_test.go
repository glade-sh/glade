package sema

import (
	"path/filepath"
	"testing"
)

func TestPublicCorpusMapArrayShorthandCompatibility(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MapArrayShorthand.cls"), `
public class MapArrayShorthand {
  public void run(List<Opportunity> opportunities) {
    Map<String, Opportunity[]> expected = new Map<String, Opportunity[]>();
    expected.put('key', new List<Opportunity>{ new Opportunity(Name = 'Acme') });
    expected.get('key').add(new Opportunity(Name = 'Beta'));

    Map<Object, Entry[]> entriesByValue = new Map<Object, Entry[]>();
    entriesByValue.put('x', new Entry[]{ new Entry() });
  }
  public class Entry {}
}
`)
	result := analyzePublicCorpusFiles(t, root, "MapArrayShorthand.cls")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Map<String")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Map<Object")
}

func TestPublicCorpusChainedAssignmentReturnsAssignedValue(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ChainedAssignment.cls"), `
public class ChainedAssignment {
  public class Result {}
  public Result getMockResult() { return new Result(); }
  public void run() {
    List<Result> mockResults = new List<Result>();
    mockResults.add(null);
    Result mockResult = mockResults[0] = getMockResult();
  }
}
`)
	result := analyzePublicCorpusFiles(t, root, "ChainedAssignment.cls")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "mockResult with void")
}
