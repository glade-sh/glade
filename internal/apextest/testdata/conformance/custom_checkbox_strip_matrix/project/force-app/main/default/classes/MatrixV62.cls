@IsTest private class MatrixV62 {
 static String sorted(Set<String> s) {List<String> v=new List<String>(s);v.sort();return String.join(v,',');}
 static void run(String profile,String grant) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:profile LIMIT 1];
  User u=new User(Alias='mxprobe',Email='matrix@example.test',EmailEncodingKey='UTF-8',LastName='Matrix',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName='matrix'+System.now().getTime()+profile.length()+grant+'@example.test');
  insert u;
  Set<String> names=new Set<String>{'MatrixObject'};if(grant=='read')names.add('MatrixRead');if(grant=='edit')names.add('MatrixEdit');
  List<PermissionSetAssignment> assigns=new List<PermissionSetAssignment>();for(PermissionSet ps:[SELECT Id FROM PermissionSet WHERE Name IN :names])assigns.add(new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id));insert assigns;
  String out='';
  System.runAs(u) {
   for(String route:new List<String>{'fresh','explicit','loaded','unloaded'}) {
    SObject row;
    if(route=='fresh')row=new AccessMatrix__c(Name='Example');
    else if(route=='explicit')row=new AccessMatrix__c(Name='Example',Flag__c=false,Mark__c=false);
    else row=AccessMatrix__c.SObjectType.newSObject(null,route=='loaded');
    String prefix=route+'|pop='+sorted(row.getPopulatedFieldsAsMap().keySet());
    try {SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{row},true);Set<String> removed=d.getRemovedFields().get('AccessMatrix__c');out+=prefix+'|removed='+(removed==null?'':sorted(removed))+'\n';}
    catch(Exception e){out+=prefix+'|throw='+e.getTypeName()+'\n';}
   }
   out+='describe|flagRead='+AccessMatrix__c.Flag__c.getDescribe().isAccessible()+'|flagCreate='+AccessMatrix__c.Flag__c.getDescribe().isCreateable()+'|markRead='+AccessMatrix__c.Mark__c.getDescribe().isAccessible()+'|markCreate='+AccessMatrix__c.Mark__c.getDescribe().isCreateable()+'\n';
  }
  System.assert(false,'CAPTURE\n'+out);
 }
 @IsTest static void adminNone(){run('System Administrator','none');}
 @IsTest static void adminRead(){run('System Administrator','read');}
 @IsTest static void adminEdit(){run('System Administrator','edit');}
 @IsTest static void standardNone(){run('Standard User','none');}
 @IsTest static void standardRead(){run('Standard User','read');}
 @IsTest static void standardEdit(){run('Standard User','edit');}
}
