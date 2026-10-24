public class SchemaNameProbe {
    public static String observe() {
        Map<String, Object> out = new Map<String, Object>();
        Schema.SObjectType token = Schema.getGlobalDescribe().get('aCcOuNt');
        out.put('global', token.getDescribe().getName());
        out.put('nameField', token.getDescribe().fields.getMap().get('nAmE').getDescribe().isNameField());
        out.put('idField', token.getDescribe().fields.getMap().get('iD').getDescribe().isNameField());
        out.put('newRecord', token.newSObject().getSObjectType().getDescribe().getName());
        out.put('reference', Contact.AccountId.getDescribe().getReferenceTo()[0].getDescribe().getName());
        Boolean childFound = false;
        for (Schema.ChildRelationship rel : token.getDescribe().getChildRelationships()) {
            if (rel.getRelationshipName() == 'Contacts' && rel.getChildSObject() == Contact.SObjectType) childFound = true;
        }
        out.put('childDescribe', childFound);
        Account parent = new Account(Name = 'Meadow');
        insert parent;
        Contact child = new Contact(LastName = 'River', AccountId = parent.Id);
        insert child;
        Account loaded = [SELECT Name, (SELECT LastName FROM Contacts) FROM Account WHERE Id = :parent.Id];
        out.put('childQuery', loaded.getSObjects('cOnTaCtS').size());
        Account decoded = (Account)JSON.deserialize('{"Contacts":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Contact"},"LastName":"Brook"}]}}', Account.class);
        out.put('childJson', decoded.Contacts[0].LastName);
        return JSON.serialize(out);
    }
}
