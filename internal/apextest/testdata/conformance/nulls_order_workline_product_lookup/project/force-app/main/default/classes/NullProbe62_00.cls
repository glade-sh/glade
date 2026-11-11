@IsTest private class NullProbe62_00 {
 @IsTest static void m00(){
  List<Product2> products=new List<Product2>{new Product2(Name='Synthetic Alpha',IsActive=true),new Product2(Name='Synthetic Beta',IsActive=true)};insert products;
  Id pb=Test.getStandardPricebookId();List<PricebookEntry> entries=new List<PricebookEntry>{new PricebookEntry(Pricebook2Id=pb,Product2Id=products[0].Id,UnitPrice=1,IsActive=true),new PricebookEntry(Pricebook2Id=pb,Product2Id=products[1].Id,UnitPrice=1,IsActive=true)};insert entries;
  WorkOrder wo=new WorkOrder(Subject='Synthetic',Status='New',Pricebook2Id=pb);insert wo;
  List<WorkOrderLineItem> lines=new List<WorkOrderLineItem>{new WorkOrderLineItem(WorkOrderId=wo.Id,Subject='Null'),new WorkOrderLineItem(WorkOrderId=wo.Id,Subject='Alpha',PricebookEntryId=entries[0].Id,Quantity=1,UnitPrice=1),new WorkOrderLineItem(WorkOrderId=wo.Id,Subject='Beta',PricebookEntryId=entries[1].Id,Quantity=1,UnitPrice=1)};insert lines;
  Set<Id> ids=new Set<Id>{lines[0].Id,lines[1].Id,lines[2].Id};Map<Id,WorkOrderLineItem> saved=new Map<Id,WorkOrderLineItem>([SELECT Id,Product2Id FROM WorkOrderLineItem WHERE Id IN :ids]);
  if(saved.size()!=3 || saved.get(lines[0].Id).Product2Id!=null || saved.get(lines[1].Id).Product2Id!=products[0].Id || saved.get(lines[2].Id).Product2Id!=products[1].Id)System.assert(false,'invalid saved shape');
  Boolean firstLow=String.valueOf(products[0].Id).compareTo(String.valueOf(products[1].Id))<0;Map<Id,String> labels=new Map<Id,String>{lines[0].Id=>'N',lines[1].Id=>(firstLow?'L':'H'),lines[2].Id=>(firstLow?'H':'L')};Map<String,String> obs=new Map<String,String>();
  for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}){List<String> seen=new List<String>();for(SObject x:Database.query('SELECT Id FROM WorkOrderLineItem WHERE Id IN :ids ORDER BY Product2Id '+tail))seen.add(labels.get((Id)x.get('Id')));obs.put(tail,String.join(seen,''));}
  System.assert(false,'CAPTURE|WorkOrderLineItem|'+JSON.serialize(new Map<String,Object>{'Product2Id'=>obs}));
 }
}
