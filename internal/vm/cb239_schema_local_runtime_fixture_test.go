package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSchemaStaticAppAndModuleDescribe(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertNotEquals(null, Schema.getAppDescribe('Sales'));
System.assert(0 < Schema.getAppDescribe('Sales').size());
System.assertNotEquals(null, Schema.getModuleDescribe());
System.assert(0 < Schema.getModuleDescribe().size());
System.assertNotEquals(null, Schema.getModuleDescribe('Sales'));
System.assert(0 < Schema.getModuleDescribe('Sales').size());
System.assertEquals(Schema.getAppDescribe('Sales'), Schema.getModuleDescribe('Sales'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestPicklistEntryActiveAndDefaultValueFieldsExist(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> entries = Account.Type.getDescribe().getPicklistValues();
System.assert(0 < entries.size());
Object first = entries.get(0);
Boolean active = first.active;
System.assertNotEquals(null, active);
Boolean dv = first.defaultValue;
System.assertNotEquals(null, dv);
Boolean hasDefault = first.isDefaultValue();
System.assertNotEquals(null, hasDefault);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFilteredLookupInfoFromReferenceFieldWithControllingFields(t *testing.T) {
	program, err := CompileAnonymous(`
Object info = Probe__c.Lookup__c.getDescribe().getFilteredLookupInfo();
System.assertNotEquals(null, info);
Boolean dependent = info.isDependent();
Boolean optionalFilter = info.isOptionalFilter();
List<String> controlling = info.getControllingFields();
System.assertNotEquals(null, controlling);
System.assertNotEquals(null, info.controllingFields);
System.assertNotEquals(null, info.dependent);
System.assertNotEquals(null, info.optionalFilter);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Probe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Probe__c",
			Label:       "Probe",
			PluralLabel: "Probes",
			KeyPrefix:   "a0B",
			Fields: map[string]storage.Field{
				"Text__c":   {APIName: "Text__c", Type: storage.FieldString, DisplayType: "STRING"},
				"Lookup__c": {APIName: "Lookup__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Account"}, FilteredLookupInfo: storage.FilteredLookupInfo{ControllingFields: []string{"Account.Name"}, Dependent: true, OptionalFilter: true}},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeTabResultGetIconUrlAndMiniIconUrl(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> tabSets = Schema.describeTabs();
System.assert(0 < tabSets.size());
Object standard = tabSets.get(0);
List<Object> tabs = standard.getTabs();
System.assert(0 < tabs.size());
Object tab = tabs.get(0);
// Getters return values — must not throw
String name = tab.getName();
String label = tab.getLabel();
Boolean custom = tab.isCustom();
Boolean notNull = tab != null;
notNull = custom != null;
notNull = name != null;
notNull = label != null;
// Field access — must not throw
notNull = tab.name != null;
notNull = tab.label != null;
notNull = tab.custom != null;
notNull = tab.colors != null;
notNull = tab.icons != null;
notNull = tab.tabEnumOrId != null;
notNull = tab.url != null;
notNull = tab.mobileUrl != null;
notNull = tab.sobjectName != null;
notNull = tab.iconUrl != null;
notNull = tab.miniIconUrl != null;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeTabResultGetters(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> tabSets = Schema.describeTabs();
Object standard = tabSets.get(0);
List<Object> tabs = standard.getTabs();
Object tab = tabs.get(0);
tab.getIconUrl();
tab.getMiniIconUrl();
tab.getMobileUrl();
tab.getTabEnumOrId();
tab.getUrl();
tab.getIcons();
tab.getColors();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeTabSetResultAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> tabSets = Schema.describeTabs();
System.assert(0 < tabSets.size());
Object ts = tabSets.get(0);
// Getters return values — must not throw
String name = ts.getName();
String label = ts.getLabel();
String tabDesc = ts.getDescription();
Boolean sel = ts.isSelected();
List<Object> tabs = ts.getTabs();
Boolean ok = name != null && label != null && tabDesc != null && sel != null && tabs != null;
// Field access — must not throw
Object logoUrl = ts.logoUrl;
Object ns = ts.namespace;
Object tabSetId = ts.tabSetId;
System.assert(ok);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeIconResultAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> tabSets = Schema.describeTabs();
Object ts = tabSets.get(0);
List<Object> tabs = ts.getTabs();
Object tab = tabs.get(0);
List<Object> icons = tab.getIcons();
System.assert(0 < icons.size());
Object icon = icons.get(0);
icon.getContentType();
icon.getHeight();
icon.getTheme();
icon.getUrl();
icon.getWidth();
icon.contentType;
icon.height;
icon.theme;
icon.url;
icon.width;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeColorResultAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> tabSets = Schema.describeTabs();
Object ts = tabSets.get(0);
List<Object> tabs = ts.getTabs();
Object tab = tabs.get(0);
List<Object> colors = tab.getColors();
System.assert(0 < colors.size());
Object color = colors.get(0);
color.getColor();
color.getContext();
color.getTheme();
color.color;
color.context;
color.theme;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeSObjectResultGetFieldSetsWithMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = describe.getFieldSets().getMap();
System.assertNotEquals(null, fieldSets.get('Summary'));
Object summary = fieldSets.get('Summary');
System.assertEquals('Account Summary', summary.getLabel());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Account",
		Name:       "Summary",
		Label:      "Account Summary",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeSObjectResultGetAssociateEntityType(t *testing.T) {
	program, err := CompileAnonymous(`
Object accountDescribe = Account.SObjectType.getDescribe();
System.assertEquals(null, accountDescribe.getAssociateEntityType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFieldSetHashCodeEqualsToString(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = describe.getFieldSets().getMap();
Object summary = fieldSets.get('Summary');
System.assertNotEquals(null, summary.hashCode());
System.assertNotEquals(null, summary.toString());
System.assertEquals(true, summary.equals(summary));
System.assertEquals(false, summary.equals(null));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Account",
		Name:       "Summary",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFieldSetMemberHashCodeEqualsToString(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = describe.getFieldSets().getMap();
Object summary = fieldSets.get('Summary');
List<Object> members = summary.getFields();
System.assert(0 < members.size());
Object member = members.get(0);
System.assertNotEquals(null, member.hashCode());
System.assertNotEquals(null, member.toString());
System.assertEquals(true, member.equals(member));
System.assertEquals(false, member.equals(null));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Account",
		Name:       "Summary",
		Fields:     []storage.FieldSetMemberMetadata{{Field: "Name"}},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFilteredLookupInfoHashCodeEqualsToString(t *testing.T) {
	program, err := CompileAnonymous(`
Object info = Probe__c.Lookup__c.getDescribe().getFilteredLookupInfo();
System.assertNotEquals(null, info.hashCode());
System.assertNotEquals(null, info.toString());
System.assertEquals(true, info.equals(info));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Probe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Probe__c",
			Label:       "Probe",
			PluralLabel: "Probes",
			KeyPrefix:   "a0B",
			Fields: map[string]storage.Field{
				"Text__c":   {APIName: "Text__c", Type: storage.FieldString, DisplayType: "STRING"},
				"Lookup__c": {APIName: "Lookup__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Account"}, FilteredLookupInfo: storage.FilteredLookupInfo{ControllingFields: []string{"Account.Name"}, Dependent: true, OptionalFilter: true}},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFieldSetSObjectTypeField(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = describe.getFieldSets().getMap();
Object summary = fieldSets.get('Summary');
System.assertEquals(Account.SObjectType, summary.sObjectType);
System.assertEquals(Account.SObjectType, summary.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Account",
		Name:       "Summary",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFieldSetNamespaceFieldAndGetter(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = describe.getFieldSets().getMap();
Object summary = fieldSets.get('Summary');
System.assertEquals(null, summary.namespace);
System.assertEquals(null, summary.getNamespace());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Account",
		Name:       "Summary",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFieldSetDescriptionField(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = describe.getFieldSets().getMap();
Object summary = fieldSets.get('Summary');
System.assertEquals('Account Summary', summary.description);
System.assertEquals('Account Summary', summary.getDescription());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName:  "Account",
		Name:        "Summary",
		Description: "Account Summary",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
