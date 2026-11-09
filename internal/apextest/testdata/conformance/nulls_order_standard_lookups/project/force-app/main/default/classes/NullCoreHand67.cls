@IsTest(SeeAllData=false) private class NullCoreHand67 {
 static String run(String obj,String field,List<SObject> ps,List<SObject> kids){
  Map<String,Object> o=new Map<String,Object>();
  try{
   insert as system ps; List<Id> pid=new List<Id>(); for(SObject p:ps) pid.add(p.Id); pid.sort();
   kids[1].put(field,pid[0]); kids[2].put(field,pid[1]);
   insert as system kids;
   Map<Id,String> lab=new Map<Id,String>{kids[0].Id=>'N',kids[1].Id=>'L',kids[2].Id=>'H'}; Set<Id> ids=lab.keySet();
   for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}){
    List<String> got=new List<String>();
    for(SObject r:Database.query('SELECT Id FROM '+obj+' WHERE Id IN :ids ORDER BY '+field+' '+tail)) got.add(lab.get(r.Id));
    o.put(tail,String.join(got,''));
   }
  }catch(Exception e){ o.put('skip','EXC '+e.getTypeName()+': '+e.getMessage().left(200)); }
  return JSON.serialize(o);
 }
 @IsTest static void m00(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Account(Name='kid',ParentId=p),new Account(Name='kid',ParentId=p),new Account(Name='kid',ParentId=p)};
  System.assert(false,'CAPTURE|Account|ParentId|'+run('Account','ParentId',ps,kids));
 }
 @IsTest static void m01(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Contact(LastName='kid',AccountId=p),new Contact(LastName='kid',AccountId=p),new Contact(LastName='kid',AccountId=p)};
  System.assert(false,'CAPTURE|Contact|AccountId|'+run('Contact','AccountId',ps,kids));
 }
 @IsTest static void m02(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Contact(LastName='kid',ReportsToId=p),new Contact(LastName='kid',ReportsToId=p),new Contact(LastName='kid',ReportsToId=p)};
  System.assert(false,'CAPTURE|Contact|ReportsToId|'+run('Contact','ReportsToId',ps,kids));
 }
 @IsTest static void m03(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Individual(LastName='np i'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Contact(LastName='kid',IndividualId=p),new Contact(LastName='kid',IndividualId=p),new Contact(LastName='kid',IndividualId=p)};
  System.assert(false,'CAPTURE|Contact|IndividualId|'+run('Contact','IndividualId',ps,kids));
 }
 @IsTest static void m04(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Case(Subject='kid',AccountId=p),new Case(Subject='kid',AccountId=p),new Case(Subject='kid',AccountId=p)};
  System.assert(false,'CAPTURE|Case|AccountId|'+run('Case','AccountId',ps,kids));
 }
 @IsTest static void m05(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Case(Subject='kid',ContactId=p),new Case(Subject='kid',ContactId=p),new Case(Subject='kid',ContactId=p)};
  System.assert(false,'CAPTURE|Case|ContactId|'+run('Case','ContactId',ps,kids));
 }
 @IsTest static void m06(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Case(Subject='np p'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Case(Subject='kid',ParentId=p),new Case(Subject='kid',ParentId=p),new Case(Subject='kid',ParentId=p)};
  System.assert(false,'CAPTURE|Case|ParentId|'+run('Case','ParentId',ps,kids));
 }
 @IsTest static void m07(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Asset(Name='np a'+i,AccountId=acct.Id));
  Id p=null; List<SObject> kids=new List<SObject>{new Case(Subject='kid',AssetId=p),new Case(Subject='kid',AssetId=p),new Case(Subject='kid',AssetId=p)};
  System.assert(false,'CAPTURE|Case|AssetId|'+run('Case','AssetId',ps,kids));
 }
 @IsTest static void m08(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Asset(Name='kid',AccountId=acct.Id,ContactId=p),new Asset(Name='kid',AccountId=acct.Id,ContactId=p),new Asset(Name='kid',AccountId=acct.Id,ContactId=p)};
  System.assert(false,'CAPTURE|Asset|ContactId|'+run('Asset','ContactId',ps,kids));
 }
 @IsTest static void m09(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Asset(Name='kid',ContactId=con.Id,AccountId=p),new Asset(Name='kid',ContactId=con.Id,AccountId=p),new Asset(Name='kid',ContactId=con.Id,AccountId=p)};
  System.assert(false,'CAPTURE|Asset|AccountId|'+run('Asset','AccountId',ps,kids));
 }
 @IsTest static void m10(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Asset(Name='np a'+i,AccountId=acct.Id));
  Id p=null; List<SObject> kids=new List<SObject>{new Asset(Name='kid',AccountId=acct.Id,ParentId=p),new Asset(Name='kid',AccountId=acct.Id,ParentId=p),new Asset(Name='kid',AccountId=acct.Id,ParentId=p)};
  System.assert(false,'CAPTURE|Asset|ParentId|'+run('Asset','ParentId',ps,kids));
 }
 @IsTest static void m11(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Product2(Name='np pr'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Asset(Name='kid',AccountId=acct.Id,Product2Id=p),new Asset(Name='kid',AccountId=acct.Id,Product2Id=p),new Asset(Name='kid',AccountId=acct.Id,Product2Id=p)};
  System.assert(false,'CAPTURE|Asset|Product2Id|'+run('Asset','Product2Id',ps,kids));
 }
 @IsTest static void m12(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),AccountId=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),AccountId=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),AccountId=p)};
  System.assert(false,'CAPTURE|Opportunity|AccountId|'+run('Opportunity','AccountId',ps,kids));
 }
 @IsTest static void m13(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Campaign(Name='np cm'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),CampaignId=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),CampaignId=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),CampaignId=p)};
  System.assert(false,'CAPTURE|Opportunity|CampaignId|'+run('Opportunity','CampaignId',ps,kids));
 }
 @IsTest static void m14(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),ContactId=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),ContactId=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),ContactId=p)};
  System.assert(false,'CAPTURE|Opportunity|ContactId|'+run('Opportunity','ContactId',ps,kids));
 }
 @IsTest static void m15(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Pricebook2(Name='np pb'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=p),new Opportunity(Name='kid',StageName='Prospecting',CloseDate=Date.today(),Pricebook2Id=p)};
  System.assert(false,'CAPTURE|Opportunity|Pricebook2Id|'+run('Opportunity','Pricebook2Id',ps,kids));
 }
 @IsTest static void m16(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Campaign(Name='np cm'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Campaign(Name='kid',ParentId=p),new Campaign(Name='kid',ParentId=p),new Campaign(Name='kid',ParentId=p)};
  System.assert(false,'CAPTURE|Campaign|ParentId|'+run('Campaign','ParentId',ps,kids));
 }
 @IsTest static void m17(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Individual(LastName='np i'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Lead(LastName='kid',Company='co',IndividualId=p),new Lead(LastName='kid',Company='co',IndividualId=p),new Lead(LastName='kid',Company='co',IndividualId=p)};
  System.assert(false,'CAPTURE|Lead|IndividualId|'+run('Lead','IndividualId',ps,kids));
 }
 @IsTest static void m18(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Pricebook2(Name='np pb'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Contract(AccountId=acct.Id,Pricebook2Id=p),new Contract(AccountId=acct.Id,Pricebook2Id=p),new Contract(AccountId=acct.Id,Pricebook2Id=p)};
  System.assert(false,'CAPTURE|Contract|Pricebook2Id|'+run('Contract','Pricebook2Id',ps,kids));
 }
 @IsTest static void m19(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Contract(AccountId=acct.Id,CustomerSignedId=p),new Contract(AccountId=acct.Id,CustomerSignedId=p),new Contract(AccountId=acct.Id,CustomerSignedId=p)};
  System.assert(false,'CAPTURE|Contract|CustomerSignedId|'+run('Contract','CustomerSignedId',ps,kids));
 }
 @IsTest static void m20(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i,AccountId=acct.Id));
  Id p=null; List<SObject> kids=new List<SObject>{new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',ShipToContactId=p),new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',ShipToContactId=p),new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',ShipToContactId=p)};
  System.assert(false,'CAPTURE|Order|ShipToContactId|'+run('Order','ShipToContactId',ps,kids));
 }
 @IsTest static void m21(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Pricebook2(Name='np pb'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',Pricebook2Id=p),new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',Pricebook2Id=p),new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',Pricebook2Id=p)};
  System.assert(false,'CAPTURE|Order|Pricebook2Id|'+run('Order','Pricebook2Id',ps,kids));
 }
 @IsTest static void m22(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contract(AccountId=acct.Id));
  Id p=null; List<SObject> kids=new List<SObject>{new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',ContractId=p),new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',ContractId=p),new Order(AccountId=acct.Id,EffectiveDate=Date.today(),Status='Draft',ContractId=p)};
  System.assert(false,'CAPTURE|Order|ContractId|'+run('Order','ContractId',ps,kids));
 }
 @IsTest static void m23(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Task(Subject='kid',WhatId=p),new Task(Subject='kid',WhatId=p),new Task(Subject='kid',WhatId=p)};
  System.assert(false,'CAPTURE|Task|WhatId|'+run('Task','WhatId',ps,kids));
 }
 @IsTest static void m24(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Task(Subject='kid',WhoId=p),new Task(Subject='kid',WhoId=p),new Task(Subject='kid',WhoId=p)};
  System.assert(false,'CAPTURE|Task|WhoId|'+run('Task','WhoId',ps,kids));
 }
 @IsTest static void m25(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Event(Subject='kid',DurationInMinutes=30,ActivityDateTime=System.now(),WhatId=p),new Event(Subject='kid',DurationInMinutes=30,ActivityDateTime=System.now(),WhatId=p),new Event(Subject='kid',DurationInMinutes=30,ActivityDateTime=System.now(),WhatId=p)};
  System.assert(false,'CAPTURE|Event|WhatId|'+run('Event','WhatId',ps,kids));
 }
 @IsTest static void m26(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Event(Subject='kid',DurationInMinutes=30,ActivityDateTime=System.now(),WhoId=p),new Event(Subject='kid',DurationInMinutes=30,ActivityDateTime=System.now(),WhoId=p),new Event(Subject='kid',DurationInMinutes=30,ActivityDateTime=System.now(),WhoId=p)};
  System.assert(false,'CAPTURE|Event|WhoId|'+run('Event','WhoId',ps,kids));
 }
 @IsTest static void m27(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new WorkOrder(Subject='kid',AccountId=p),new WorkOrder(Subject='kid',AccountId=p),new WorkOrder(Subject='kid',AccountId=p)};
  System.assert(false,'CAPTURE|WorkOrder|AccountId|'+run('WorkOrder','AccountId',ps,kids));
 }
 @IsTest static void m28(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Contact(LastName='np c'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new WorkOrder(Subject='kid',ContactId=p),new WorkOrder(Subject='kid',ContactId=p),new WorkOrder(Subject='kid',ContactId=p)};
  System.assert(false,'CAPTURE|WorkOrder|ContactId|'+run('WorkOrder','ContactId',ps,kids));
 }
 @IsTest static void m29(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new ServiceContract(Name='kid',AccountId=p),new ServiceContract(Name='kid',AccountId=p),new ServiceContract(Name='kid',AccountId=p)};
  System.assert(false,'CAPTURE|ServiceContract|AccountId|'+run('ServiceContract','AccountId',ps,kids));
 }
 @IsTest static void m30(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Schema.Location(Name='np l'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Schema.Location(Name='kid',ParentLocationId=p),new Schema.Location(Name='kid',ParentLocationId=p),new Schema.Location(Name='kid',ParentLocationId=p)};
  System.assert(false,'CAPTURE|Location|ParentLocationId|'+run('Location','ParentLocationId',ps,kids));
 }
 @IsTest static void m31(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Account(Name='np acct'+i));
  Id p=null; List<SObject> kids=new List<SObject>{new Expense(Title='kid',TransactionDate=Date.today(),AccountId=p),new Expense(Title='kid',TransactionDate=Date.today(),AccountId=p),new Expense(Title='kid',TransactionDate=Date.today(),AccountId=p)};
  System.assert(false,'CAPTURE|Expense|AccountId|'+run('Expense','AccountId',ps,kids));
 }
 @IsTest static void m32(){
  Account acct=new Account(Name='np base'); insert as system acct; Contact con=new Contact(LastName='np base'); insert as system con; Campaign camp=new Campaign(Name='np base'); insert as system camp;
  List<SObject> ps=new List<SObject>(); for(Integer i=0;i<2;i++) ps.add(new Lead(LastName='np l'+i,Company='co'));
  Id p=null; List<SObject> kids=new List<SObject>{new CampaignMember(CampaignId=camp.Id,ContactId=null,LeadId=p),new CampaignMember(CampaignId=camp.Id,ContactId=null,LeadId=p),new CampaignMember(CampaignId=camp.Id,ContactId=null,LeadId=p)};
  System.assert(false,'CAPTURE|CampaignMember|LeadId|'+run('CampaignMember','LeadId',ps,kids));
 }
}
