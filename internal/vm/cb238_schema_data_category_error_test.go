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
  // Schema describe R265 uses an existing object with no category support.
  pair.setSobject('Account');
  pair.setDataCategoryGroupName('A23MissingCategoryGroup');
  Schema.describeDataCategoryGroupStructures(
    new List<Schema.DataCategoryGroupSobjectTypePair>{pair}, false);
  System.assert(false, 'describeDataCategoryGroupStructures should fail without data category metadata');
} catch (Exception e) {
  System.assertEquals('System.InvalidParameterValueException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
