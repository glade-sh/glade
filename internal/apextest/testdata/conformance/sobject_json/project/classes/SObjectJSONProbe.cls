public class SObjectJSONProbe {
    private static Map<String, String> ids = new Map<String, String>();
    private static String normalized(String text) {
        for (String id : ids.keySet()) text = text.replace(id, ids.get(id));
        return text;
    }
    private static Map<String, String> encode(Object value, System.Type target) {
        Map<String, String> out = new Map<String, String>();
        out.put('serialize', normalized(JSON.serialize(value)));
        out.put('pretty', normalized(JSON.serializePretty(value)));
        out.put('suppress', normalized(JSON.serialize(value, true)));
        out.put('prettySuppress', normalized(JSON.serializePretty(value, true)));
        try {
            out.put('roundTrip', normalized(JSON.serialize(JSON.deserialize(JSON.serialize(value), target))));
        } catch (Exception e) { out.put('roundTrip', e.getTypeName() + ': ' + e.getMessage()); }
        return out;
    }
    public static Map<String, Map<String, String>> observe() {
        Savepoint sp = Database.setSavepoint();
        Account parent = new Account(Name='Parent', Phone='123', Description=null);
        Account other = new Account(Name='Other');
        insert new List<Account>{parent, other};
        Contact child = new Contact(LastName='Child', FirstName='One', AccountId=parent.Id);
        insert child;
        Json_Record__c custom = new Json_Record__c(Name='Custom', Zeta__c='z', Alpha__c='a', Empty__c=null, Account__c=parent.Id);
        insert custom;
        ids.put(String.valueOf(parent.Id), '<account>');
        ids.put(String.valueOf(other.Id), '<other>');
        ids.put(String.valueOf(child.Id), '<contact>');
        ids.put(String.valueOf(custom.Id), '<custom>');
        Map<String, Object> cases = new Map<String, Object>();
        cases.put('empty', new Account());
        cases.put('constructed', new Account(Name='N', Phone='P', Industry='Technology', Description=null));
        cases.put('reversed', new Account(Description=null, Industry='Technology', Phone='P', Name='N'));
        Account dynamicRecord = new Account();
        dynamicRecord.put('Phone', 'P'); dynamicRecord.put('Name', 'N'); dynamicRecord.put('Description', null);
        dynamicRecord.put('Phone', 'Q');
        cases.put('putOverwrite', dynamicRecord);
        cases.put('nullId', new Account(Id=null, Name='N'));
        cases.put('assignedId15', new Account(Name='N', Id='001000000000001'));
        cases.put('assignedId18', new Account(Id='001000000000001AAA', Name='N'));
        cases.put('inserted', parent);
        Account queryNameId = [SELECT Name, Id FROM Account WHERE Id=:parent.Id];
        cases.put('queryNameId', queryNameId);
        Account queryIdName = [SELECT Id, Name FROM Account WHERE Id=:parent.Id];
        cases.put('queryIdName', queryIdName);
        Account queryFields = [SELECT Id, Name, Phone, Industry, Description FROM Account WHERE Id=:parent.Id];
        cases.put('queryFields', queryFields);
        Account queryReversed = [SELECT Description, Industry, Phone, Name, Id FROM Account WHERE Id=:parent.Id];
        cases.put('queryReversed', queryReversed);
        Account queryImplicitId = [SELECT Name FROM Account WHERE Id=:parent.Id];
        cases.put('queryImplicitId', queryImplicitId);
        Account changed = [SELECT Phone, Name, Description FROM Account WHERE Id=:parent.Id];
        changed.Industry='Technology'; changed.Name='Changed'; changed.Description=null;
        cases.put('queryThenWrite', changed);
        cases.put('constructedParent', new Contact(LastName='C', Account=new Account(Name='P'), Email=null));
        Contact queryParent = [SELECT LastName, Account.Name, Account.Phone, Email FROM Contact WHERE Id=:child.Id];
        cases.put('queryParent', queryParent);
        Account queryNullParent = [SELECT Name, Parent.Name FROM Account WHERE Id=:other.Id];
        cases.put('queryNullParent', queryNullParent);
        Account queryChildren = [SELECT Name, (SELECT LastName, FirstName, Email FROM Contacts) FROM Account WHERE Id=:parent.Id];
        cases.put('queryChildren', queryChildren);
        Account queryEmptyChildren = [SELECT Name, (SELECT LastName FROM Contacts) FROM Account WHERE Id=:other.Id];
        cases.put('queryEmptyChildren', queryEmptyChildren);
        cases.put('customConstructed', new Json_Record__c(Name='C', Zeta__c='z', Alpha__c='a', Empty__c=null));
        cases.put('customInserted', custom);
        Json_Record__c customQuery = [SELECT Name, Zeta__c, Alpha__c, Empty__c FROM Json_Record__c WHERE Id=:custom.Id];
        cases.put('customQuery', customQuery);
        Json_Record__c customQueryReversed = [SELECT Empty__c, Alpha__c, Zeta__c, Name, Id FROM Json_Record__c WHERE Id=:custom.Id];
        cases.put('customQueryReversed', customQueryReversed);
        Json_Record__c customParent = [SELECT Name, Account__r.Name, Empty__c FROM Json_Record__c WHERE Id=:custom.Id];
        cases.put('customParent', customParent);
        cases.put('deserialized', JSON.deserialize('{"Phone":"P","Name":"N","Description":null,"Id":"001000000000001AAA"}', Account.class));
        cases.put('deserializedReversed', JSON.deserialize('{"Id":"001000000000001AAA","Description":null,"Name":"N","Phone":"P"}', Account.class));
        cases.put('emptyList', new List<Account>());
        Map<String, Map<String, String>> rows = new Map<String, Map<String, String>>();
        for (String name : cases.keySet()) {
            Object value = cases.get(name);
            rows.put(name, encode(value, name == 'emptyList' ? List<Account>.class : SObject.class));
        }
        for (String name : new List<String>{'missingAttributes', 'unknownField', 'strictUnknown', 'invalidId', 'nullBoundary'}) {
            String input = name == 'missingAttributes' ? '{"Name":"N"}' : (name == 'unknownField' || name == 'strictUnknown') ? '{"Bogus__c":1}' : name == 'invalidId' ? '{"Id":"bad"}' : 'null';
            try {
                Object value = name == 'strictUnknown' ? JSON.deserializeStrict(input, Account.class) : JSON.deserialize(input, name == 'missingAttributes' ? SObject.class : Account.class);
                rows.put(name, new Map<String, String>{'deserialize'=>JSON.serialize(value)});
            } catch (Exception e) { rows.put(name, new Map<String, String>{'deserialize'=>e.getTypeName() + ': ' + e.getMessage()}); }
        }
        Database.rollback(sp);
        return rows;
    }
    public static void emit() {
        Map<String, Map<String, String>> rows = observe();
        for (String name : rows.keySet()) System.debug('SOBJECT_JSON|' + name + '|' + JSON.serialize(rows.get(name)));
    }
}
