public class SObjectJSON2Probe {
    private static Map<String,String> ids = new Map<String,String>();
    private static String normalized(String value) {
        for (String id : ids.keySet()) value = value.replace(id, ids.get(id));
        value = value.replaceAll('[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+(?:Z|[+-][0-9]{4})', '<datetime>');
        return value;
    }
    private static Map<String,String> encode(Object value) {
        Map<String,String> out = new Map<String,String>();
        out.put('serialize',normalized(JSON.serialize(value)));
        out.put('pretty',normalized(JSON.serializePretty(value)));
        out.put('suppress',normalized(JSON.serialize(value,true)));
        out.put('prettySuppress',normalized(JSON.serializePretty(value,true)));
        try { out.put('roundTrip',normalized(JSON.serialize(JSON.deserialize(JSON.serialize(value),SObject.class)))); }
        catch(Exception e) { out.put('roundTrip',e.getTypeName()+': '+e.getMessage()); }
        return out;
    }
    public static Map<String,Map<String,String>> observe() {
        Savepoint sp = Database.setSavepoint();
        Account a = new Account(Name='Parent',Phone='123',Industry='Technology',AnnualRevenue=1200.50,
            BillingStreet='1 Main',BillingCity='Town',BillingState='CA',BillingPostalCode='94000',BillingCountry='US',BillingLatitude=12.5,BillingLongitude=-45.25);
        insert a;
        Contact c = new Contact(LastName='Child',AccountId=a.Id,MailingStreet='2 Main',MailingCity='Village'); insert c;
        Opportunity o = new Opportunity(Name='Deal',StageName='Prospecting',CloseDate=Date.newInstance(2030,1,2),Amount=12); insert o;
        Task t = new Task(Subject='One',WhatId=a.Id,WhoId=c.Id); insert t;
        Task t2 = new Task(Subject='Two',WhatId=o.Id); insert t2;
        Json_Place__c p = new Json_Place__c(Name='Place',Point__Latitude__s=12.5,Point__Longitude__s=-45.25); insert p;
        Pricebook2 b = new Pricebook2(Name='Book',IsActive=true); insert b;
        ids.put(String.valueOf(a.Id),'<account>'); ids.put(String.valueOf(c.Id),'<contact>'); ids.put(String.valueOf(o.Id),'<opportunity>');
        ids.put(String.valueOf(t.Id),'<task>'); ids.put(String.valueOf(t2.Id),'<task2>'); ids.put(String.valueOf(p.Id),'<place>'); ids.put(String.valueOf(b.Id),'<book>');
        ids.put(UserInfo.getUserId(),'<user>'); ids.put(UserInfo.getName(),'<user-name>');
        Id aId=a.Id,cId=c.Id,tId=t.Id,t2Id=t2.Id,pId=p.Id,bId=b.Id;
        Map<String,String> queries = new Map<String,String>{
            'multiParent'=>'SELECT LastName,Account.Name,Account.Owner.Name FROM Contact WHERE Id=:cId',
            'multiParentReverse'=>'SELECT Account.Owner.Name,Account.Phone,LastName FROM Contact WHERE Id=:cId',
            'multiParentNull'=>'SELECT LastName,Account.Parent.Owner.Name FROM Contact WHERE Id=:cId',
            'polyParents'=>'SELECT Subject,What.Name,Who.Name,Owner.Name FROM Task WHERE Id=:tId',
            'polyParentsReverse'=>'SELECT Owner.Name,Who.Name,What.Name,Subject FROM Task WHERE Id=:tId',
            'polyNullWho'=>'SELECT Subject,Who.Name,What.Name FROM Task WHERE Id=:t2Id',
            'typeofAccount'=>'SELECT Subject,TYPEOF What WHEN Account THEN Phone,Name WHEN Opportunity THEN Amount,Name ELSE Name END FROM Task WHERE Id=:tId',
            'typeofOpportunity'=>'SELECT Subject,TYPEOF What WHEN Account THEN Phone,Name WHEN Opportunity THEN Amount,Name ELSE Name END FROM Task WHERE Id=:t2Id',
            'typeofWho'=>'SELECT Subject,TYPEOF Who WHEN Contact THEN LastName,FirstName ELSE Name END FROM Task WHERE Id=:tId',
            'fieldsStandard'=>'SELECT FIELDS(STANDARD) FROM Pricebook2 WHERE Id=:bId',
            'fieldsAll'=>'SELECT FIELDS(ALL) FROM Json_Place__c WHERE Id=:pId LIMIT 1',
            'fieldsCustom'=>'SELECT FIELDS(CUSTOM) FROM Json_Place__c WHERE Id=:pId LIMIT 1',
            'toLabel'=>'SELECT Name,toLabel(Industry) FROM Account WHERE Id=:aId',
            'toLabelAlias'=>'SELECT Name,toLabel(Industry) sector FROM Account WHERE Id=:aId',
            'format'=>'SELECT Name,format(AnnualRevenue) FROM Account WHERE Id=:aId',
            'formatAlias'=>'SELECT Name,format(AnnualRevenue) revenue FROM Account WHERE Id=:aId',
            'address'=>'SELECT Name,BillingAddress FROM Account WHERE Id=:aId',
            'addressReverse'=>'SELECT BillingAddress,Name FROM Account WHERE Id=:aId',
            'addressComponents'=>'SELECT BillingCity,BillingAddress,BillingLatitude FROM Account WHERE Id=:aId',
            'mailingAddress'=>'SELECT LastName,MailingAddress FROM Contact WHERE Id=:cId',
            'location'=>'SELECT Name,Point__c FROM Json_Place__c WHERE Id=:pId',
            'locationReverse'=>'SELECT Point__c,Name FROM Json_Place__c WHERE Id=:pId',
            'locationComponents'=>'SELECT Point__Latitude__s,Point__c,Point__Longitude__s FROM Json_Place__c WHERE Id=:pId',
            'unknownField'=>'SELECT Not_A_Field__c FROM Account WHERE Id=:aId',
            'emptyBoundary'=>'SELECT Name,BillingAddress FROM Account WHERE Name=\'Missing synthetic record\''
        };
        Map<String,Map<String,String>> rows = new Map<String,Map<String,String>>();
        for(String shape:queries.keySet()) {
            try {
                List<SObject> found=Database.query(queries.get(shape));
                rows.put(shape,encode(found.isEmpty()?null:found[0]));
            } catch(Exception e) { rows.put(shape,new Map<String,String>{'query'=>e.getTypeName()+': '+e.getMessage()}); }
        }
        Database.rollback(sp);
        return rows;
    }
    public static void emit() {
        Map<String,Map<String,String>> rows=observe();
        for(String shape:rows.keySet()) System.debug('SOBJECT_JSON2|'+shape+'|'+JSON.serialize(rows.get(shape)));
    }
}
