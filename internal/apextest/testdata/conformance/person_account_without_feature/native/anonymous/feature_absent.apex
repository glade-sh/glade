Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.debug('FEATURE_ABSENT|' + JSON.serialize(new Map<String, Object>{
    'FirstName' => fields.containsKey('FirstName'),
    'LastName' => fields.containsKey('LastName'),
    'PersonEmail' => fields.containsKey('PersonEmail'),
    'IsPersonAccount' => fields.containsKey('IsPersonAccount')
}));
