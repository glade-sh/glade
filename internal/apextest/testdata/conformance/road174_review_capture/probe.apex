Account acc = new Account(Name='Access probe'); insert acc;
Lead lead = new Lead(LastName='Access probe', Company='Example'); insert lead;
Campaign campaign = new Campaign(Name='Access probe'); insert campaign;
Product2 product = new Product2(Name='Access probe', IsActive=true); insert product;
Asset asset = new Asset(Name='Access probe', AccountId=acc.Id, Product2Id=product.Id); insert asset;
Pricebook2 book = new Pricebook2(Name='Access probe', IsActive=true); insert book;
Contract contract = new Contract(AccountId=acc.Id, StartDate=Date.today(), ContractTerm=12); insert contract;
Order ord = new Order(AccountId=acc.Id, EffectiveDate=Date.today(), Status='Draft'); insert ord;
Pricebook2 standardBook = [SELECT Id, IsActive FROM Pricebook2 WHERE IsStandard=true LIMIT 1];
if (!standardBook.IsActive) { standardBook.IsActive=true; update standardBook; }
PricebookEntry entry = new PricebookEntry(Pricebook2Id=standardBook.Id, Product2Id=product.Id, UnitPrice=10, IsActive=true); insert entry;
Opportunity opp = new Opportunity(Name='Access probe', StageName='Prospecting', CloseDate=Date.today(), AccountId=acc.Id, CampaignId=campaign.Id, Pricebook2Id=standardBook.Id); insert opp;
OpportunityLineItem line = new OpportunityLineItem(OpportunityId=opp.Id, PricebookEntryId=entry.Id, Quantity=1, UnitPrice=10); insert line;
for (String objectName : new List<String>{'Lead','Campaign','Contract','Asset','Product2','Pricebook2','Order','OpportunityLineItem'}) {
 try {
  List<SObject> rows = Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM '+objectName+' LIMIT 1');
  SObject a=rows[0].getSObject('UserRecordAccess');
  List<Object> vals=new List<Object>();
  for (String f : new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'}) vals.add(a.get(f));
  System.debug('ROAD174R|projection|'+objectName+'|ok|'+JSON.serialize(vals));
 } catch (Exception e) { System.debug('ROAD174R|projection|'+objectName+'|reject|'+e.getTypeName()+':'+e.getMessage()); }
}
for (String q : new List<String>{'SELECT Campaign.UserRecordAccess.HasReadAccess,Campaign.UserRecordAccess.HasEditAccess FROM Opportunity LIMIT 1','SELECT Product2.UserRecordAccess.HasReadAccess,Product2.UserRecordAccess.HasEditAccess FROM Asset LIMIT 1','SELECT PricebookEntry.Product2.UserRecordAccess.HasReadAccess FROM OpportunityLineItem LIMIT 1','SELECT UserRecordAccess.UserId FROM Lead LIMIT 1','SELECT UserRecordAccess.HasReadAccess FROM User LIMIT 1'}) {
 try { List<SObject> rows=Database.query(q); System.debug('ROAD174R|query|'+q+'|ok|'+JSON.serialize(rows)); }
 catch (Exception e) { System.debug('ROAD174R|query|'+q+'|reject|'+e.getTypeName()+':'+e.getMessage()); }
}
