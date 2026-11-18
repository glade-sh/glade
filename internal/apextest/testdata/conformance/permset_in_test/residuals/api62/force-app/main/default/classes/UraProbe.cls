@IsTest private class UraProbe {
 static String ura(Id uid, Id rid) {
  try { List<UserRecordAccess> rows = [SELECT RecordId, HasReadAccess, HasEditAccess, HasDeleteAccess, HasTransferAccess, HasAllAccess, MaxAccessLevel FROM UserRecordAccess WHERE UserId = :uid AND RecordId = :rid];
   if (rows.isEmpty()) return 'none';
   return 'r=' + rows[0].HasReadAccess + ',e=' + rows[0].HasEditAccess + ',d=' + rows[0].HasDeleteAccess + ',t=' + rows[0].HasTransferAccess + ',a=' + rows[0].HasAllAccess + ',m=' + rows[0].MaxAccessLevel;
  } catch (Exception e) { return 'ex:' + e.getTypeName(); }
 }
 @IsTest static void t() {
  Id ps = [SELECT Id FROM PermissionSet WHERE Name = 'Ura_Set'].Id;
  User u = PsrUtil.newUser(PsrUtil.limitedProfileId(), 'UA', 1); System.runAs(new User(Id = UserInfo.getUserId())) { insert u; }
  List<SObject> recs = new List<SObject>{ new Ura_Priv__c(Name='p'), new Ura_Ro__c(Name='o'), new Ura_Rw__c(Name='w') };
  System.runAs(new User(Id = UserInfo.getUserId())) { insert recs; }
  String p = 'ura';
  for (SObject r : recs) p += '|' + String.valueOf(r.getSObjectType()) + ':before=' + ura(u.Id, r.Id);
  System.runAs(new User(Id = UserInfo.getUserId())) { insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps); }
  for (SObject r : recs) p += '|' + String.valueOf(r.getSObjectType()) + ':after=' + ura(u.Id, r.Id);
  System.runAs(u) {
   for (SObject r : recs) p += '|' + String.valueOf(r.getSObjectType()) + ':self=' + ura(u.Id, r.Id);
   Ura_Priv__c own = new Ura_Priv__c(Name='own'); Ura_Ro__c ownR = new Ura_Ro__c(Name='ownr'); Ura_Rw__c ownW = new Ura_Rw__c(Name='ownw');
   insert own; insert ownR; insert ownW;
   p += '|own_priv=' + ura(u.Id, own.Id) + '|own_ro=' + ura(u.Id, ownR.Id) + '|own_rw=' + ura(u.Id, ownW.Id);
  }
  System.assert(false, p);
 }
}
