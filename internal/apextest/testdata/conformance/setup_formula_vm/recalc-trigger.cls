@IsTest private class SetupFormulaVMProbeTest {
 @IsTest static void actor() {
  User actor=new User(Alias='svprobe', Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=UserInfo.getProfileId(), Username='synthetic.vm.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=false);
   Account a=new Account(Name='Probe');
   List<FormulaRecalcResult> r=Formula.recalculateFormulas(new List<SObject>{a});
   Object recalculated=a.get('SetupFlag__c');
   insert a;
   Boolean triggerValue=[SELECT TriggerValue__c FROM Account WHERE Id=:a.Id].TriggerValue__c;
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,Object>{'recalcSuccess'=>r[0].isSuccess(),'recalc'=>recalculated,'trigger'=>triggerValue}));
  }
 }
}
