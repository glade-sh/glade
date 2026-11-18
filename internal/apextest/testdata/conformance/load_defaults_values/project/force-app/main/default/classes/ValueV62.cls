@IsTest private class ValueV62 {
 static void run(String profile,Boolean marketing) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:profile LIMIT 1];
  User u=new User(Alias='valprob',Email='value@example.test',EmailEncodingKey='UTF-8',LastName='Value',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName='value'+System.now().getTime()+profile.length()+'@example.test',UserPermissionsMarketingUser=marketing);
  String out='';
  System.runAs(u) {
   Map<String,List<String>> specs=new Map<String,List<String>>{'Product2'=>new List<String>{'IsSerialized','TransferRecordMode'},'Campaign'=>new List<String>{'ExpectedResponse','NumberSent'}};
   for(String name:specs.keySet()) {
    SObject loaded=Schema.getGlobalDescribe().get(name).newSObject(null,true);
    SObject blank=Schema.getGlobalDescribe().get(name).newSObject(null,false);
    for(String field:specs.get(name)) out+=name+'.'+field+'|loaded='+JSON.serialize(loaded.get(field))+'|loadedPop='+loaded.getPopulatedFieldsAsMap().containsKey(field)+'|blank='+JSON.serialize(blank.get(field))+'|blankPop='+blank.getPopulatedFieldsAsMap().containsKey(field)+'\n';
   }
  }
  System.assert(false,'CAPTURE\n'+out);
 }
 @IsTest static void admin() {run('System Administrator',false);}
 @IsTest static void adminMarketing() {run('System Administrator',true);}
 @IsTest static void standard() {run('Standard User',false);}
}
