public class ChangeEventFollowup {
    public static Map<String,Object> rows = new Map<String,Object>();
    public static void run() {
        Map<String, Schema.SObjectField> fields = AccountChangeEvent.SObjectType.getDescribe().fields.getMap();
        for (String name : new List<String>{'FirstName','LastName','Salutation'}) {
            Schema.DescribeFieldResult f = fields.get(name).getDescribe();
            emit('shape.' + name, new Map<String,Object>{'type'=>String.valueOf(f.getType()),'length'=>f.getLength(),'nillable'=>f.isNillable(),'label'=>f.getLabel()});
        }
        for (String objectName : new List<String>{'Account','AccountChangeEvent'}) {
            Map<String,Schema.SObjectField> ff = Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap();
            Map<String,Object> presence = new Map<String,Object>();
            for (String name : new List<String>{'Division','IsCustomerPortal','RecordTypeId'}) presence.put(name, ff.containsKey(name));
            emit('features.' + objectName, presence);
        }
        for (String objectName : new List<String>{'Parcel__c','Parcel__ChangeEvent'}) {
            Map<String,Schema.SObjectField> ff = Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap();
            List<String> names = new List<String>(ff.keySet()); names.sort();
            List<String> readable = new List<String>();
            for (String name : names) if (ff.get(name).getDescribe().isAccessible()) readable.add(name);
            emit('fls.' + objectName, new Map<String,Object>{'fields'=>names,'readable'=>readable});
        }
        List<String> children = new List<String>();
        for (Schema.ChildRelationship child : User.SObjectType.getDescribe().getChildRelationships()) {
            String name = String.valueOf(child.getChildSObject());
            if (name.endsWith('ChangeEvent')) children.add(name + '.' + child.getField() + ':' + child.getRelationshipName());
        }
        children.sort();
        emit('children.User', children);
    }
    private static void emit(String id, Object value) {
        if (id == 'children.User') {
            List<String> installed = new List<String>();
            for (String child : (List<String>)value) {
                if (child.startsWith('AccountChangeEvent.') || child.startsWith('ContactChangeEvent.') || child.startsWith('CaseChangeEvent.') || child.startsWith('Parcel__ChangeEvent.')) installed.add(child);
            }
            rows.put(id, installed);
        } else rows.put(id, value);
        System.debug('FOLLOWUP ' + id + '\t' + JSON.serialize(value));
    }
}
