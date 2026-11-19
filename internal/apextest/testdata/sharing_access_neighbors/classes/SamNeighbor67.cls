@IsTest private class SamNeighbor67 {
  static User mk(String profile, String extra, Boolean active) {
    Profile p = [SELECT Id FROM Profile WHERE Name = :profile LIMIT 1];
    String k = String.valueOf(Math.abs(Crypto.getRandomInteger()));
    User u = new User(Alias = 'sm' + k.left(5), Email = 'sam@example.invalid', EmailEncodingKey = 'UTF-8', LastName = 'Sam', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'sam' + k + '@example.invalid.glade', IsActive = active);
    Id adminSet = [SELECT Id FROM PermissionSet WHERE Name = 'Sam_Admin'].Id;
    System.runAs(new User(Id = UserInfo.getUserId())) {
      insert u;
      if (profile == 'Standard User') { insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Sam_Access'].Id); }
      if (extra != null) { insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = :extra].Id); }
      if ([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId() AND PermissionSetId = :adminSet] == 0) { insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = adminSet); }
    }
    return u;
  }
  static User std(String extra) { return mk('Standard User', extra, true); }
  static User minimum() { return mk('Minimum Access - Salesforce', null, true); }
  static Sam_RO__c ro(User u) { Sam_RO__c r = new Sam_RO__c(Name = 'r', OwnerId = u.Id); insert as system r; return r; }
  static Id[] sh(User u) {
    Sam_Sh__c mine = new Sam_Sh__c(Name = 'std', OwnerId = u.Id);
    Sam_Sh__c other = new Sam_Sh__c(Name = 'adm');
    insert as system new List<Sam_Sh__c>{mine, other};
    return new Id[]{mine.Id, other.Id};
  }
  static Id none(User u) { Sam_None__c r = new Sam_None__c(Name = 'x'); insert as system r; return r.Id; }
  static Id acct() { Account a = new Account(Name = 'x'); insert as system a; return a.Id; }
  static void fin(String s) { System.assert(false, 'RESULT => ' + s); }
  @IsTest static void n_readonly_ins_user() { User u = std(null); String s = 'ok'; System.runAs(u) { try { insert as user new Sam_RO__c(Name = 'x'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_create_only_ups_user() { User u = std('Sam_RO_C'); String s = 'ok'; System.runAs(u) { try { upsert as user new Sam_RO__c(Name = 'x'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_readonly_dbups_t() { User u = std(null); String s = 'ok'; System.runAs(u) { try { s = SamFmt.ur(Database.upsert(new List<Sam_RO__c>{new Sam_RO__c(Name = 'a')}, true, AccessLevel.USER_MODE)); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_hidden_upd_user() { User u = std(null); Id[] d = sh(u); String s = 'ok'; System.runAs(u) { try { update as user new Sam_Sh__c(Id = d[1], Name = 'u'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_hidden_ups_user() { User u = std(null); Id[] d = sh(u); String s = 'ok'; System.runAs(u) { try { upsert as user new Sam_Sh__c(Id = d[1], Name = 'u'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_count_like() { User u = std(null); none(null); String s = 'ok'; System.runAs(u) { try { s = 'n=' + [SELECT COUNT() FROM Sam_None__c WHERE Name LIKE 'x%']; } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_count_two_filters() { User u = std(null); none(null); String s = 'ok'; System.runAs(u) { try { s = 'n=' + [SELECT COUNT() FROM Sam_None__c WHERE Name = 'x' AND Name != 'y']; } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_strip_creatable_child() { User u = std(null); Account a = new Account(Name = 'p'); insert as system a; insert as system new Sam_None__c(Name = 'c', Account__c = a.Id); List<Account> l = [SELECT Id, Name, (SELECT Id, Name FROM Sam_Nones__r) FROM Account WHERE Id = :a.Id WITH SYSTEM_MODE]; String s = 'ok'; System.runAs(u) { try { s = SamFmt.strip(Security.stripInaccessible(AccessType.UPDATABLE, l)); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_strip_outside_runas() { Account a = new Account(Name = 'p'); insert as system a; insert as system new Sam_None__c(Name = 'c', Account__c = a.Id); List<Account> l = [SELECT Id, Name, (SELECT Id, Name FROM Sam_Nones__r) FROM Account WHERE Id = :a.Id WITH SYSTEM_MODE]; String s = 'ok'; try { s = SamFmt.strip(Security.stripInaccessible(AccessType.READABLE, l)); } catch (Exception e) { s = SamFmt.ex(e); } fin(s); }
  @IsTest static void n_conflict_outside() { insert as system new Sam_Obj__c(Name = 'a', Open__c = 'o'); String s = 'ok'; try { s = 'n=' + Database.query('SELECT Id, Open__c FROM Sam_Obj__c WITH SYSTEM_MODE', AccessLevel.USER_MODE).size(); } catch (Exception e) { s = SamFmt.ex(e); } fin(s); }
  @IsTest static void n_conflict_sys_sys() { User u = std(null); insert as system new Sam_Obj__c(Name = 'a', Open__c = 'o', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { s = 'n=' + Database.query('SELECT Id, Open__c FROM Sam_Obj__c WITH SYSTEM_MODE', AccessLevel.SYSTEM_MODE).size(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_count_conflict_sys_arg() { User u = std(null); insert as system new Sam_Obj__c(Name = 'a', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { s = 'n=' + Database.countQuery('SELECT COUNT() FROM Sam_Obj__c WITH USER_MODE', AccessLevel.SYSTEM_MODE); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_suggest_fixed() { Sam_Obj__c r = new Sam_Obj__c(Name = 'needle'); insert as system r; Test.setFixedSearchResults(new List<Id>{r.Id}); String s = 'ok'; try { s = 'n=' + Search.suggest('needle', 'Sam_Obj__c', new Search.SuggestionOption()).getSuggestionResults().size(); } catch (Exception e) { s = SamFmt.ex(e); } fin(s); }
  @IsTest static void n_batch_stop_outside() { User u = std(null); sh(u); Test.startTest(); System.runAs(u) { SamEnqWo67.b('Wo', 'b'); } Test.stopTest(); fin(SamLog.all()); }
  @IsTest static void n_undelete_readonly_outside() { Sam_RO__c r = new Sam_RO__c(Name = 'n'); insert as system r; delete as system r; String s = 'ok'; try { undelete as user new Sam_RO__c(Id = r.Id); } catch (Exception e) { s = SamFmt.ex(e); } fin(s); }
  @IsTest static void n_field_user_dml() { User u = std(null); String s = 'ok'; System.runAs(u) { try { insert as user new Sam_Obj__c(Name = 'n', Secret__c = 's'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_missing_name_user_dml() { User u = std(null); String s = 'ok'; System.runAs(u) { try { insert as user new Account(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void n_missing_name_db_user() { User u = std(null); String s = 'ok'; System.runAs(u) { try { Database.insert(new Account(), AccessLevel.USER_MODE); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
}
