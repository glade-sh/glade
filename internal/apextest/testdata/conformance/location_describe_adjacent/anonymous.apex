for (String objectName : new List<String>{'Account','PlaceProbe__c'}) {
    Map<String, Schema.SObjectField> fields = Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap();
    String point = objectName == 'Account' ? 'Second_Point' : 'Place_Point';
    String text = objectName == 'Account' ? 'Second_Text' : 'Place_Text';
    for (String name : new List<String>{point+'__c',point+'__Latitude__s',point.toLowerCase()+'__longitude__s',text+'__c',text+'__Latitude__s','Absent__Longitude__s'}) {
        Schema.SObjectField f = fields.get(name);
        System.debug('CAPTURE|' + objectName + '|' + name + '|' + fields.containsKey(name) + '|' + (f == null ? 'null' : f.getDescribe().getName()) + '|' + (f == null ? 'null' : String.valueOf(f.getDescribe().getCompoundFieldName())));
    }
}
