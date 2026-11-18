@IsTest private class Ld08HierarchyDefaultsTest {
 @IsTest static void observe() {
  delete [SELECT Id FROM HierPref__c];
  HierPref__c none=HierPref__c.getInstance();
  HierPref__c org=HierPref__c.getOrgDefaults();
  Map<String,Object> o=new Map<String,Object>{'instNull'=>none==null,'orgNull'=>org==null,
   'instValue'=>none==null?'n/a':none.Value__c,'instNum'=>none==null?'n/a':String.valueOf(none.Num__c),
   'instFlag'=>none==null?'n/a':String.valueOf(none.Flag__c),'instNote'=>none==null?'n/a':none.Note__c,
   'instIdNull'=>none==null?true:none.Id==null,'instOwnerUser'=>none==null?false:none.SetupOwnerId==UserInfo.getUserId(),
   'orgIdNull'=>org==null?true:org.Id==null,'orgOwnerOrg'=>org==null?false:org.SetupOwnerId==UserInfo.getOrganizationId(),
   'userRow'=>byId(UserInfo.getUserId()),'orgRow'=>byId(UserInfo.getOrganizationId())};
  insert new HierPref__c(Name='Partial');
  HierPref__c partial=HierPref__c.getOrgDefaults();
  o.put('partialValue',partial==null?'null':partial.Value__c); o.put('partialNum',partial==null?'null':String.valueOf(partial.Num__c));
  o.put('partialFlag',partial==null?'null':String.valueOf(partial.Flag__c)); o.put('partialNote',partial==null?'null':partial.Note__c);
  o.put('partialOwnerOrg',partial!=null && partial.SetupOwnerId==UserInfo.getOrganizationId()); o.put('partialHasId',partial!=null && partial.Id!=null);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
 static String byId(Id owner) { try { HierPref__c row=HierPref__c.getValues(owner); return row==null?'null':row.Value__c+'/'+String.valueOf(row.Num__c)+'/'+row.Flag__c; } catch(Exception e) { return e.getTypeName(); } }
}
