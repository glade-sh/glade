public class PlatformRelationshipProbe {
    public static List<Object> observed = new List<Object>();
    public static void emit(String kind, String parent, String relationship, String child, String field, String query) {
        Map<String,Object> row = new Map<String,Object>{'kind'=>kind,'parent'=>parent,'relationship'=>relationship,'child'=>child,'field'=>field,'query'=>query};
        try { List<SObject> records=Database.query(query); row.put('outcome','ok');
            if(kind=='access') { List<Object> values=new List<Object>(); for(SObject record:records) values.add(record); row.put('values',values); }
        } catch(Exception e) { row.put('outcome','reject'); row.put('message',e.getMessage()); }
        observed.add(row);
        System.debug(LoggingLevel.ERROR,'PLATFORM_ROW|'+JSON.serialize(row));
    }
    public static String compact() {
        List<Object> rows=new List<Object>();
        for(Object entry:observed) { Map<String,Object> row=(Map<String,Object>)entry; rows.add(new List<Object>{row.get('parent'),row.get('relationship'),row.get('outcome')}); }
        return JSON.serialize(rows);
    }
    public static void sweep(String parent) {
        List<String> names = new List<String>(); Map<String,Schema.ChildRelationship> relationships=new Map<String,Schema.ChildRelationship>();
        for(Schema.ChildRelationship rel:Schema.getGlobalDescribe().get(parent).getDescribe().getChildRelationships()) {
            if(rel.getRelationshipName()!=null) relationships.put(rel.getRelationshipName(),rel);
        }
        names.addAll(relationships.keySet()); names.sort();
        for(String name:names) { Schema.ChildRelationship rel=relationships.get(name);
            emit('child',parent,name,String.valueOf(rel.getChildSObject()),String.valueOf(rel.getField()),'SELECT Id,(SELECT Id FROM '+name+' LIMIT 1) FROM '+parent+' LIMIT 1');
        }
        emit('invalid',parent,'NoSuchChildren','','','SELECT Id,(SELECT Id FROM NoSuchChildren) FROM '+parent+' LIMIT 1');
        if(parent!='User') emit('invalid',parent,'Badges','','','SELECT Id,(SELECT Id FROM Badges) FROM '+parent+' LIMIT 1');
    }
    public static void accessRows() {
        Account a=new Account(Name='Relationship probe'); insert a;
        Contact c=new Contact(LastName='Probe',AccountId=a.Id); insert c;
        LinkProbe__c link=new LinkProbe__c(Name='Probe',Account__c=a.Id); insert link;
        String fields='UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel';
        for(String parent:new List<String>{'Account','Contact','Case','Opportunity','LinkProbe__c','User'}) emit('access',parent,'UserRecordAccess','','','SELECT '+fields+' FROM '+parent+' LIMIT 1');
        emit('access','Account','lowercase','','','SELECT userrecordaccess.hasreadaccess FROM Account LIMIT 1');
        emit('access','Contact','parent','','','SELECT Account.UserRecordAccess.HasEditAccess FROM Contact LIMIT 1');
        emit('access','LinkProbe__c','parent','','','SELECT Account__r.UserRecordAccess.HasEditAccess FROM LinkProbe__c LIMIT 1');
        emit('access','Account','child','','','SELECT Id,(SELECT UserRecordAccess.HasEditAccess FROM Contacts) FROM Account LIMIT 1');
        for(String leaf:new List<String>{'Id','RecordId','UserId','Bogus'}) emit('access','Account',leaf,'','','SELECT UserRecordAccess.'+leaf+' FROM Account LIMIT 1');
        emit('access','Account','where','','','SELECT Id FROM Account WHERE UserRecordAccess.HasReadAccess=true');
        emit('access','Account','order','','','SELECT Id FROM Account ORDER BY UserRecordAccess.HasReadAccess');
        emit('access','Account','group','','','SELECT UserRecordAccess.HasReadAccess FROM Account GROUP BY UserRecordAccess.HasReadAccess');
        emit('access','Account','missingParent','','','SELECT Parent.UserRecordAccess.HasReadAccess FROM Account LIMIT 1');
    }
}
