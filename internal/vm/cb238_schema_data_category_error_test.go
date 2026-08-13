package vm

import "testing"

func TestCB238SchemaDataCategoryMissingMetadataErrors(t *testing.T) {
	program, err := CompileAnonymous(`
try {
  Schema.describeDataCategoryGroups(new List<String>{'Account'});
  System.assert(false, 'describeDataCategoryGroups should fail without data category metadata');
} catch (Exception e) {
  System.assertEquals('System.InvalidParameterValueException', e.getTypeName());
}
try {
  Schema.DataCategoryGroupSobjectTypePair pair = new Schema.DataCategoryGroupSobjectTypePair();
  pair.setSobject('Knowledge__kav');
  pair.setDataCategoryGroupName('Products');
  Schema.describeDataCategoryGroupStructures(
    new List<Schema.DataCategoryGroupSobjectTypePair>{pair}, false);
  System.assert(false, 'describeDataCategoryGroupStructures should fail without data category metadata');
} catch (Exception e) {
  System.assertEquals('System.NullPointerException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
