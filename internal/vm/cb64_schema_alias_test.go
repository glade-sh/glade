package vm

import "testing"

func TestExecAPI67QualifiedSchemaDataCategoryDescribeAliases(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, System.Schema.describeDataCategoryGroups(new List<String>()).size());
System.assertEquals(0, System.Schema.describeDataCategoryGroupStructures(new List<Schema.DataCategoryGroupSobjectTypePair>(), false).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
