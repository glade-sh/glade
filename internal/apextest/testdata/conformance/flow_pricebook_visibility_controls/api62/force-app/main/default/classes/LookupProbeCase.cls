public class LookupProbeCase {
 public static String metadata() {
  List<Contact> cs = new List<Contact>{new Contact(LastName='MetaCheck', Title='Alpha'),new Contact(LastName='MetaCheck', Title='Missing'),new Contact(LastName='MetaCheck')}; insert cs;
  Map<Id, Contact> got = new Map<Id, Contact>([SELECT Description FROM Contact WHERE Id IN :cs]);
  return got.get(cs[0].Id).Description+'|'+got.get(cs[1].Id).Description+'|'+got.get(cs[2].Id).Description;
 }
 public static String book() {
  Id pricebookId = Test.isRunningTest() ? Test.getStandardPricebookId() : [SELECT Id FROM Pricebook2 WHERE IsStandard = true LIMIT 1].Id;
  Opportunity o = new Opportunity(Name='BookCheck',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=pricebookId); insert o;
  return [SELECT Description FROM Opportunity WHERE Id=:o.Id].Description;
 }
 public static String customBook() {
  Pricebook2 book = new Pricebook2(Name='Secondary', IsActive=true); insert book;
  Opportunity o = new Opportunity(Name='BookCheck',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=book.Id); insert o;
  return [SELECT Description FROM Opportunity WHERE Id=:o.Id].Description;
 }
}
