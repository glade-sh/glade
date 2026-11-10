@IsTest private class ShapeV62 {
 static User actor(String profile, Boolean marketing) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:profile LIMIT 1];
  return new User(Alias='shprobe',Email='shape@example.test',EmailEncodingKey='UTF-8',LastName='Shape',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName='v036shape'+System.now().getTime()+profile.length()+'@example.test',UserPermissionsMarketingUser=marketing);
 }
 static void run(String profile, Boolean marketing) {
  Map<String,List<String>> specs=new Map<String,List<String>>{
   'Contact'=>new List<String>{'IsPriorityRecord'},'Lead'=>new List<String>{'IsPriorityRecord'},
   'Product2'=>new List<String>{'IsSerialized','TransferRecordMode','ProductClass'},
   'Campaign'=>new List<String>{'ExpectedResponse','NumberSent'},
   'Case'=>new List<String>{'ClosedOnCreate','IsEscalated','IsStopped'},
   'Task'=>new List<String>{'IsRecurrence2','IsRecurrence2Exception','IsRecurrence2Exclusion'},
   'Pricebook2'=>new List<String>{'IsStandard'}
  };
  String out='';
  System.runAs(actor(profile,marketing)) {
   for (String objName:specs.keySet()) {
    Schema.SObjectType typ=Schema.getGlobalDescribe().get(objName);SObject obj=typ.newSObject(null,true);
    Map<String,Schema.SObjectField> fields=typ.getDescribe().fields.getMap();
    out+=objName+'|create='+typ.getDescribe().isCreateable()+'\n';
    for (String f:specs.get(objName)) {
     out+=objName+'.'+f+'|exists='+fields.containsKey(f);
     if(fields.containsKey(f)) {Schema.DescribeFieldResult d=fields.get(f).getDescribe();out+='|read='+d.isAccessible()+'|create='+d.isCreateable()+'|default='+d.isDefaultedOnCreate()+'|pop='+obj.getPopulatedFieldsAsMap().containsKey(f);}
     out+='\n';
    }
   }
  }
  System.assert(false,'CAPTURE\n'+out);
 }
 @IsTest static void admin() {run('System Administrator',false);}
 @IsTest static void adminMarketing() {run('System Administrator',true);}
 @IsTest static void standard() {run('Standard User',false);}
 @IsTest static void standardMarketing() {run('Standard User',true);}
}
