public class LtProbe {
  static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
  @AuraEnabled public static String run(String kind, String key) {
    try {
      if (kind == 'stmt') { Contact c = new Contact(LastName='L', RouteKey__c=key); upsert c Contact.RouteKey__c; return 'ok'; }
      if (kind == 'dbFalse') { Contact c = new Contact(LastName='L', RouteKey__c=key); Database.UpsertResult r = Database.upsert(c, Contact.RouteKey__c, false); return r.isSuccess() ? 'ok created=' + r.isCreated() : 'rowErr ' + r.getErrors()[0].getMessage(); }
      if (kind == 'dbDefault') { Contact c = new Contact(LastName='L', RouteKey__c=key); Database.UpsertResult[] r = Database.upsert(new List<Contact>{c}, Contact.RouteKey__c); return r[0].isSuccess() ? 'ok created=' + r[0].isCreated() : 'rowErr'; }
      return 'bad';
    } catch (Exception e) { return 'EX ' + ex(e); }
  }
  @AuraEnabled public static String seed(String key) { insert new Contact(LastName='S', RouteKey__c=key); return 'seeded'; }
}
