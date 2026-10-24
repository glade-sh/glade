List<String> objectNames = new List<String>{'Account', 'Contact'};
Map<String, List<String>> fieldsByObject = new Map<String, List<String>>{
    'Account' => new List<String>{'Id', 'Name', 'CreatedDate', 'ParentId'},
    'Contact' => new List<String>{'Id', 'FirstName', 'LastName', 'CreatedDate', 'AccountId'}
};
Map<String, Schema.SObjectType> globalDescribe = Schema.getGlobalDescribe();
for (String objectName : objectNames) {
    Schema.DescribeSObjectResult objectDescribe = globalDescribe.get(objectName).getDescribe();
    Map<String, Object> objectRow = new Map<String, Object>{
        'kind' => 'object',
        'object' => objectName,
        'label' => objectDescribe.getLabel(),
        'pluralLabel' => objectDescribe.getLabelPlural()
    };
    System.debug('NATIVE_ROW:' + JSON.serialize(objectRow));
    Map<String, Schema.SObjectField> objectFields = objectDescribe.fields.getMap();
    for (String fieldName : fieldsByObject.get(objectName)) {
        Schema.DescribeFieldResult fieldDescribe = objectFields.get(fieldName).getDescribe();
        List<String> referenceTo = new List<String>();
        for (Schema.SObjectType referenceType : fieldDescribe.getReferenceTo()) {
            referenceTo.add(referenceType.getDescribe().getName());
        }
        referenceTo.sort();
        Map<String, Object> fieldRow = new Map<String, Object>{
            'kind' => 'field',
            'object' => objectName,
            'field' => fieldName,
            'type' => String.valueOf(fieldDescribe.getType()).toUpperCase(),
            'label' => fieldDescribe.getLabel(),
            'referenceTo' => referenceTo,
            'relationshipName' => fieldDescribe.getRelationshipName()
        };
        System.debug('NATIVE_ROW:' + JSON.serialize(fieldRow));
    }
}
Schema.DescribeSObjectResult accountDescribe = globalDescribe.get('Account').getDescribe();
for (Schema.ChildRelationship childRelationship : accountDescribe.getChildRelationships()) {
    Schema.SObjectType childType = childRelationship.getChildSObject();
    Schema.SObjectField childField = childRelationship.getField();
    if (childType != null && childField != null && childType.getDescribe().getName() == 'Account' && childField.getDescribe().getName() == 'ParentId') {
        Map<String, Object> relationshipRow = new Map<String, Object>{
            'kind' => 'childRelationship',
            'object' => childType.getDescribe().getName(),
            'field' => childField.getDescribe().getName(),
            'parentObject' => 'Account',
            'relationshipName' => childRelationship.getRelationshipName()
        };
        System.debug('NATIVE_ROW:' + JSON.serialize(relationshipRow));
    }
}
