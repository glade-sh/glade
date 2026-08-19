package vm

import "testing"

func TestCB119MetadataSerializationDeduplicatesInheritedFields(t *testing.T) {
	program, err := CompileAnonymous(`
CB119Child item = new CB119Child();
item.fullName = 'Feature.Default';
String serialized = JSON.serialize(item);
System.assertEquals(1, serialized.split('"fullName"').size() - 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "CB119Base",
		Fields: map[string]Field{
			"fullName": {Name: "fullName", Type: "String"},
		},
		FieldOrder: []string{"fullName"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "CB119Child",
		SuperClass: "CB119Base",
		Fields: map[string]Field{
			"fullName": {Name: "fullName", Type: "String"},
		},
		FieldOrder: []string{"fullName"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB119DeployResultSerializationUsesEmptyMessages(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> serialized = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(new Metadata.DeployResult()));
System.assertEquals(true, serialized.containsKey('messages'));
System.assertEquals(0, ((List<Object>)serialized.get('messages')).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB119DeployContainerSerializationUsesComponents(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomMetadata item = new Metadata.CustomMetadata();
item.fullName = 'Feature.Default';
container.addMetadata(item);
Map<String,Object> serialized = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(container));
System.assertEquals(true, serialized.containsKey('components'));
System.assertEquals(false, serialized.containsKey('metadata'));
System.assertEquals(1, container.getMetadata().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB119ConnectorTestUtilRequiresTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
try {
    UserProvisioning.ConnectorTestUtil.createConnectedApp('Outside Test');
    System.assert(false, 'expected System.TypeException');
} catch (System.TypeException e) {
    System.assertEquals('Cannot call test methods in non-test context', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB119ConnectorTestUtilWorksInTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectedApplication app = UserProvisioning.ConnectorTestUtil.createConnectedApp('Local App');
System.assertEquals('Local App', app.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB119SessionManagementMockHasStableAPI67KeySet(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,String> first = Auth.SessionManagement.getCurrentSession();
Map<String,String> second = Auth.SessionManagement.getCurrentSession();
List<String> expectedKeys = new List<String>{
    'NumSecondsValid', 'LastModifiedDate', 'CreatedDate', 'LoginGeoId',
    'LoginHistoryId', 'LoginDomain', 'LogoutUrl', 'ParentId', 'SessionId',
    'SessionSecurityLevel', 'SourceIp', 'LoginSubType', 'LoginType', 'UserType',
    'SessionType', 'Username', 'UsersId'
};
System.assertEquals(17, first.size());
for (String key : expectedKeys) {
    System.assertEquals(true, first.containsKey(key));
}
System.assertEquals(JSON.serialize(first), JSON.serialize(second));
System.assertEquals('local-session', first.get('SessionId'));
System.assertEquals(null, first.get('Username'));
System.assertEquals(null, first.get('UsersId'));
System.assertEquals(null, first.get('SourceIp'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
