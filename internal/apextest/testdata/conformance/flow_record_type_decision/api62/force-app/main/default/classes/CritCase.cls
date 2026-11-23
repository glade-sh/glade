public class CritCase {
 static String res(Database.SaveResult r) { return r.isSuccess() ? 'ok' : 'ERR:' + String.valueOf(r.getErrors()[0].getStatusCode()) + ':' + r.getErrors()[0].getMessage(); }
 public static String pl() {
  String p = [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name;
  List<String> out = new List<String>();
  for (String t : new List<String>{p, 'other', null}) {
   Contact c = new Contact(LastName='CritPL', Title=t);
   Database.SaveResult r = Database.insert(c, false);
   if (!r.isSuccess()) { out.add(res(r)); continue; }
   c = [SELECT Description, Department, AssistantName, MailingCity FROM Contact WHERE Id = :c.Id];
   out.add(c.Description + ',' + c.Department + ',' + c.AssistantName + ',' + c.MailingCity);
  }
  return String.join(out, '|');
 }
 public static String rt() {
  Id probe = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('Probe_Type').getRecordTypeId();
  Id other = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('Other_Type').getRecordTypeId();
  List<Account> accts = new List<Account>{new Account(Name='RT1', AccountNumber='CritRT', RecordTypeId=probe), new Account(Name='RT2', AccountNumber='CritRT', RecordTypeId=other), new Account(Name='RT3', AccountNumber='CritRT')};
  insert accts;
  List<String> out = new List<String>();
  for (Account a : accts) {
   a.Name = a.Name + 'u';
   Database.SaveResult r = Database.update(a, false);
   if (!r.isSuccess()) { out.add(res(r)); continue; }
   Account got = [SELECT Site, Description, RecordType.DeveloperName FROM Account WHERE Id = :a.Id];
   out.add(got.Site + ',' + got.Description + ',' + got.RecordType.DeveloperName);
  }
  return String.join(out, '|');
 }
 public static String rv() {
  Account acme = new Account(Name='Acme RV'); insert acme;
  insert new Account(Name='Decoy RV');
  Contact bossA = new Contact(LastName='Boss', AccountId=acme.Id); Contact bossB = new Contact(LastName='Boss'); insert new List<Contact>{bossA, bossB};
  List<String> out = new List<String>();
  for (Contact c : new List<Contact>{new Contact(LastName='CritRV', ReportsToId=bossA.Id), new Contact(LastName='CritRV', ReportsToId=bossB.Id), new Contact(LastName='CritRV')}) {
   Database.SaveResult r = Database.insert(c, false);
   if (!r.isSuccess()) { out.add(res(r)); continue; }
   Contact got = [SELECT Description, Department FROM Contact WHERE Id = :c.Id];
   out.add(got.Description + ',' + got.Department);
  }
  return String.join(out, '|');
 }
 public static String mv() {
  List<String> out = new List<String>();
  for (String t : new List<String>{'Alpha Item', 'beta item', 'Missing', null}) {
   Contact c = new Contact(LastName='CritMV', Title=t);
   Database.SaveResult r = Database.insert(c, false);
   if (!r.isSuccess()) { out.add(res(r)); continue; }
   out.add(String.valueOf([SELECT Description FROM Contact WHERE Id = :c.Id].Description));
  }
  return String.join(out, '|');
 }
 public static String pb(Boolean customBook) {
  if (customBook) insert new Pricebook2(Name='Standard Price Book', IsActive=true);
  Product2 p = new Product2(Name='CritPB', IsActive=true);
  Database.SaveResult r = Database.insert(p, false);
  if (!r.isSuccess()) return res(r);
  List<PricebookEntry> entries = [SELECT Pricebook2.IsStandard, UnitPrice FROM PricebookEntry WHERE Product2Id = :p.Id];
  return 'ok,entries=' + entries.size() + (entries.isEmpty() ? '' : ',std=' + entries[0].Pricebook2.IsStandard);
 }
 public static String em(String marker) {
  Contact c = new Contact(LastName=marker, Title='T1', Email='probe-to@example.com');
  Database.SaveResult r = Database.insert(c, false);
  String head = res(r);
  if (r.isSuccess()) head += ',' + [SELECT Description FROM Contact WHERE Id = :c.Id].Description;
  return head + ',emails=' + Limits.getEmailInvocations();
 }
}
