@IsTest private class HiddenV62 {
 static User actor(String profile) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:profile LIMIT 1];
  return new User(Alias='hdprobe',Email='hidden@example.test',EmailEncodingKey='UTF-8',LastName='Hidden',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName='hidden'+System.now().getTime()+profile.length()+'@example.test');
 }
 static String sorted(Set<String> s) {List<String> v=new List<String>(s);v.sort();return String.join(v,',');}
 static void run(String profile) {
  String out='';
  System.runAs(actor(profile)) {
   for (String name:new List<String>{'Case','Task'}) {
    Schema.SObjectType typ=Schema.getGlobalDescribe().get(name);
    for(String route:new List<String>{'plain','false','true','changed','cloned'}) {
     SObject row;
     if(route=='plain') { if(name=='Case') row=new Case(); else row=new Task(); }
     else if(route=='false') row=typ.newSObject(null,false);
     else { row=typ.newSObject(null,true); if(route=='changed') row.put('Subject','Synthetic'); if(route=='cloned') row=row.clone(false,true,false,false); }
     try { SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{row},true); Set<String> removed=d.getRemovedFields().get(name);out+=name+'.'+route+'|removed='+(removed==null?'':sorted(removed))+'\n'; }
     catch(Exception e) {out+=name+'.'+route+'|throw='+e.getTypeName()+'\n';}
    }
   }
  }
  System.assert(false,'CAPTURE\n'+out);
 }
 @IsTest static void admin() {run('System Administrator');}
 @IsTest static void standard() {run('Standard User');}
}
