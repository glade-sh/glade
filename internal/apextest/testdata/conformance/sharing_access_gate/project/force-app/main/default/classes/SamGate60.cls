@IsTest private class SamGate60 {
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
  @IsTest static void d5_01_ins_user() { User u = std(); String s = 'ok'; System.runAs(u) { try { insert as user new Sam_Obj__c(Name = 'n', Secret__c = 's'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_02_upd_user() { User u = std(); Sam_Obj__c r = new Sam_Obj__c(Name = 'n', OwnerId = u.Id); insert r; String s = 'ok'; System.runAs(u) { try { r.Secret__c = 's'; update as user r; } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_03_dbins_t() { User u = std(); String s = 'ok'; System.runAs(u) { try { s = SamFmt.sr(Database.insert(new List<Sam_Obj__c>{new Sam_Obj__c(Name = 'n', Secret__c = 's')}, true, AccessLevel.USER_MODE)); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_04_dbins_f() { User u = std(); String s = 'ok'; System.runAs(u) { try { s = SamFmt.sr(Database.insert(new List<Sam_Obj__c>{new Sam_Obj__c(Name = 'n', Secret__c = 's')}, false, AccessLevel.USER_MODE)); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_05_account_no_name() { User u = std(); String s = 'ok'; System.runAs(u) { try { insert as user new Account(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_05b_account_no_name_def() { User u = std(); String s = 'ok'; System.runAs(u) { try { insert new Account(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_06_query_user() { User u = std(); insert new Sam_Obj__c(Name = 'n', Secret__c = 's', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { s = 'n=' + [SELECT Secret__c FROM Sam_Obj__c WITH USER_MODE].size(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_07_dynquery_user() { User u = std(); insert new Sam_Obj__c(Name = 'n', Secret__c = 's', OwnerId = u.Id); String s = 'ok'; System.runAs(u) { try { s = 'n=' + Database.query('SELECT Secret__c FROM Sam_Obj__c', AccessLevel.USER_MODE).size(); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_08_ins_def() { User u = std(); String s = 'ok'; System.runAs(u) { try { insert new Sam_Obj__c(Name = 'n', Secret__c = 's'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_09_ups_user() { User u = std(); String s = 'ok'; System.runAs(u) { try { upsert as user new Sam_Obj__c(Name = 'n', Secret__c = 's'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_10_list_ins_user() { User u = std(); String s = 'ok'; System.runAs(u) { try { insert as user new List<Sam_Obj__c>{new Sam_Obj__c(Name = 'a'), new Sam_Obj__c(Name = 'n', Secret__c = 's')}; } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_11_ins_user_open() { User u = std(); String s = 'ok'; System.runAs(u) { try { insert as user new Sam_Obj__c(Name = 'n', Open__c = 'o'); } catch (Exception e) { s = SamFmt.ex(e); } } fin(s); }
  @IsTest static void d5_12_ins_user_outside() { std(); String s = 'ok'; try { insert as user new Sam_Obj__c(Name = 'n', Secret__c = 's'); } catch (Exception e) { s = SamFmt.ex(e); } fin(s); }
}
