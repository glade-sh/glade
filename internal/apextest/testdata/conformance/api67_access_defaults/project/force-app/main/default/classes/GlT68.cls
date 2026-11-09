@IsTest private class GlT68 {
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
  static String sel(String[] ps) { String s=''; for (String l : GlCfg.log) { for (String p : ps) { if (l.startsWith(p)) { s += l + ' ;; '; break; } } } return s; }
  @IsTest static void std_g1() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g1(d[0]); } fin(s); }
  @IsTest static void adm_g1() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g1(d[0]); fin(s); }
  @IsTest static void std_g6() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g6(d[0]); } fin(s); }
  @IsTest static void adm_g6() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g6(d[0]); fin(s); }
  @IsTest static void std_g7() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g7(d[0]); } fin(s); }
  @IsTest static void adm_g7() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g7(d[0]); fin(s); }
  @IsTest static void std_g2() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g2(d[0]); } fin(s); }
  @IsTest static void adm_g2() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g2(d[0]); fin(s); }
  @IsTest static void std_g8() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g8(d[0]); } fin(s); }
  @IsTest static void adm_g8() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g8(d[0]); fin(s); }
  @IsTest static void std_g3() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g3(d[0]); } fin(s); }
  @IsTest static void adm_g3() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g3(d[0]); fin(s); }
  @IsTest static void std_g9() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g9(d[0]); } fin(s); }
  @IsTest static void adm_g9() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g9(d[0]); fin(s); }
  @IsTest static void std_g4() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g4(d[0]); } fin(s); }
  @IsTest static void adm_g4() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g4(d[0]); fin(s); }
  @IsTest static void std_g5() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s = GlScen68.g5(d[0]); } fin(s); }
  @IsTest static void adm_g5() { User u = mkUser(); Id[] d = data(u); String s = GlScen68.g5(d[0]); fin(s); }
  @IsTest static void std_shdirect() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s += 'direct>ShBase: ' + new ShBase68().all(d[1]) + ' ;; '; s += 'direct>ShWo: ' + new ShWo68().all(d[1]) + ' ;; '; s += 'direct>ShW: ' + new ShW68().all(d[1]) + ' ;; '; s += 'direct>ShInh: ' + new ShInh68().all(d[1]) + ' ;; '; s += 'direct>ShSubBase: ' + new ShSubBase68().all(d[1]) + ' ;; '; s += 'direct>ShSubWo: ' + new ShSubWo68().all(d[1]) + ' ;; '; s += 'direct>ShSubW: ' + new ShSubW68().all(d[1]) + ' ;; '; s += 'direct>ShSubInh: ' + new ShSubInh68().all(d[1]) + ' ;; '; s += 'direct>ShSubSubWo: ' + new ShSubSubWo68().all(d[1]) + ' ;; '; } fin(s); }
  @IsTest static void std_shviaWo() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s += 'Wo>ShBase: ' + ShCallerWo68.ShBase(d[1]) + ' ;; '; s += 'Wo>ShWo: ' + ShCallerWo68.ShWo(d[1]) + ' ;; '; s += 'Wo>ShW: ' + ShCallerWo68.ShW(d[1]) + ' ;; '; s += 'Wo>ShInh: ' + ShCallerWo68.ShInh(d[1]) + ' ;; '; s += 'Wo>ShSubBase: ' + ShCallerWo68.ShSubBase(d[1]) + ' ;; '; s += 'Wo>ShSubWo: ' + ShCallerWo68.ShSubWo(d[1]) + ' ;; '; s += 'Wo>ShSubW: ' + ShCallerWo68.ShSubW(d[1]) + ' ;; '; s += 'Wo>ShSubInh: ' + ShCallerWo68.ShSubInh(d[1]) + ' ;; '; s += 'Wo>ShSubSubWo: ' + ShCallerWo68.ShSubSubWo(d[1]) + ' ;; '; } fin(s); }
  @IsTest static void std_shviaW() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s += 'W>ShBase: ' + ShCallerW68.ShBase(d[1]) + ' ;; '; s += 'W>ShWo: ' + ShCallerW68.ShWo(d[1]) + ' ;; '; s += 'W>ShW: ' + ShCallerW68.ShW(d[1]) + ' ;; '; s += 'W>ShInh: ' + ShCallerW68.ShInh(d[1]) + ' ;; '; s += 'W>ShSubBase: ' + ShCallerW68.ShSubBase(d[1]) + ' ;; '; s += 'W>ShSubWo: ' + ShCallerW68.ShSubWo(d[1]) + ' ;; '; s += 'W>ShSubW: ' + ShCallerW68.ShSubW(d[1]) + ' ;; '; s += 'W>ShSubInh: ' + ShCallerW68.ShSubInh(d[1]) + ' ;; '; s += 'W>ShSubSubWo: ' + ShCallerW68.ShSubSubWo(d[1]) + ' ;; '; } fin(s); }
  @IsTest static void std_shviaNone() { User u = mkUser(); Id[] d = data(u); String s=''; System.runAs(u) { s += 'None>ShBase: ' + ShCallerNone68.ShBase(d[1]) + ' ;; '; s += 'None>ShWo: ' + ShCallerNone68.ShWo(d[1]) + ' ;; '; s += 'None>ShW: ' + ShCallerNone68.ShW(d[1]) + ' ;; '; s += 'None>ShInh: ' + ShCallerNone68.ShInh(d[1]) + ' ;; '; s += 'None>ShSubBase: ' + ShCallerNone68.ShSubBase(d[1]) + ' ;; '; s += 'None>ShSubWo: ' + ShCallerNone68.ShSubWo(d[1]) + ' ;; '; s += 'None>ShSubW: ' + ShCallerNone68.ShSubW(d[1]) + ' ;; '; s += 'None>ShSubInh: ' + ShCallerNone68.ShSubInh(d[1]) + ' ;; '; s += 'None>ShSubSubWo: ' + ShCallerNone68.ShSubSubWo(d[1]) + ' ;; '; } fin(s); }
  @IsTest static void adm_shdirect() { User u = mkUser(); Id[] d = data(u); String s=''; s += 'direct>ShBase: ' + new ShBase68().all(d[1]) + ' ;; '; s += 'direct>ShWo: ' + new ShWo68().all(d[1]) + ' ;; '; s += 'direct>ShW: ' + new ShW68().all(d[1]) + ' ;; '; s += 'direct>ShInh: ' + new ShInh68().all(d[1]) + ' ;; '; s += 'direct>ShSubBase: ' + new ShSubBase68().all(d[1]) + ' ;; '; s += 'direct>ShSubWo: ' + new ShSubWo68().all(d[1]) + ' ;; '; s += 'direct>ShSubW: ' + new ShSubW68().all(d[1]) + ' ;; '; s += 'direct>ShSubInh: ' + new ShSubInh68().all(d[1]) + ' ;; '; s += 'direct>ShSubSubWo: ' + new ShSubSubWo68().all(d[1]) + ' ;; '; fin(s); }
  @IsTest static void adm_shviaWo() { User u = mkUser(); Id[] d = data(u); String s=''; s += 'Wo>ShBase: ' + ShCallerWo68.ShBase(d[1]) + ' ;; '; s += 'Wo>ShWo: ' + ShCallerWo68.ShWo(d[1]) + ' ;; '; s += 'Wo>ShW: ' + ShCallerWo68.ShW(d[1]) + ' ;; '; s += 'Wo>ShInh: ' + ShCallerWo68.ShInh(d[1]) + ' ;; '; s += 'Wo>ShSubBase: ' + ShCallerWo68.ShSubBase(d[1]) + ' ;; '; s += 'Wo>ShSubWo: ' + ShCallerWo68.ShSubWo(d[1]) + ' ;; '; s += 'Wo>ShSubW: ' + ShCallerWo68.ShSubW(d[1]) + ' ;; '; s += 'Wo>ShSubInh: ' + ShCallerWo68.ShSubInh(d[1]) + ' ;; '; s += 'Wo>ShSubSubWo: ' + ShCallerWo68.ShSubSubWo(d[1]) + ' ;; '; fin(s); }
  @IsTest static void adm_shviaW() { User u = mkUser(); Id[] d = data(u); String s=''; s += 'W>ShBase: ' + ShCallerW68.ShBase(d[1]) + ' ;; '; s += 'W>ShWo: ' + ShCallerW68.ShWo(d[1]) + ' ;; '; s += 'W>ShW: ' + ShCallerW68.ShW(d[1]) + ' ;; '; s += 'W>ShInh: ' + ShCallerW68.ShInh(d[1]) + ' ;; '; s += 'W>ShSubBase: ' + ShCallerW68.ShSubBase(d[1]) + ' ;; '; s += 'W>ShSubWo: ' + ShCallerW68.ShSubWo(d[1]) + ' ;; '; s += 'W>ShSubW: ' + ShCallerW68.ShSubW(d[1]) + ' ;; '; s += 'W>ShSubInh: ' + ShCallerW68.ShSubInh(d[1]) + ' ;; '; s += 'W>ShSubSubWo: ' + ShCallerW68.ShSubSubWo(d[1]) + ' ;; '; fin(s); }
  @IsTest static void adm_shviaNone() { User u = mkUser(); Id[] d = data(u); String s=''; s += 'None>ShBase: ' + ShCallerNone68.ShBase(d[1]) + ' ;; '; s += 'None>ShWo: ' + ShCallerNone68.ShWo(d[1]) + ' ;; '; s += 'None>ShW: ' + ShCallerNone68.ShW(d[1]) + ' ;; '; s += 'None>ShInh: ' + ShCallerNone68.ShInh(d[1]) + ' ;; '; s += 'None>ShSubBase: ' + ShCallerNone68.ShSubBase(d[1]) + ' ;; '; s += 'None>ShSubWo: ' + ShCallerNone68.ShSubWo(d[1]) + ' ;; '; s += 'None>ShSubW: ' + ShCallerNone68.ShSubW(d[1]) + ' ;; '; s += 'None>ShSubInh: ' + ShCallerNone68.ShSubInh(d[1]) + ' ;; '; s += 'None>ShSubSubWo: ' + ShCallerNone68.ShSubSubWo(d[1]) + ' ;; '; fin(s); }
  @IsTest static void std_trg_dml() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; System.runAs(u) { insert new Gl_Trg__c(Name='t'); } fin(sel(new String[]{'t_dml','t_dbi_none'})); }
  @IsTest static void std_trg_db() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; System.runAs(u) { insert new Gl_Trg__c(Name='t'); } fin(sel(new String[]{'t_db'})); }
  @IsTest static void std_trg_soql() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; System.runAs(u) { insert new Gl_Trg__c(Name='t'); } fin(sel(new String[]{'t_soql','t_sosl'})); }
  @IsTest static void std_trg_dyn() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; System.runAs(u) { insert new Gl_Trg__c(Name='t'); } fin(sel(new String[]{'t_dyn_q','t_dyn_count','t_dyn_loc'})); }
  @IsTest static void std_trg_bind() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; System.runAs(u) { insert new Gl_Trg__c(Name='t'); } fin(sel(new String[]{'t_dyn_bind','t_dyn_cntbind','t_dyn_locbind','t_cnt','t_dyncnt','t_syscnt','t_upd'})); }
  @IsTest static void adm_trg_dml() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; insert new Gl_Trg__c(Name='t'); fin(sel(new String[]{'t_dml','t_dbi_none'})); }
  @IsTest static void adm_trg_db() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; insert new Gl_Trg__c(Name='t'); fin(sel(new String[]{'t_db'})); }
  @IsTest static void adm_trg_soql() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; insert new Gl_Trg__c(Name='t'); fin(sel(new String[]{'t_soql','t_sosl'})); }
  @IsTest static void adm_trg_dyn() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; insert new Gl_Trg__c(Name='t'); fin(sel(new String[]{'t_dyn_q','t_dyn_count','t_dyn_loc'})); }
  @IsTest static void adm_trg_bind() { User u = mkUser(); Id[] d = data(u); GlCfg.ver=68; GlCfg.stdRec=d[0]; GlCfg.admRec=d[1]; insert new Gl_Trg__c(Name='t'); fin(sel(new String[]{'t_dyn_bind','t_dyn_cntbind','t_dyn_locbind','t_cnt','t_dyncnt','t_syscnt','t_upd'})); }
}