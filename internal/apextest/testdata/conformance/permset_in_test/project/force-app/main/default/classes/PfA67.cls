@IsTest private class PfA67 {
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
  @IsTest static void v01_direct_f() { String K = 'F'; User u = mkU(); System.runAs(new User(Id = me())) { asg(u.Id, K); } asg(me(), K); String a = rd(K, false); String b = inSelf(K, false); String c = inU(u, K, false); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void v01_direct_o() { String K = 'O'; User u = mkU(); System.runAs(new User(Id = me())) { asg(u.Id, K); } asg(me(), K); String a = rd(K, false); String b = inSelf(K, false); String c = inU(u, K, false); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void v01_direct_c() { String K = 'C'; User u = mkU(); System.runAs(new User(Id = me())) { asg(u.Id, K); } asg(me(), K); String a = rd(K, false); String b = inSelf(K, false); String c = inU(u, K, false); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void v02_inrunas_f() { String K = 'F'; User u = mkU(); String x; System.runAs(new User(Id = me())) { asg(u.Id, K); asg(me(), K); x = rd(K, true); } String a = rd(K, true); String b = inSelf(K, true); String c = inU(u, K, true); String d = rd(K, true); fin('X=' + x + ' | A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void v02_inrunas_o() { String K = 'O'; User u = mkU(); String x; System.runAs(new User(Id = me())) { asg(u.Id, K); asg(me(), K); x = rd(K, true); } String a = rd(K, true); String b = inSelf(K, true); String c = inU(u, K, true); String d = rd(K, true); fin('X=' + x + ' | A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void v02_inrunas_c() { String K = 'C'; User u = mkU(); String x; System.runAs(new User(Id = me())) { asg(u.Id, K); asg(me(), K); x = rd(K, true); } String a = rd(K, true); String b = inSelf(K, true); String c = inU(u, K, true); String d = rd(K, true); fin('X=' + x + ' | A=' + a + ' | B=' + b + ' | C=' + c + ' | D=' + d); }
  @IsTest static void v03_start_after_f() { String K = 'F'; asg(me(), K); Test.startTest(); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d); }
  @IsTest static void v03_start_after_o() { String K = 'O'; asg(me(), K); Test.startTest(); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d); }
  @IsTest static void v03_start_after_c() { String K = 'C'; asg(me(), K); Test.startTest(); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d); }
  @IsTest static void v04_start_inside_f() { String K = 'F'; Test.startTest(); asg(me(), K); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); String e = inSelf(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d + ' | E=' + e); }
  @IsTest static void v04_start_inside_o() { String K = 'O'; Test.startTest(); asg(me(), K); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); String e = inSelf(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d + ' | E=' + e); }
  @IsTest static void v04_start_inside_c() { String K = 'C'; Test.startTest(); asg(me(), K); String a = rd(K, false); String b = inSelf(K, false); Test.stopTest(); String d = rd(K, false); String e = inSelf(K, false); fin('A=' + a + ' | B=' + b + ' | D=' + d + ' | E=' + e); }
  @IsTest static void v05_nested_f() { String K = 'F'; asg(me(), K); String s1; String s2; String s3; System.runAs(new User(Id = me())) { s1 = rd(K, false); System.runAs(new User(Id = me())) { s2 = rd(K, false); } s3 = rd(K, false); } String o = rd(K, false); fin('s1=' + s1 + ' | s2=' + s2 + ' | s3=' + s3 + ' | o=' + o); }
  @IsTest static void v05_nested_o() { String K = 'O'; asg(me(), K); String s1; String s2; String s3; System.runAs(new User(Id = me())) { s1 = rd(K, false); System.runAs(new User(Id = me())) { s2 = rd(K, false); } s3 = rd(K, false); } String o = rd(K, false); fin('s1=' + s1 + ' | s2=' + s2 + ' | s3=' + s3 + ' | o=' + o); }
  @IsTest static void v05_nested_c() { String K = 'C'; asg(me(), K); String s1; String s2; String s3; System.runAs(new User(Id = me())) { s1 = rd(K, false); System.runAs(new User(Id = me())) { s2 = rd(K, false); } s3 = rd(K, false); } String o = rd(K, false); fin('s1=' + s1 + ' | s2=' + s2 + ' | s3=' + s3 + ' | o=' + o); }
  @IsTest static void v06_unassign_f() { String K = 'F'; asg(me(), K); String b = inSelf(K, false); unasg(me(), K); String a = rd(K, false); String c = inSelf(K, false); fin('B=' + b + ' | A=' + a + ' | C=' + c); }
  @IsTest static void v06_unassign_o() { String K = 'O'; asg(me(), K); String b = inSelf(K, false); unasg(me(), K); String a = rd(K, false); String c = inSelf(K, false); fin('B=' + b + ' | A=' + a + ' | C=' + c); }
  @IsTest static void v06_unassign_c() { String K = 'C'; asg(me(), K); String b = inSelf(K, false); unasg(me(), K); String a = rd(K, false); String c = inSelf(K, false); fin('B=' + b + ' | A=' + a + ' | C=' + c); }
  @IsTest static void v07_u_nested_f() { String K = 'F'; User u = mkU(); String x; System.runAs(new User(Id = me())) { asg(u.Id, K); x = inU(u, K, false); } String y = inU(u, K, false); fin('X=' + x + ' | Y=' + y); }
  @IsTest static void v07_u_nested_o() { String K = 'O'; User u = mkU(); String x; System.runAs(new User(Id = me())) { asg(u.Id, K); x = inU(u, K, false); } String y = inU(u, K, false); fin('X=' + x + ' | Y=' + y); }
  @IsTest static void v07_u_nested_c() { String K = 'C'; User u = mkU(); String x; System.runAs(new User(Id = me())) { asg(u.Id, K); x = inU(u, K, false); } String y = inU(u, K, false); fin('X=' + x + ' | Y=' + y); }
  @IsTest static void v08_u_direct_f() { String K = 'F'; User u = mkU(); asg(u.Id, K); String c = inU(u, K, false); fin('C=' + c); }
  @IsTest static void v08_u_direct_o() { String K = 'O'; User u = mkU(); asg(u.Id, K); String c = inU(u, K, false); fin('C=' + c); }
  @IsTest static void v08_u_direct_c() { String K = 'C'; User u = mkU(); asg(u.Id, K); String c = inU(u, K, false); fin('C=' + c); }
  @IsTest static void v09_two_blocks_f() { String K = 'F'; asg(me(), K); String b1 = inSelf(K, false); String b2 = inSelf(K, false); fin('B1=' + b1 + ' | B2=' + b2); }
  @IsTest static void v09_two_blocks_o() { String K = 'O'; asg(me(), K); String b1 = inSelf(K, false); String b2 = inSelf(K, false); fin('B1=' + b1 + ' | B2=' + b2); }
  @IsTest static void v09_two_blocks_c() { String K = 'C'; asg(me(), K); String b1 = inSelf(K, false); String b2 = inSelf(K, false); fin('B1=' + b1 + ' | B2=' + b2); }
  @IsTest static void v10_u_after_self_f() { String K = 'F'; User u = mkU(); asg(me(), K); asg(u.Id, K); String c = inU(u, K, false); String b = inSelf(K, false); String c2 = inU(u, K, false); fin('C=' + c + ' | B=' + b + ' | C2=' + c2); }
  @IsTest static void v10_u_after_self_o() { String K = 'O'; User u = mkU(); asg(me(), K); asg(u.Id, K); String c = inU(u, K, false); String b = inSelf(K, false); String c2 = inU(u, K, false); fin('C=' + c + ' | B=' + b + ' | C2=' + c2); }
  @IsTest static void v10_u_after_self_c() { String K = 'C'; User u = mkU(); asg(me(), K); asg(u.Id, K); String c = inU(u, K, false); String b = inSelf(K, false); String c2 = inU(u, K, false); fin('C=' + c + ' | B=' + b + ' | C2=' + c2); }

}
