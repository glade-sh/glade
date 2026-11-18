@IsTest private class PfNeighbor58 {
  static void fin(String s) { System.assert(false, 'RESULT => ' + s); }
  // Uncaptured below API 62.0: only the single-record insert as user has a row (sharing_access_gate d5_12).
  @IsTest static void n4_update_as_user() { Pf_Fld__c r = new Pf_Fld__c(Name = 'n'); insert as system r; String s = 'ok'; try { r.Secret__c = 's'; update as user r; } catch (Exception e) { s = e.getTypeName(); } fin(s); }
  @IsTest static void n5_list_insert_as_user() { String s = 'ok'; try { insert as user new List<Pf_Fld__c>{new Pf_Fld__c(Name = 'n', Secret__c = 's')}; } catch (Exception e) { s = e.getTypeName(); } fin(s); }
  @IsTest static void n6_default_insert() { String s = 'ok'; try { insert new Pf_Fld__c(Name = 'n', Secret__c = 's'); } catch (Exception e) { s = e.getTypeName(); } fin(s); }
}
