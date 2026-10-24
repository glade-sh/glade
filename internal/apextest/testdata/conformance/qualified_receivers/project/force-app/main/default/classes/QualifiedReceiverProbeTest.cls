@IsTest
private class QualifiedReceiverProbeTest {
  @IsTest static void observe() {
    Object value0 = Schema.Account.getSObjectType();
    Object value1 = Schema.Account.SObjectType;
    Object value2 = Schema.Account.sObjectType;
    Object value3 = Schema.Account.SObjectType.fields.Name.getDescribe();
    Object value4 = Account.SObjectType.fields.Name.getDescribe();
    Object value5 = SObjectType.Account.fields.Name.getName();
    Object value6 = SObjectType.Account.fieldSets.Summary.getFields();
    Object value7 = Schema.SObjectType.Account.fieldSets.Summary.getFields();
    Object value8 = Schema.SObjectType.Account.getName();
    Object value9 = Schema.SObjectType.Account.fields.getMap();
    Object value10 = Account.fields.Name.getDescribe();
    Object value11 = Schema.Account.fields.Name.getDescribe();
    Object value12 = Schema.Meadow__c.getSObjectType();
    Object value13 = Schema.Meadow__c.SObjectType;
    Object value14 = Schema.Meadow__c.sObjectType;
    Object value15 = Schema.Meadow__c.SObjectType.fields.Flag__c.getDescribe();
    Object value16 = Meadow__c.SObjectType.fields.Flag__c.getDescribe();
    Object value17 = SObjectType.Meadow__c.fields.Flag__c.getName();
    Object value18 = SObjectType.Meadow__c.fieldSets.Summary.getFields();
    Object value19 = Schema.SObjectType.Meadow__c.fieldSets.Summary.getFields();
    Object value20 = Schema.SObjectType.Meadow__c.getName();
    Object value21 = Schema.SObjectType.Meadow__c.fields.getMap();
    Object value22 = Meadow__c.fields.Flag__c.getDescribe();
    Object value23 = Schema.Meadow__c.fields.Flag__c.getDescribe();
    Object value24 = Label.MeadowCaption.replace('a', 'b');
    Object value25 = System.Label.MeadowCaption.replace('a', 'b');
    Object value26 = Label.MeadowCaption.length();
    Object value27 = String.format(Label.MeadowCaption, new List<String>{'x'});
    System.assert(false, 'OBS|' + JSON.serialize(QualifiedReceiverProbe.observe()));
  }
}
