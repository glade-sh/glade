@IsTest private class PublicLead67 {
 @IsTest static void publicDefault() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u=new User(LastName='Duplicate Reader',Alias='dupread',Email='reader@example.invalid',Username='publiclead'+String.valueOf(DateTime.now().getTime())+'@example.invalid',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u;
  System.runAs(new User(Id=UserInfo.getUserId())) { insert new Lead(LastName='Seed',Company='Synthetic',Email='publiclead@example.invalid'); }
  System.runAs(u) {
   System.assertEquals(1,[SELECT count() FROM Lead WHERE Email='publiclead@example.invalid'],'public default visibility');
   Database.SaveResult fresh=Database.insert(new Lead(LastName='Fresh',Company='Synthetic',Email='fresh@example.invalid'),false); System.assert(fresh.isSuccess(),'nonmatch saves');
   Database.SaveResult dup=Database.insert(new Lead(LastName='Candidate',Company='Synthetic',Email='publiclead@example.invalid'),false); System.assertEquals(false,dup.isSuccess()); System.assertEquals(StatusCode.DUPLICATES_DETECTED,dup.getErrors()[0].getStatusCode());
   Database.SaveResult blank=Database.insert(new Lead(LastName='Blank',Company='Synthetic'),false); System.assert(blank.isSuccess(),'blank boundary saves');
   Boolean blocked=false; try { insert new Lead(LastName='Statement',Company='Synthetic',Email='publiclead@example.invalid'); } catch(DmlException e) { blocked=e.getDmlStatusCode(0)=='DUPLICATES_DETECTED'; } System.assert(blocked,'statement duplicate blocked');
  }
 }
}