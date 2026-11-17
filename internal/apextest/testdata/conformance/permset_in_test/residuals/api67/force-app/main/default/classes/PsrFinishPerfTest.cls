@IsTest private class PsrFinishPerfTest {
 @TestSetup static void setup() {
  List<User> us = new List<User>();
  Id profileId = PsrUtil.limitedProfileId();
  for (Integer i=0;i<100;i++) us.add(PsrUtil.newUser(profileId,'T',i));
  insert as system us;
  List<PermissionSet> sets = new List<PermissionSet>();
  for (Integer i=0;i<50;i++) sets.add(new PermissionSet(Name='PsrBulk'+i, Label='Psr Bulk '+i));
  insert as system sets;
 }
 @IsTest static void timing() {
  List<User> us = [SELECT Id FROM User WHERE Alias LIKE 'psrT%' LIMIT 100];
  List<PermissionSet> sets = [SELECT Id FROM PermissionSet WHERE Name LIKE 'PsrBulk%' LIMIT 50];
  List<PermissionSetAssignment> rows = new List<PermissionSetAssignment>();
  for (User u:us) for (PermissionSet p:sets) rows.add(new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=p.Id));
  Long start=System.now().getTime();
  Database.SaveResult[] rs=Database.insert(rows,false,AccessLevel.SYSTEM_MODE);
  Long elapsed=System.now().getTime()-start; Integer ok=0; String err='';
  for (Database.SaveResult r:rs) { if (r.isSuccess()) ok++; else if (err=='') err=String.valueOf(r.getErrors()[0].getStatusCode()); }
  System.assert(false,'users='+us.size()+';sets='+sets.size()+';rows='+rows.size()+';ok='+ok+';ms='+elapsed+';err='+err);
 }
}
