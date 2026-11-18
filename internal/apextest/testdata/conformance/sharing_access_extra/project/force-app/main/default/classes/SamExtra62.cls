@IsTest private class SamExtra62 {
  static User std() {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    String k = String.valueOf(Math.abs(Crypto.getRandomInteger()));
    User u = new User(Alias = 'sm' + k.left(5), Email = 'sam@example.invalid', EmailEncodingKey = 'UTF-8', LastName = 'Sam', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'sam' + k + '@example.invalid.glade');
    Id adminSet = [SELECT Id FROM PermissionSet WHERE Name = 'Sam_Admin'].Id;
    System.runAs(new User(Id = UserInfo.getUserId())) {
      insert u;
      insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Sam_Access'].Id);
      if ([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId() AND PermissionSetId = :adminSet] == 0) { insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = adminSet); }
    }
    return u;
  }
  static void fin(String s) { System.assert(false, 'RESULT => ' + s); }
  @IsTest static void u2_04_runas_version() { String s = 'none'; Integer b = Limits.getDmlStatements(); try { System.runAs(new Version(1, 0)) { s = 'ran'; } s += ' d=' + (Limits.getDmlStatements() - b); } catch (Exception e) { s = SamFmt.ex(e); } fin(s); }
  @IsTest static void u3_05_cursor_user() { User u = std(); insert new Sam_Obj__c(Name = 'n', Secret__c = 's', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { Database.Cursor c = Database.getCursor('SELECT Secret__c FROM Sam_Obj__c', AccessLevel.USER_MODE); s = 'n=' + c.getNumRecords(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void u3_05b_cursor_def() { User u = std(); insert new Sam_Obj__c(Name = 'n', Secret__c = 's', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { Database.Cursor c = Database.getCursor('SELECT Secret__c FROM Sam_Obj__c'); s = 'n=' + c.getNumRecords(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void u3_05c_cursor_sys() { User u = std(); insert new Sam_Obj__c(Name = 'n', Secret__c = 's', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { Database.Cursor c = Database.getCursor('SELECT Secret__c FROM Sam_Obj__c', AccessLevel.SYSTEM_MODE); s = 'n=' + c.getNumRecords(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void u3_06_undelete_user() { User u = std(); Sam_Obj__c r = new Sam_Obj__c(Name = 'n', OwnerId = u.Id); insert r; delete r; String s = 'ok'; System.runAs(u) { try { List<Database.UndeleteResult> l = Database.undelete(new List<Id>{r.Id}, true, AccessLevel.USER_MODE); s = 'ok=' + l[0].isSuccess(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void u3_06b_merge_user() { User u = std(); Account a = new Account(Name = 'a', OwnerId = u.Id); Account b = new Account(Name = 'b', OwnerId = u.Id); insert new List<Account>{a, b}; String s = 'ok'; System.runAs(u) { try { Database.MergeResult r = Database.merge(a, b.Id, true, AccessLevel.USER_MODE); s = 'ok=' + r.isSuccess(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void u3_06c_convert_user() { User u = std(); Lead l = new Lead(LastName = 'l', Company = 'c', OwnerId = u.Id); insert l; String status = [SELECT MasterLabel FROM LeadStatus WHERE IsConverted = true LIMIT 1].MasterLabel; String s = 'ok'; System.runAs(u) { try { Database.LeadConvert lc = new Database.LeadConvert(); lc.setLeadId(l.Id); lc.setConvertedStatus(status); Database.LeadConvertResult r = Database.convertLead(lc, true, AccessLevel.USER_MODE); s = 'ok=' + r.isSuccess(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void u3_07_suggest_user() { User u = std(); insert new Sam_Obj__c(Name = 'needle', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { Search.SuggestionResults r = Search.suggest('needle', 'Sam_Obj__c', new Search.SuggestionOption(), AccessLevel.USER_MODE); s = 'n=' + r.getSuggestionResults().size(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
}
