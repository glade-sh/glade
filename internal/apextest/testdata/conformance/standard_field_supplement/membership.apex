Map<String,Object> rows = new Map<String,Object>();
for (String obj : new List<String>{'Account','Lead','Order'}) {
    Map<String,Schema.SObjectField> fm = Schema.getGlobalDescribe().get(obj).getDescribe().fields.getMap();
    Map<String,Object> fs = new Map<String,Object>();
    for (String name : fm.keySet()) {
        Schema.DescribeFieldResult f = fm.get(name).getDescribe();
        fs.put(f.getName(), new Map<String,Object>{'type'=>String.valueOf(f.getType()),'createable'=>f.isCreateable(),'updateable'=>f.isUpdateable(),'nillable'=>f.isNillable(),'accessible'=>f.isAccessible(),'permissionable'=>f.isPermissionable(),'length'=>f.getLength(),'precision'=>f.getPrecision(),'scale'=>f.getScale()});
    }
    rows.put(obj, fs);
    rows.put(obj+'_boundary', fm.containsKey('Id'));
    rows.put(obj+'_rejection', fm.containsKey('Case251DefinitelyMissing__c'));
}
System.debug('CASE251_JSON:' + JSON.serialize(rows));
