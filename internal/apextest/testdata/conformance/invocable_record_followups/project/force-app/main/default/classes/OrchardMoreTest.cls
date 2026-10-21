@IsTest
public with sharing class OrchardMoreTest {
    public static Integer reentries=0;
    public static Boolean nested=false;
    public static Invocable.Action.Result action(String name, SObject record, Map<String,Object> extra) {
        Invocable.Action a=Invocable.Action.createCustomAction('flow',name);
        Map<String,Object> inputs=new Map<String,Object>{'record'=>record}; if(extra!=null) inputs.putAll(extra);
        a.setInvocations(new List<Map<String,Object>>{inputs});
        Invocable.Action.Result result=a.invoke()[0]; System.assert(result.isSuccess(),String.valueOf(result.getErrors()));return result;
    }
    static Case invoke(String name, Case record) {return (Case)action(name,record,null).getOutputParameters().get('record');}
    static void emit(String name,Map<String,String> rows){System.assert(false,'OBSERVED|'+name+'|'+JSON.serialize(rows));}
    @IsTest static void sortedLookup(){
        insert new List<Account>{new Account(Name='Zulu',Site='group'),new Account(Name='Alpha',Site='group')};
        emit('sortedLookup',new Map<String,String>{'asc'=>invoke('OrchardSortedAsc',new Case(Subject='group')).Subject,'desc'=>invoke('OrchardSortedDesc',new Case(Subject='group')).Subject});
    }
    @IsTest static void relationship(){
        Case c=new Case(Subject='seed',Status='New',Origin='Phone');insert c;
        String expected=[SELECT LastName FROM User WHERE Id=:UserInfo.getUserId()].LastName;
        emit('relationship',new Map<String,String>{'matches'=>String.valueOf(expected==invoke('OrchardRelationship',c).Subject)});
    }
    @IsTest static void person(){
        Id rt=[SELECT Id FROM RecordType WHERE SObjectType='Account' AND IsPersonType=true LIMIT 1].Id;
        Account a=new Account(RecordTypeId=rt,LastName='Garden',PersonMobilePhone='5550101');insert a;
        a=[SELECT PersonContactId FROM Account WHERE Id=:a.Id];
        Case c=new Case(Subject='person',Status='New',Origin='Phone',AccountId=a.Id);insert c;
        invoke('OrchardPerson',c);c=[SELECT ContactId,SuppliedPhone FROM Case WHERE Id=:c.Id];
        emit('person',new Map<String,String>{'contact'=>String.valueOf(c.ContactId==a.PersonContactId),'phone'=>c.SuppliedPhone});
    }
    @IsTest static void counter(){
        Task t=new Task(Subject='counter');insert t;
        Invocable.Action.Result r=action('OrchardCounter',t,new Map<String,Object>{'count'=>2});
        String first=[SELECT Description FROM Task WHERE Id=:t.Id].Description;
        action('OrchardCounter',t,null);
        emit('counter',new Map<String,String>{'first'=>first,'default'=>[SELECT Description FROM Task WHERE Id=:t.Id].Description,'output'=>String.valueOf(r.getOutputParameters().get('count'))});
    }
    @IsTest static void emptyPicklist(){
        Case c=new Case(ObservationTags__c='');
        emit('emptyPicklist',new Map<String,String>{'isNull'=>String.valueOf(c.ObservationTags__c==null),'length'=>String.valueOf(c.ObservationTags__c==null ? -1 : c.ObservationTags__c.length())});
    }
    @IsTest static void queriedRecordType(){
        Id rt=Case.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Observation').getRecordTypeId();
        Case c=new Case(Subject='rt',Status='New',Origin='Phone',RecordTypeId=rt);insert as system c;
        c=[SELECT Id,Subject,RecordTypeId FROM Case WHERE Id=:c.Id];
        Case out=invoke('OrchardIdentity',c);
        emit('queriedRecordType',new Map<String,String>{'retained'=>String.valueOf(out.RecordTypeId==rt),'input'=>String.valueOf(c.RecordTypeId==rt)});
    }
    static User user(String tag){
        Id profile=[SELECT Id FROM Profile WHERE Name='Orchard Reader'].Id;
        return new User(ProfileId=profile,Alias=tag,Email=tag+'@example.com',LastName=tag,Username=tag+System.currentTimeMillis()+'@example.com',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
    }
    @IsTest static void userFields(){
        User u=user('reader');insert u;Map<String,String> rows=new Map<String,String>();
        System.runAs(u){Map<String,Schema.SObjectField> fields=User.SObjectType.getDescribe().fields.getMap();for(String name:new List<String>{'Name','Email','Phone','Alias','IsActive','LastName','ManagerId','UserRoleId','ProfileId','ObservationFlag__c'}) rows.put(name,String.valueOf(fields.get(name).getDescribe().isAccessible()));}
        emit('userFields',rows);
    }
    @IsTest static void deniedDML(){
        User a=user('owner');User b=user('other');insert new List<User>{a,b};Parcel__c p=new Parcel__c(Name='before');
        System.runAs(a){insert p;}
        String message='allowed';System.runAs(b){try{update new Parcel__c(Id=p.Id,Name='after');}catch(DmlException e){message=e.getMessage().replace(String.valueOf(p.Id),'<ID>').replace(String.valueOf(p.Id).substring(0,15),'<ID>');}}
        emit('deniedDML',new Map<String,String>{'message'=>message});
    }
    @IsTest static void reentry(){
        nested=true;Case c=new Case(Subject='nest',Status='New',Origin='Phone');insert c;nested=false;
        emit('reentry',new Map<String,String>{'updates'=>String.valueOf(reentries),'description'=>[SELECT Description FROM Case WHERE Id=:c.Id].Description});
    }
}
