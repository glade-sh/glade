@IsTest private class MeadowScheduleOwnerProbeTest {
 @IsTest static void observe() {
  Map<String,Boolean> values = MeadowScheduleOwnerProbe.observe();
  Id original = UserInfo.getUserId();
  User other = new User(FirstName='Meadow', LastName='Owner', Alias='ownprobe', Email='owner@example.invalid', Username='owner.'+UserInfo.getOrganizationId()+'.'+Datetime.now().getTime()+'@example.invalid', ProfileId=UserInfo.getProfileId(), TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US');
  insert other;
  User expected = [SELECT Name, Profile.Name FROM User WHERE Id=:other.Id];
  Id scheduled;
  try {
   System.runAs(other) {
    scheduled = System.schedule('Meadow run-as owner', '0 0 0 1 1 ? 2099', new MeadowScheduleOwnerProbe());
    CronTrigger row = [SELECT OwnerId FROM CronTrigger WHERE Id=:scheduled];
    values.put('SR01', row.OwnerId==other.Id);
   }
   CronTrigger row = [SELECT OwnerId FROM CronTrigger WHERE Id=:scheduled];
   List<User> owners = [SELECT Name, Profile.Name FROM User WHERE Id=:row.OwnerId];
   values.put('SR02', row.OwnerId==other.Id && owners.size()==1 && owners[0].Name==expected.Name && owners[0].Profile.Name==expected.Profile.Name);
   values.put('SC04', UserInfo.getUserId()==original);
  } finally { if(scheduled!=null) System.abortJob(scheduled); }
  System.assert(false, 'PROBE|'+JSON.serialize(values));
 }
}
