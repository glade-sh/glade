@IsTest private class GlSl67 {
  static User mkUser() {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    String un = 'gl' + String.valueOf(Crypto.getRandomInteger()).replace('-','') + '@example.com';
    User u = new User(Alias='gl', Email=un, EmailEncodingKey='UTF-8', LastName='GlStd', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', Username=un, ProfileId=p.Id);
    PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'Gl_Access'];
    PermissionSet pa = [SELECT Id FROM PermissionSet WHERE Name = 'Gl_Admin_Access'];
    System.runAs(new User(Id = UserInfo.getUserId())) {
      insert u;
      insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=ps.Id);
      insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(), PermissionSetId=pa.Id);
    }
    return u;
  }
  static Id[] data(User u) {
    Gl_Obj__c a = new Gl_Obj__c(Name='adm', Open__c='o', Secret__c='x');
    insert a;
    Gl_Obj__c s = new Gl_Obj__c(Name='std', Open__c='o', Secret__c='x', OwnerId=u.Id);
    insert s;
    return new Id[]{s.Id, a.Id};
  }
  static void fin(String s) { System.assert(false, 'RESULT ' + s); }
  static String fld(SObject o, String f) {
    try { return String.valueOf(o.get(f)); } catch (Exception e) { return e.getTypeName() + '/' + e.getMessage(); }
  }
  static String dsc(List<List<SObject>> r) {
    String s = 'n=' + r[0].size();
    for (SObject o : r[0]) { s += ' [open=' + fld(o, 'Open__c') + ' secret=' + fld(o, 'Secret__c') + ']'; }
    return s;
  }
  static String locCount(Database.QueryLocator q) {
    try { Integer c = 0; Database.QueryLocatorIterator it = q.iterator(); while (it.hasNext()) { it.next(); c++; } return 'rows=' + c; } catch (Exception e) { return 'iterEX ' + e.getTypeName() + ': ' + e.getMessage(); }
  }
  static String sosl(Id[] ids) {
    String s = '';
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_sec_def=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c)]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_def=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_sec_user=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c) WITH USER_MODE]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_user=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_mix_def=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Open__c, Secret__c)]) + ' ;; '; } catch (Exception e) { s += 'sosl_mix_def=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_mix_user=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Open__c, Secret__c) WITH USER_MODE]) + ' ;; '; } catch (Exception e) { s += 'sosl_mix_user=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_open_def=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Open__c)]) + ' ;; '; } catch (Exception e) { s += 'sosl_open_def=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_open_user=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Open__c) WITH USER_MODE]) + ' ;; '; } catch (Exception e) { s += 'sosl_open_user=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_sec_where_def=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id WHERE Secret__c = 'x')]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_where_def=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_sec_where_user=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id WHERE Secret__c = 'x') WITH USER_MODE]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_where_user=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    return s;
  }
  static String locA() {
    String s = '';
    Map<String,Object> m = new Map<String,Object>{'n'=>'std'};
    String qa = 'SELECT Id, Open__c FROM Gl_Obj__c WHERE Name = :n';
    String qs = 'SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n';
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qa, m, AccessLevel.USER_MODE); s += 'loc_acc_plain=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_acc_plain=' + GlFmt.ex(e) + ' ;; '; }
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qs, m, AccessLevel.USER_MODE); s += 'loc_sec_plain=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_sec_plain=' + GlFmt.ex(e) + ' ;; '; }
    return s;
  }
  static String locB() {
    String s = '';
    Map<String,Object> m = new Map<String,Object>{'n'=>'std'};
    String qa = 'SELECT Id, Open__c FROM Gl_Obj__c WHERE Name = :n';
    String qs = 'SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n';
    Integer n = 5;
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qa, m, AccessLevel.USER_MODE); s += 'loc_acc_scopeint=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_acc_scopeint=' + GlFmt.ex(e) + ' ;; '; }
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qs, m, AccessLevel.USER_MODE); s += 'loc_sec_scopeint=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_sec_scopeint=' + GlFmt.ex(e) + ' ;; '; }
    return s;
  }
  static String locC() {
    String s = '';
    Map<String,Object> m = new Map<String,Object>{'n'=>'std'};
    String qa = 'SELECT Id, Open__c FROM Gl_Obj__c WHERE Name = :n';
    String qs = 'SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n';
    String n = 'zzz';
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qa, m, AccessLevel.USER_MODE); s += 'loc_acc_scopestr=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_acc_scopestr=' + GlFmt.ex(e) + ' ;; '; }
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qs, m, AccessLevel.USER_MODE); s += 'loc_sec_scopestr=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_sec_scopestr=' + GlFmt.ex(e) + ' ;; '; }
    return s;
  }
  static String locD() {
    String s = '';
    Map<String,Object> m = new Map<String,Object>{'n'=>'std'};
    String qa = 'SELECT Id, Open__c FROM Gl_Obj__c WHERE Name = :n';
    String qs = 'SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n';
    try { Integer n = 3; } catch (Exception x) {}
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qa, m, AccessLevel.USER_MODE); s += 'loc_acc_departedint=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_acc_departedint=' + GlFmt.ex(e) + ' ;; '; }
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qs, m, AccessLevel.USER_MODE); s += 'loc_sec_departedint=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_sec_departedint=' + GlFmt.ex(e) + ' ;; '; }
    return s;
  }
  static String locE() {
    String s = '';
    Map<String,Object> m = new Map<String,Object>{'n'=>'std'};
    String qa = 'SELECT Id, Open__c FROM Gl_Obj__c WHERE Name = :n';
    String qs = 'SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n';
    if (m.size() == 1) { String n = 'zzz'; }
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qa, m, AccessLevel.USER_MODE); s += 'loc_acc_departedstr=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_acc_departedstr=' + GlFmt.ex(e) + ' ;; '; }
    try { Database.QueryLocator l = Database.getQueryLocatorWithBinds(qs, m, AccessLevel.USER_MODE); s += 'loc_sec_departedstr=OK ' + locCount(l) + ' ;; '; } catch (Exception e) { s += 'loc_sec_departedstr=' + GlFmt.ex(e) + ' ;; '; }
    return s;
  }
  static String loc() { return locA() + locB() + locC() + locD() + locE(); }
  static String soslEdge(Id[] ids, Id[] other) {
    String s = '';
    Test.setFixedSearchResults(new List<Id>());
    try { s += 'sosl_sec_def_none=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c)]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_def_none=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(other);
    try { s += 'sosl_sec_def_otherowner=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c)]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_def_otherowner=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(other);
    try { s += 'sosl_sec_user_otherowner=' + dsc([FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c) WITH USER_MODE]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_user_otherowner=' + GlFmt.ex(e) + ' ;; '; }
    Test.setFixedSearchResults(ids);
    try { s += 'sosl_sec_def_nomatchterm=' + dsc([FIND 'zzqq*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c)]) + ' ;; '; } catch (Exception e) { s += 'sosl_sec_def_nomatchterm=' + GlFmt.ex(e) + ' ;; '; }
    return s;
  }
  @IsTest static void std_sosl() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = sosl(d); } fin(s); }
  @IsTest static void adm_sosl() { User u = mkUser(); Id[] d = data(u); fin(sosl(d)); }
  @IsTest static void std_loc() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = loc(); } fin(s); }
  @IsTest static void adm_loc() { User u = mkUser(); Id[] d = data(u); fin(loc()); }
  @IsTest static void std_sosl_edge() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = soslEdge(new Id[]{d[0]}, new Id[]{d[1]}); } fin(s); }
  @IsTest static void adm_sosl_edge() { User u = mkUser(); Id[] d = data(u); fin(soslEdge(new Id[]{d[0]}, new Id[]{d[1]})); }
}
