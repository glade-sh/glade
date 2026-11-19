@IsTest private class NullProbe67_00 {
 @IsTest static void m00(){
  WorkOrder wo=new WorkOrder(Subject='Synthetic',Status='New');insert wo;
  Database.SaveResult missing=Database.insert(new ServiceAppointment(Subject='Null'),false);
  Database.SaveResult control=Database.insert(new ServiceAppointment(Subject='Populated',ParentRecordId=wo.Id),false);
  System.assert(false,'CAPTURE|ServiceAppointment|'+JSON.serialize(new Map<String,Object>{'ParentRecordId'=>new Map<String,Object>{'nullSuccess'=>missing.isSuccess(),'nullErrors'=>missing.getErrors(),'controlSuccess'=>control.isSuccess(),'controlErrors'=>control.getErrors()}}));
 }
 @IsTest static void m01(){
  List<Product2> p=new List<Product2>{new Product2(Name='Synthetic Alpha',IsActive=true),new Product2(Name='Synthetic Beta',IsActive=true)};insert p;Id pb=Test.getStandardPricebookId();
  List<PricebookEntry> e=new List<PricebookEntry>{new PricebookEntry(Pricebook2Id=pb,Product2Id=p[0].Id,UnitPrice=1,IsActive=true),new PricebookEntry(Pricebook2Id=pb,Product2Id=p[1].Id,UnitPrice=1,IsActive=true)};insert e;
  Opportunity o=new Opportunity(Name='Synthetic Opportunity',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=pb);insert o;
  List<OpportunityLineItem> lines=new List<OpportunityLineItem>{new OpportunityLineItem(OpportunityId=o.Id,PricebookEntryId=e[0].Id,Quantity=1,UnitPrice=1),new OpportunityLineItem(OpportunityId=o.Id,PricebookEntryId=e[1].Id,Quantity=1,UnitPrice=1)};insert lines;
  List<OpportunityLineItem> saved=[SELECT Product2Id,PricebookEntryId FROM OpportunityLineItem WHERE Id IN :lines];Boolean matches=true;for(OpportunityLineItem x:saved)matches=matches && ((x.PricebookEntryId==e[0].Id && x.Product2Id==p[0].Id)||(x.PricebookEntryId==e[1].Id && x.Product2Id==p[1].Id));
  System.assert(false,'CAPTURE|OpportunityLineItem|'+JSON.serialize(new Map<String,Object>{'Product2Id'=>new Map<String,Object>{'omittedProductPopulatedFromEntry'=>matches,'savedCount'=>saved.size()}}));
 }
 @IsTest static void m02(){
  List<Product2> p=new List<Product2>{new Product2(Name='Synthetic Alpha',IsActive=true),new Product2(Name='Synthetic Beta',IsActive=true)};insert p;Id pb=Test.getStandardPricebookId();
  List<PricebookEntry> e=new List<PricebookEntry>{new PricebookEntry(Pricebook2Id=pb,Product2Id=p[0].Id,UnitPrice=1,IsActive=true),new PricebookEntry(Pricebook2Id=pb,Product2Id=p[1].Id,UnitPrice=1,IsActive=true)};insert e;
  Account a=new Account(Name='Synthetic');insert a;Order o=new Order(AccountId=a.Id,EffectiveDate=Date.today(),Status='Draft',Pricebook2Id=pb);insert o;
  List<OrderItem> lines=new List<OrderItem>{new OrderItem(OrderId=o.Id,PricebookEntryId=e[0].Id,Quantity=1,UnitPrice=1),new OrderItem(OrderId=o.Id,PricebookEntryId=e[1].Id,Quantity=1,UnitPrice=1)};insert lines;
  List<OrderItem> saved=[SELECT Product2Id,PricebookEntryId FROM OrderItem WHERE Id IN :lines];Boolean matches=true;for(OrderItem x:saved)matches=matches && ((x.PricebookEntryId==e[0].Id && x.Product2Id==p[0].Id)||(x.PricebookEntryId==e[1].Id && x.Product2Id==p[1].Id));
  System.assert(false,'CAPTURE|OrderItem|'+JSON.serialize(new Map<String,Object>{'Product2Id'=>new Map<String,Object>{'omittedProductPopulatedFromEntry'=>matches,'savedCount'=>saved.size()}}));
 }
}