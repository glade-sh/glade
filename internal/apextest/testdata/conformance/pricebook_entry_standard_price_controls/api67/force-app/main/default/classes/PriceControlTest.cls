@IsTest private class PriceControlTest {
 @IsTest static void probe() {
  Product2 p=new Product2(Name='First product',IsActive=true); insert p;
  Product2 q=new Product2(Name='Second product',IsActive=true); insert q;
  Pricebook2 b=new Pricebook2(Name='Custom book',IsActive=true); insert b;
  PricebookEntry std=new PricebookEntry(Pricebook2Id=Test.getStandardPricebookId(),Product2Id=p.Id,UnitPrice=5,IsActive=true); insert std;
  String a,boundary;
  try { insert new PricebookEntry(Pricebook2Id=b.Id,Product2Id=p.Id,UnitPrice=4,IsActive=true); a='ok'; } catch(DmlException e) { a=String.valueOf(e.getDmlStatusCode(0)); }
  try { insert new PricebookEntry(Pricebook2Id=b.Id,Product2Id=q.Id,UnitPrice=4,IsActive=true); boundary='ok'; } catch(DmlException e) { boundary=String.valueOf(e.getDmlStatusCode(0)); }
  System.assert(false,'OBS=standard=ok|same='+a+'|other='+boundary);
 }
}
