Map<String, Schema.SObjectType> globalTypes = Schema.getGlobalDescribe();
Schema.DescribeSObjectResult parentDescribe = globalTypes.get('Orchard__c').getDescribe();
Schema.DescribeSObjectResult childDescribe = globalTypes.get('AlphaBranch__c').getDescribe();
List<String> customFieldNames = new List<String>();
for (String fieldName : childDescribe.fields.getMap().keySet()) {
    if (fieldName.endsWith('__c')) customFieldNames.add(fieldName);
}
System.debug('CHILD_FIELDS|' + JSON.serialize(customFieldNames));

List<String> childOrder = new List<String>();
for (Schema.ChildRelationship relationship : parentDescribe.getChildRelationships()) {
    String childName = relationship.getChildSObject().getDescribe().getName();
    if (childName == 'AlphaBranch__c' || childName == 'ZuluBranch__c') {
        childOrder.add(childName + '|' + relationship.getField().getDescribe().getName() + '|' + relationship.getRelationshipName());
    }
}
System.debug('CHILD_ORDER|' + JSON.serialize(childOrder));

try {
    List<String> fieldPaths = new List<String>();
    for (Schema.FieldSetMember member : parentDescribe.fieldSets.getMap().get('Overview').getFields()) {
        fieldPaths.add(member.getFieldPath());
    }
    System.debug('FIELDSET_PATHS|' + JSON.serialize(fieldPaths));
} catch (Exception ex) {
    System.debug('FIELDSET_PATHS|' + ex.getTypeName() + '|' + ex.getMessage());
}

try {
    SObject parent = globalTypes.get('Orchard__c').newSObject();
    parent.put('Name', 'Matrix Parent');
    insert parent;
    Date sharedDate = Date.newInstance(2024, 3, 7);
    List<SObject> childRecords = new List<SObject>();
    for (String childName : new List<String>{'First', 'Second', 'Third'}) {
        SObject child = globalTypes.get('AlphaBranch__c').newSObject();
        child.put('Name', childName);
        child.put('AB_Orchard__c', parent.get('Id'));
        child.put('VisitDate__c', sharedDate);
        childRecords.add(child);
    }
    insert childRecords;
    String query = 'SELECT Name FROM AlphaBranch__c WHERE AB_Orchard__c = \'' + String.valueOf(parent.get('Id')) + '\' ORDER BY VisitDate__c DESC';
    List<String> tieOrder = new List<String>();
    for (SObject row : Database.query(query)) tieOrder.add(String.valueOf(row.get('Name')));
    System.debug('CHILD_TIE_ORDER|' + JSON.serialize(tieOrder));
} catch (Exception ex) {
    System.debug('CHILD_TIE_ORDER|' + ex.getTypeName() + '|' + ex.getMessage());
}
