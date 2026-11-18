@IsTest private class PfS62 {
  static final Map<String,String> PS = new Map<String,String>{'F' => 'Pf_Field', 'O' => 'Pf_Object', 'C' => 'Pf_Cust'};
  static void fin(String s) { System.assert(false, 'RESULT => ' + s); }
  static User newStd() {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    String k = String.valueOf(Math.abs(Crypto.getRandomInteger()));
    return new User(Alias = 'pf' + k.left(5), Email = 'pf@example.invalid', EmailEncodingKey = 'UTF-8', LastName = 'PfU', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'pf' + k + '@example.invalid.glade');
  }
  static User mkU() {
    User u = newStd();
    System.runAs(new User(Id = UserInfo.getUserId())) { insert u; }
    return u;
  }
  static Id psId(String k) { return [SELECT Id FROM PermissionSet WHERE Name = :PS.get(k)].Id; }
  static void asg(Id uid, String k) { insert new PermissionSetAssignment(AssigneeId = uid, PermissionSetId = psId(k)); }
  static void unasg(Id uid, String k) { Id p = psId(k); delete [SELECT Id FROM PermissionSetAssignment WHERE AssigneeId = :uid AND PermissionSetId = :p]; }
  static Id me() { return UserInfo.getUserId(); }
  static String ex(Exception e) { return e.getTypeName() + ' ' + e.getMessage().left(50); }
  static String rd(String k, Boolean dml) {
    String s = '';
    if (k == 'F') {
      s += 'cr=' + Pf_Fld__c.Secret__c.getDescribe().isCreateable() + ' rd=' + Pf_Fld__c.Secret__c.getDescribe().isAccessible();
      try { List<Pf_Fld__c> l = [SELECT Secret__c FROM Pf_Fld__c WITH USER_MODE LIMIT 1]; s += ' q=ok'; } catch (Exception e) { s += ' q=' + e.getTypeName(); }
      if (dml) { try { insert as user new Pf_Fld__c(Name = 'n', Secret__c = 's'); s += ' ins=ok'; } catch (Exception e) { s += ' ins=' + ex(e); } }
    } else if (k == 'O') {
      s += 'cr=' + Pf_Obj__c.SObjectType.getDescribe().isCreateable() + ' rd=' + Pf_Obj__c.SObjectType.getDescribe().isAccessible();
      try { List<Pf_Obj__c> l = [SELECT Id FROM Pf_Obj__c WITH USER_MODE LIMIT 1]; s += ' q=ok'; } catch (Exception e) { s += ' q=' + e.getTypeName(); }
      if (dml) { try { insert as user new Pf_Obj__c(Name = 'n'); s += ' ins=ok'; } catch (Exception e) { s += ' ins=' + ex(e); } }
    } else {
      s += 'perm=' + FeatureManagement.checkPermission('Pf_Cust');
    }
    return s;
  }
  static String inSelf(String k, Boolean dml) { String s; System.runAs(new User(Id = me())) { s = rd(k, dml); } return s; }
  static String inU(User u, String k, Boolean dml) { String s; System.runAs(u) { s = rd(k, dml); } return s; }
  @TestSetup static void ts() { User u = mkU(); for (String k : new List<String>{'F','O','C'}) { asg(me(), k); System.runAs(new User(Id = me())) { asg(u.Id, k); } } }
  @IsTest static void s01_plain_f() { String K = 'F'; User u = [SELECT Id FROM User WHERE LastName = 'PfU' AND Alias LIKE 'pf%' LIMIT 1]; String a = rd(K, false); String b = inSelf(K, false); String c = inU(u, K, false); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void s01_plain_o() { String K = 'O'; User u = [SELECT Id FROM User WHERE LastName = 'PfU' AND Alias LIKE 'pf%' LIMIT 1]; String a = rd(K, false); String b = inSelf(K, false); String c = inU(u, K, false); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void s01_plain_c() { String K = 'C'; User u = [SELECT Id FROM User WHERE LastName = 'PfU' AND Alias LIKE 'pf%' LIMIT 1]; String a = rd(K, false); String b = inSelf(K, false); String c = inU(u, K, false); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void s02_dml_f() { String K = 'F'; User u = [SELECT Id FROM User WHERE LastName = 'PfU' AND Alias LIKE 'pf%' LIMIT 1]; String a = rd(K, true); String b = inSelf(K, true); String c = inU(u, K, true); String d = rd(K, true); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void s02_dml_o() { String K = 'O'; User u = [SELECT Id FROM User WHERE LastName = 'PfU' AND Alias LIKE 'pf%' LIMIT 1]; String a = rd(K, true); String b = inSelf(K, true); String c = inU(u, K, true); String d = rd(K, true); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void s02_dml_c() { String K = 'C'; User u = [SELECT Id FROM User WHERE LastName = 'PfU' AND Alias LIKE 'pf%' LIMIT 1]; String a = rd(K, true); String b = inSelf(K, true); String c = inU(u, K, true); String d = rd(K, true); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void s03_start_f() { String K = 'F'; Test.startTest(); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d); }
  @IsTest static void s03_start_o() { String K = 'O'; Test.startTest(); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d); }
  @IsTest static void s03_start_c() { String K = 'C'; Test.startTest(); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d); }
  @IsTest static void s04_unassign_f() { String K = 'F'; String b = inSelf(K, false); unasg(me(), K); String a = rd(K, false); String c = inSelf(K, false); fin('B=' + b + ' | A=' + a + ' | C=' + c); }
  @IsTest static void s04_unassign_o() { String K = 'O'; String b = inSelf(K, false); unasg(me(), K); String a = rd(K, false); String c = inSelf(K, false); fin('B=' + b + ' | A=' + a + ' | C=' + c); }
  @IsTest static void s04_unassign_c() { String K = 'C'; String b = inSelf(K, false); unasg(me(), K); String a = rd(K, false); String c = inSelf(K, false); fin('B=' + b + ' | A=' + a + ' | C=' + c); }

}
