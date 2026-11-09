public with sharing class GlScen66 {
  static String dml_ins_secret(Id rec) { try { insert new Gl_Obj__c(Name='n', Secret__c='s'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dml_ins_open(Id rec) { try { insert new Gl_Obj__c(Name='n', Open__c='o'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dml_upd_secret(Id rec) { try { update new Gl_Obj__c(Id=rec, Secret__c='z'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dml_upd_open(Id rec) { try { update new Gl_Obj__c(Id=rec, Open__c='z'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dml_ups_secret(Id rec) { try { upsert new Gl_Obj__c(Id=rec, Secret__c='z'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dml_ins_as_system(Id rec) { try { insert as system new Gl_Obj__c(Name='n', Secret__c='s'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g1(Id rec) { String s=''; 
    s += 'dml_ins_secret=' + dml_ins_secret(rec) + ' ;; ';
    s += 'dml_ins_open=' + dml_ins_open(rec) + ' ;; ';
    s += 'dml_upd_secret=' + dml_upd_secret(rec) + ' ;; ';
    s += 'dml_upd_open=' + dml_upd_open(rec) + ' ;; ';
    s += 'dml_ups_secret=' + dml_ups_secret(rec) + ' ;; ';
    s += 'dml_ins_as_system=' + dml_ins_as_system(rec) + ' ;; ';
    return s; }
  static String dml_del(Id rec) { try { delete new Gl_Obj__c(Id=rec); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g6(Id rec) { String s=''; 
    s += 'dml_del=' + dml_del(rec) + ' ;; ';
    return s; }
  static String dml_del_as_system(Id rec) { try { delete as system new Gl_Obj__c(Id=rec); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g7(Id rec) { String s=''; 
    s += 'dml_del_as_system=' + dml_del_as_system(rec) + ' ;; ';
    return s; }
  static String db_ins_default(Id rec) { try { Database.insert(new Gl_Obj__c(Name='n', Secret__c='s')); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_ins_partial(Id rec) { try { List<Database.SaveResult> r = Database.insert(new List<Gl_Obj__c>{new Gl_Obj__c(Name='n', Secret__c='s')}, false); return GlFmt.sr(r); } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_upd_partial(Id rec) { try { List<Database.SaveResult> r = Database.update(new List<Gl_Obj__c>{new Gl_Obj__c(Id=rec, Secret__c='z')}, false); return GlFmt.sr(r); } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_ups_partial(Id rec) { try { List<Database.UpsertResult> r = Database.upsert(new List<Gl_Obj__c>{new Gl_Obj__c(Id=rec, Secret__c='z')}, false); return GlFmt.ur(r); } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_ins_user(Id rec) { try { Database.insert(new Gl_Obj__c(Name='n', Secret__c='s'), AccessLevel.USER_MODE); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_ins_system(Id rec) { try { Database.insert(new Gl_Obj__c(Name='n', Secret__c='s'), AccessLevel.SYSTEM_MODE); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g2(Id rec) { String s=''; 
    s += 'db_ins_default=' + db_ins_default(rec) + ' ;; ';
    s += 'db_ins_partial=' + db_ins_partial(rec) + ' ;; ';
    s += 'db_upd_partial=' + db_upd_partial(rec) + ' ;; ';
    s += 'db_ups_partial=' + db_ups_partial(rec) + ' ;; ';
    s += 'db_ins_user=' + db_ins_user(rec) + ' ;; ';
    s += 'db_ins_system=' + db_ins_system(rec) + ' ;; ';
    return s; }
  static String db_del_partial(Id rec) { try { List<Database.DeleteResult> r = Database.delete(new List<Id>{rec}, false); return GlFmt.dr(r); } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g8(Id rec) { String s=''; 
    s += 'db_del_partial=' + db_del_partial(rec) + ' ;; ';
    return s; }
  static String soql_sel_secret(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id, Secret__c FROM Gl_Obj__c]; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_sel_open(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id, Open__c FROM Gl_Obj__c]; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_where_secret(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id FROM Gl_Obj__c WHERE Secret__c='x']; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_order_secret(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id FROM Gl_Obj__c ORDER BY Secret__c]; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_count_secret(Id rec) { try { Integer n=[SELECT COUNT() FROM Gl_Obj__c WHERE Secret__c='x']; return 'n='+n; } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_sys_secret(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id, Secret__c FROM Gl_Obj__c WITH SYSTEM_MODE]; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_user_secret(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id, Secret__c FROM Gl_Obj__c WITH USER_MODE]; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String sosl_secret(Id rec) { try { List<List<SObject>> r=[FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c)]; return 'n='+r[0].size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String sosl_user_secret(Id rec) { try { List<List<SObject>> r=[FIND 'x*' IN ALL FIELDS RETURNING Gl_Obj__c(Id, Secret__c) WITH USER_MODE]; return 'n='+r[0].size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String soql_where_user(Id rec) { try { List<Gl_Obj__c> l=[SELECT Id FROM Gl_Obj__c WHERE Secret__c='x' WITH USER_MODE]; return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g3(Id rec) { String s=''; 
    s += 'soql_sel_secret=' + soql_sel_secret(rec) + ' ;; ';
    s += 'soql_sel_open=' + soql_sel_open(rec) + ' ;; ';
    s += 'soql_where_secret=' + soql_where_secret(rec) + ' ;; ';
    s += 'soql_order_secret=' + soql_order_secret(rec) + ' ;; ';
    s += 'soql_count_secret=' + soql_count_secret(rec) + ' ;; ';
    s += 'soql_sys_secret=' + soql_sys_secret(rec) + ' ;; ';
    s += 'soql_user_secret=' + soql_user_secret(rec) + ' ;; ';
    s += 'sosl_secret=' + sosl_secret(rec) + ' ;; ';
    s += 'sosl_user_secret=' + sosl_user_secret(rec) + ' ;; ';
    s += 'soql_where_user=' + soql_where_user(rec) + ' ;; ';
    return s; }
  static String db_ins_mixed(Id rec) { try { List<Gl_Obj__c> l=new List<Gl_Obj__c>{new Gl_Obj__c(Name='mx', Open__c='o'), new Gl_Obj__c(Name='mx', Secret__c='s')}; List<Database.SaveResult> r=Database.insert(l,false);Integer c=[SELECT COUNT() FROM Gl_Obj__c WHERE Name='mx' WITH SYSTEM_MODE]; return GlFmt.sr(r)+' count='+c; } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_ins_user_partial(Id rec) { try { List<Gl_Obj__c> l=new List<Gl_Obj__c>{new Gl_Obj__c(Name='mx', Open__c='o'), new Gl_Obj__c(Name='mx', Secret__c='s')}; List<Database.SaveResult> r=Database.insert(l,false,AccessLevel.USER_MODE);Integer c=[SELECT COUNT() FROM Gl_Obj__c WHERE Name='mx' WITH SYSTEM_MODE]; return GlFmt.sr(r)+' count='+c; } catch (Exception e) { return GlFmt.ex(e); } }
  static String db_ins_list_true(Id rec) { try { List<Gl_Obj__c> l=new List<Gl_Obj__c>{new Gl_Obj__c(Name='mx', Open__c='o'), new Gl_Obj__c(Name='mx', Secret__c='s')}; String m='no-ex'; try { Database.insert(l); } catch (Exception x) { m=GlFmt.ex(x); }Integer c=[SELECT COUNT() FROM Gl_Obj__c WHERE Name='mx' WITH SYSTEM_MODE]; return m+' count='+c; } catch (Exception e) { return GlFmt.ex(e); } }
  static String stmt_ins_list(Id rec) { try { List<Gl_Obj__c> l=new List<Gl_Obj__c>{new Gl_Obj__c(Name='mx', Open__c='o'), new Gl_Obj__c(Name='mx', Secret__c='s')}; String m='no-ex'; try { insert l; } catch (Exception x) { m=GlFmt.ex(x); }Integer c=[SELECT COUNT() FROM Gl_Obj__c WHERE Name='mx' WITH SYSTEM_MODE]; return m+' count='+c; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g9(Id rec) { String s=''; 
    s += 'db_ins_mixed=' + db_ins_mixed(rec) + ' ;; ';
    s += 'db_ins_user_partial=' + db_ins_user_partial(rec) + ' ;; ';
    s += 'db_ins_list_true=' + db_ins_list_true(rec) + ' ;; ';
    s += 'stmt_ins_list=' + stmt_ins_list(rec) + ' ;; ';
    return s; }
  static String dyn_count_user(Id rec) { try { Integer n=Database.countQuery('SELECT COUNT() FROM Gl_Obj__c WHERE Secret__c = \'x\'', AccessLevel.USER_MODE); return 'n='+n; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_q_secret(Id rec) { try { List<SObject> l=Database.query('SELECT Id, Secret__c FROM Gl_Obj__c'); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_q_where(Id rec) { try { List<SObject> l=Database.query('SELECT Id FROM Gl_Obj__c WHERE Secret__c = \'x\''); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_q_open(Id rec) { try { List<SObject> l=Database.query('SELECT Id, Open__c FROM Gl_Obj__c'); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_q_sys(Id rec) { try { List<SObject> l=Database.query('SELECT Id, Secret__c FROM Gl_Obj__c', AccessLevel.SYSTEM_MODE); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_q_user(Id rec) { try { List<SObject> l=Database.query('SELECT Id, Secret__c FROM Gl_Obj__c', AccessLevel.USER_MODE); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_count_where(Id rec) { try { Integer n=Database.countQuery('SELECT COUNT() FROM Gl_Obj__c WHERE Secret__c = \'x\''); return 'n='+n; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_count_sys(Id rec) { try { Integer n=Database.countQuery('SELECT COUNT() FROM Gl_Obj__c WHERE Secret__c = \'x\'', AccessLevel.SYSTEM_MODE); return 'n='+n; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_loc_secret(Id rec) { try { Database.QueryLocator q=Database.getQueryLocator('SELECT Id, Secret__c FROM Gl_Obj__c'); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_loc_sys(Id rec) { try { Database.QueryLocator q=Database.getQueryLocator('SELECT Id, Secret__c FROM Gl_Obj__c', AccessLevel.SYSTEM_MODE); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g4(Id rec) { String s=''; 
    s += 'dyn_count_user=' + dyn_count_user(rec) + ' ;; ';
    s += 'dyn_q_secret=' + dyn_q_secret(rec) + ' ;; ';
    s += 'dyn_q_where=' + dyn_q_where(rec) + ' ;; ';
    s += 'dyn_q_open=' + dyn_q_open(rec) + ' ;; ';
    s += 'dyn_q_sys=' + dyn_q_sys(rec) + ' ;; ';
    s += 'dyn_q_user=' + dyn_q_user(rec) + ' ;; ';
    s += 'dyn_count_where=' + dyn_count_where(rec) + ' ;; ';
    s += 'dyn_count_sys=' + dyn_count_sys(rec) + ' ;; ';
    s += 'dyn_loc_secret=' + dyn_loc_secret(rec) + ' ;; ';
    s += 'dyn_loc_sys=' + dyn_loc_sys(rec) + ' ;; ';
    return s; }
  static String dyn_bind_user(Id rec) { try { List<SObject> l=Database.queryWithBinds('SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n', new Map<String,Object>{'n'=>'std'}, AccessLevel.USER_MODE); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_bind_sys(Id rec) { try { List<SObject> l=Database.queryWithBinds('SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n', new Map<String,Object>{'n'=>'std'}, AccessLevel.SYSTEM_MODE); return 'n='+l.size(); } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_cntbind_user(Id rec) { try { Integer n=Database.countQueryWithBinds('SELECT COUNT() FROM Gl_Obj__c WHERE Secret__c = :n', new Map<String,Object>{'n'=>'std'}, AccessLevel.USER_MODE); return 'n='+n; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_cntbind_sys(Id rec) { try { Integer n=Database.countQueryWithBinds('SELECT COUNT() FROM Gl_Obj__c WHERE Secret__c = :n', new Map<String,Object>{'n'=>'std'}, AccessLevel.SYSTEM_MODE); return 'n='+n; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_locbind_user(Id rec) { try { Database.QueryLocator q=Database.getQueryLocatorWithBinds('SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n', new Map<String,Object>{'n'=>'std'}, AccessLevel.USER_MODE); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  static String dyn_locbind_sys(Id rec) { try { Database.QueryLocator q=Database.getQueryLocatorWithBinds('SELECT Id, Secret__c FROM Gl_Obj__c WHERE Name = :n', new Map<String,Object>{'n'=>'std'}, AccessLevel.SYSTEM_MODE); return 'OK'; } catch (Exception e) { return GlFmt.ex(e); } }
  @AuraEnabled public static String g5(Id rec) { String s=''; 
    s += 'dyn_bind_user=' + dyn_bind_user(rec) + ' ;; ';
    s += 'dyn_bind_sys=' + dyn_bind_sys(rec) + ' ;; ';
    s += 'dyn_cntbind_user=' + dyn_cntbind_user(rec) + ' ;; ';
    s += 'dyn_cntbind_sys=' + dyn_cntbind_sys(rec) + ' ;; ';
    s += 'dyn_locbind_user=' + dyn_locbind_user(rec) + ' ;; ';
    s += 'dyn_locbind_sys=' + dyn_locbind_sys(rec) + ' ;; ';
    return s; }
}