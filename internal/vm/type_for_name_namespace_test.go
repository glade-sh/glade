package vm

import "testing"

func TestExecTypeForNameResolvesCommonPlatformNamespaces(t *testing.T) {
	program, err := CompileAnonymous(`
Type displayType = Type.forName('Schema', 'DisplayType');
System.assertNotEquals(null, displayType);
System.assertEquals('Schema.DisplayType', displayType.getName());

Type describeField = Type.forName('Schema.DescribeFieldResult');
System.assertNotEquals(null, describeField);
System.assertEquals('Schema.DescribeFieldResult', describeField.getName());

Type standardController = Type.forName('ApexPages', 'StandardController');
System.assertNotEquals(null, standardController);
System.assertEquals('ApexPages.StandardController', standardController.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
