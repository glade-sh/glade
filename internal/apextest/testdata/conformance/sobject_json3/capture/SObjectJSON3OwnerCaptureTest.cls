@IsTest private class SObjectJSON3OwnerCaptureTest {
private static User makeUser() {
 User u=new User(Alias='sj3test',Email='sj3@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='fr',LocaleSidKey='fr_FR',ProfileId=UserInfo.getProfileId(),TimeZoneSidKey='Europe/Paris',Username='sobjjson3.owner@example.invalid'); insert u; return u;
}
@IsTest static void isolated() { System.assert(false,'SOBJECT_OWNER|isolated|'+JSON.serialize(SObjectJSON3Probe.observeOwner())); }
@IsTest(SeeAllData=true) static void seeAll() { System.assert(false,'SOBJECT_OWNER|seeAll|'+JSON.serialize(SObjectJSON3Probe.observeOwner())); }
@IsTest static void runAsIsolated() { User u=makeUser(); System.runAs(u) { System.assert(false,'SOBJECT_OWNER|runAsIsolated|'+JSON.serialize(SObjectJSON3Probe.observeOwner())); } }
@IsTest(SeeAllData=true) static void runAsSeeAll() { User u=makeUser(); System.runAs(u) { System.assert(false,'SOBJECT_OWNER|runAsSeeAll|'+JSON.serialize(SObjectJSON3Probe.observeOwner())); } }
}
