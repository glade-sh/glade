@IsTest public class SetupRunAsVisibilityTest {
    @TestSetup static void seed() {
        System.runAs(new User(Id=UserInfo.getUserId())) {
            Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
            User actor=new User(Username='runas.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='rnas',Email='runas@example.invalid',LastName='SyntheticRunAs',ProfileId=profile.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
            insert actor;
            PermissionSet grant=[SELECT Id FROM PermissionSet WHERE Name='SyntheticRunAsAccess' LIMIT 1];
            insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=grant.Id);
        }
        insert new RunAsPriv__c(Name='AdminSeed');
        insert new RunAsOpen__c(Name='AdminSeed');
        System.runAs(actor()) { insert new RunAsPriv__c(Name='UserSeed'); }
    }
    static User actor() { return [SELECT Id FROM User WHERE LastName='SyntheticRunAs' LIMIT 1]; }
    static Map<String,Object> counts() {
        return new Map<String,Object>{
            'privAdmin'=>[SELECT COUNT() FROM RunAsPriv__c WHERE Name='AdminSeed'],
            'privUser'=>[SELECT COUNT() FROM RunAsPriv__c WHERE Name='UserSeed'],
            'open'=>[SELECT COUNT() FROM RunAsOpen__c WHERE Name='AdminSeed']
        };
    }
    static void report(String method, Map<String,Object> row) { System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(row)); }
    @IsTest static void outside() { report('outside',counts()); }
    @IsTest static void inside() {
        Map<String,Object> row=new Map<String,Object>();
        System.runAs(actor()) { row=counts(); }
        report('inside',row);
    }
    @IsTest static void singleton() {
        Map<String,Object> row=new Map<String,Object>();
        System.runAs(actor()) {
            try { RunAsPriv__c r=[SELECT Id FROM RunAsPriv__c WHERE Name='AdminSeed' LIMIT 1]; row.put('privAdmin','row'); } catch (Exception e) { row.put('privAdmin',e.getTypeName()+':'+e.getMessage()); }
            try { RunAsPriv__c r=[SELECT Id FROM RunAsPriv__c WHERE Name='UserSeed' LIMIT 1]; row.put('privUser','row'); } catch (Exception e) { row.put('privUser',e.getTypeName()+':'+e.getMessage()); }
            try { RunAsOpen__c r=[SELECT Id FROM RunAsOpen__c WHERE Name='AdminSeed' LIMIT 1]; row.put('open','row'); } catch (Exception e) { row.put('open',e.getTypeName()+':'+e.getMessage()); }
            try { RunAsPriv__c r=[SELECT Id FROM RunAsPriv__c LIMIT 1]; row.put('anyPriv','row'); } catch (Exception e) { row.put('anyPriv',e.getTypeName()+':'+e.getMessage()); }
        }
        report('singleton',row);
    }
    @IsTest static void helpers() {
        Map<String,Object> row=new Map<String,Object>{'outsideWith'=>RunAsWithSharing.countPriv(),'outsideWithout'=>RunAsWithoutSharing.countPriv(),'outsideInherited'=>RunAsInherited.countPriv()};
        System.runAs(actor()) {
            row.put('insideWith',RunAsWithSharing.countPriv());
            row.put('insideWithout',RunAsWithoutSharing.countPriv());
            row.put('insideInherited',RunAsInherited.countPriv());
        }
        report('helpers',row);
    }
    @IsTest static void beforeAfter() {
        Map<String,Object> row=new Map<String,Object>{'before'=>counts()};
        User u=actor();
        System.runAs(u) { row.put('inside',counts()); }
        row.put('after',counts());
        report('beforeAfter',row);
    }
    @IsTest static void insideInsert() {
        Map<String,Object> row=new Map<String,Object>();
        System.runAs(actor()) {
            insert new RunAsPriv__c(Name='InnerSeed');
            row.put('insideInner',[SELECT COUNT() FROM RunAsPriv__c WHERE Name='InnerSeed']);
        }
        row.put('outsideInner',[SELECT COUNT() FROM RunAsPriv__c WHERE Name='InnerSeed']);
        report('insideInsert',row);
    }
    @IsTest static void ownerTransfer() {
        User u=actor();
        RunAsPriv__c r=[SELECT Id FROM RunAsPriv__c WHERE Name='AdminSeed' LIMIT 1];
        r.OwnerId=u.Id; update r;
        Map<String,Object> row=new Map<String,Object>();
        System.runAs(u) { row=counts(); }
        report('ownerTransfer',row);
    }
    @IsTest static void manualShare() {
        User u=actor();
        RunAsPriv__c r=[SELECT Id FROM RunAsPriv__c WHERE Name='AdminSeed' LIMIT 1];
        insert new RunAsPriv__Share(ParentId=r.Id,UserOrGroupId=u.Id,AccessLevel='Read',RowCause='Manual');
        Map<String,Object> row=new Map<String,Object>();
        System.runAs(u) { row=counts(); }
        report('manualShare',row);
    }
    @IsTest static void userLookup() {
        User u=actor();
        Map<String,Object> row=new Map<String,Object>{'outside'=>[SELECT COUNT() FROM User WHERE LastName='SyntheticRunAs']};
        System.runAs(u) {
            row.put('inside',[SELECT COUNT() FROM User WHERE LastName='SyntheticRunAs']);
            row.put('self',UserInfo.getUserId()==u.Id);
            row.put('selfLookup',[SELECT COUNT() FROM User WHERE Id=:UserInfo.getUserId()]);
        }
        report('userLookup',row);
    }
    @IsTest static void nested() {
        User u=actor();
        Id adminId=UserInfo.getUserId();
        Map<String,Object> row=new Map<String,Object>();
        System.runAs(u) {
            System.runAs(new User(Id=adminId)) { row.put('same',counts()); }
            row.put('afterInner',counts());
        }
        report('nested',row);
    }
}
