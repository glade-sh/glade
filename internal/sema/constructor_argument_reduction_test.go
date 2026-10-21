package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestConstructorArgumentReduction(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"src/classes", "src/objects/AssessmentIndicatorDefinition/fields", "src/objects/AssessmentTaskIndDefinition/fields"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeSemaFile(t, filepath.Join(root, "src/objects/AssessmentIndicatorDefinition/AssessmentIndicatorDefinition.object-meta.xml"), `<CustomObject/>`)
	for name, body := range map[string]string{
		"ParentDefinition__c": `<type>Lookup</type><referenceTo>AssessmentIndicatorDefinition</referenceTo><relationshipName>Definitions</relationshipName>`,
		"Criticality__c":      `<type>Picklist</type>`,
		"Group__c":            `<type>Text</type><length>255</length>`,
		"QuestionKind__c":     `<type>Picklist</type>`,
	} {
		writeSemaFile(t, filepath.Join(root, "src/objects/AssessmentIndicatorDefinition/fields", name+".field-meta.xml"), `<CustomField><fullName>`+name+`</fullName>`+body+`</CustomField>`)
	}
	writeSemaFile(t, filepath.Join(root, "src/objects/AssessmentTaskIndDefinition/fields/ExternalKey__c.field-meta.xml"), `<CustomField><fullName>ExternalKey__c</fullName><type>Text</type><length>255</length></CustomField>`)
	writeSemaFile(t, filepath.Join(root, "src/classes/ConstructorArgumentProbe.cls"), `
public class ConstructorArgumentProbe {
  public void run(Id visitId) {
    List<AssessmentTaskIndDefinition> definitions = [SELECT Id, DisplayOrder,
      AssessmentIndDefinition.Description, AssessmentIndDefinition.Name,
      AssessmentIndDefinition.DataType, AssessmentIndDefinition.ParentDefinition__c,
      AssessmentIndDefinitionId, AssessmentIndDefinition.Criticality__c,
      AssessmentIndDefinition.Group__c, AssessmentIndDefinition.QuestionKind__c
      FROM AssessmentTaskIndDefinition WHERE Id = :visitId WITH SECURITY_ENFORCED];
    Map<String, Integer> counts = getCounts(visitId);
    Map<String, InspectionAssessmentInd> responses = new Map<String, InspectionAssessmentInd>();
    Map<Id, Id> taskIds = new Map<Id, Id>();
    GroupWrapper groupValue = new GroupWrapper();
    for (AssessmentTaskIndDefinition definition : definitions) {
      Id taskId = taskIds.get(definition.AssessmentTaskDefinitionId);
      InspectionAssessmentInd response = responses.get(String.valueOf(taskId));
      String text = !String.isBlank(definition.AssessmentIndDefinition.Description)
        ? definition.AssessmentIndDefinition.Name + ' ' + definition.AssessmentIndDefinition.Description
        : definition.AssessmentIndDefinition.Name;
      groupValue.values.add(new Wrapper(text, taskId,
        definition.AssessmentIndDefinition.DataType,
        definition.AssessmentIndDefinition.ParentDefinition__c,
        definition.AssessmentIndDefinitionId,
        'Critical'.equalsIgnoreCase(definition.AssessmentIndDefinition.Criticality__c),
        definition.DisplayOrder, response,
        counts.get((String.isNotBlank(definition.AssessmentIndDefinition.Group__c)
          ? definition.AssessmentIndDefinition.Group__c : '') + ':' + String.valueOf(definition.AssessmentIndDefinitionId)),
        definition.AssessmentIndDefinition.QuestionKind__c));
    }
  }
  private Map<String, Integer> getCounts(Id visitId) { return new Map<String, Integer>(); }
  public class GroupWrapper { public List<Wrapper> values = new List<Wrapper>(); }
  public class Wrapper {
    public Wrapper(String label, Id taskId, String kind, Id parentId, Id definitionId,
      Boolean critical, Integer orderValue, InspectionAssessmentInd response,
      Integer countValue, String category) {}
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "src/classes/AdjacentConstructorProbe.cls"), `
public class AdjacentConstructorProbe {
  public class Wrapper {
    public Wrapper(String label, Id taskId, String kind, Id parentId, Id definitionId,
      Boolean critical) {}
  }
}
`)
	invalidPath := filepath.Join(root, "src/classes/InvalidConstructorArgumentProbe.cls")
	writeSemaFile(t, invalidPath, `
public class InvalidConstructorArgumentProbe {
  public void run() { new Wrapper(false); }
  public class Wrapper { public Wrapper(String label) {} }
}
`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, sch)
	result := Analyze(index)
	denialControls := 0
	for _, diag := range result.Diagnostics {
		if diag.File == invalidPath && diag.Code == "GLADESEMA011" {
			denialControls++
			continue
		}
		if diag.Severity == "error" {
			t.Errorf("unexpected diagnostic: %#v", diag)
		}
	}
	if denialControls != 1 {
		t.Errorf("invalid-constructor denial controls = %d, want 1", denialControls)
	}
}
