@IsTest private class NullProbe67_00 {
 @IsTest static void m00(){
  Account a=new Account(Name='Synthetic Account');insert a;
  Database.SaveResult missing=Database.insert(new Order(EffectiveDate=Date.today(),Status='Draft'),false);
  Database.SaveResult control=Database.insert(new Order(AccountId=a.Id,EffectiveDate=Date.today(),Status='Draft'),false);
  System.assert(false,'CAPTURE|Order|'+JSON.serialize(new Map<String,Object>{'AccountId'=>new Map<String,Object>{'nullSuccess'=>missing.isSuccess(),'nullErrors'=>missing.getErrors(),'controlSuccess'=>control.isSuccess(),'controlErrors'=>control.getErrors()}}));
 }
 @IsTest static void m01(){
  Product2 p=new Product2(Name='Synthetic Product',IsActive=true);insert p;Id pb=Test.getStandardPricebookId();
  PricebookEntry e=new PricebookEntry(Pricebook2Id=pb,Product2Id=p.Id,UnitPrice=1,IsActive=true);insert e;
  Opportunity o=new Opportunity(Name='Synthetic Opportunity',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=pb);insert o;
  Database.SaveResult missing=Database.insert(new OpportunityLineItem(OpportunityId=o.Id,Quantity=1,UnitPrice=1),false);
  Database.SaveResult control=Database.insert(new OpportunityLineItem(OpportunityId=o.Id,PricebookEntryId=e.Id,Quantity=1,UnitPrice=1),false);
  System.assert(false,'CAPTURE|OpportunityLineItem|'+JSON.serialize(new Map<String,Object>{'PricebookEntryId'=>new Map<String,Object>{'nullSuccess'=>missing.isSuccess(),'nullErrors'=>missing.getErrors(),'controlSuccess'=>control.isSuccess(),'controlErrors'=>control.getErrors()}}));
 }
}