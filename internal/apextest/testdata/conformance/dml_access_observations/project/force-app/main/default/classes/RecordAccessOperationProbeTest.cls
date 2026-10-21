@IsTest public class RecordAccessOperationProbeTest {
    @TestSetup static void seed() {
        Profile profile=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
        insert new User(Username='access.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='access',Email='access@example.invalid',LastName='SyntheticAccessReader',ProfileId=profile.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
    }
    @IsTest static void userUpsert() { observe('userUpsert'); }
    @IsTest static void userUndelete() { observe('userUndelete'); }
    @IsTest static void systemUpdate() { observe('systemUpdate'); }
    @IsTest static void systemUpsert() { observe('systemUpsert'); }
    @IsTest static void systemUndelete() { observe('systemUndelete'); }
    private static void observe(String kind) {
        User actor=[SELECT Id FROM User WHERE LastName='SyntheticAccessReader' LIMIT 1];
        SObject record=kind == 'systemUpdate' || kind == 'systemUpsert' ? (SObject)new Contact(LastName='Existing') : (SObject)new Case(Subject='Existing',Status='New',Origin='Web');
        insert record;
        if (kind == 'userUndelete' || kind == 'systemUndelete') delete record;
        Map<String,Object> row;
        System.runAs(actor) { row=RecordAccessOperationProbe.observe(kind,record); }
        System.assert(false,'OBSERVED|'+kind+'|'+JSON.serialize(row));
    }
}
