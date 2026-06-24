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

func TestPublicCorpusNestedFluentReturnKeepsOwnerType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SObjectDataLoader.cls"), `
public class SObjectDataLoader {
  public static String serialize(Object records, SerializeConfig config) {
    return '';
  }

  public class SerializeConfig {
    public SerializeConfig auto(Schema.SObjectType objectType) {
      return this;
    }

    public SerializeConfig follow(Schema.SObjectField field) {
      return this;
    }

    public SerializeConfig followChild(Schema.SObjectField field) {
      return this;
    }

    public SerializeConfig omit(Schema.SObjectField field) {
      return this;
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSObjectDataLoader.cls"), `
public class UsesSObjectDataLoader {
  public void run() {
    String serializedData = SObjectDataLoader.serialize(new List<Opportunity>(),
      new SObjectDataLoader.SerializeConfig().
        followChild(OpportunityLineItem.OpportunityId).     // child records
          follow(OpportunityLineItem.PricebookEntryId).     // price book entries
            follow(PricebookEntry.Product2Id).              // products
            omit(OpportunityLineItem.UnitPrice));

    serializedData = SObjectDataLoader.serialize(new Set<Id>{'006000000000000AAA'},
      new SObjectDataLoader.SerializeConfig().
        auto(Opportunity.sObjectType).          // infer graph
        omit(Opportunity.AccountId));
  }
}
`)
	result := analyzePublicCorpusFiles(t, root, "SObjectDataLoader.cls", "UsesSObjectDataLoader.cls")

	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "follow")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "omit")
}
