@IsTest private class MultiPriceProbeTest {
 @IsTest static void probe() {
  Product2 p=new Product2(Name='Product',IsActive=true); insert p;
  Pricebook2 b=new Pricebook2(Name='Custom',IsActive=true); insert b;
  String before;
  try { insert new PricebookEntry(Pricebook2Id=b.Id,Product2Id=p.Id,UnitPrice=5,IsActive=true); before='ok'; } catch(DmlException e) { before=String.valueOf(e.getDmlStatusCode(0)); }
  insert new PricebookEntry(Pricebook2Id=Test.getStandardPricebookId(),Product2Id=p.Id,UnitPrice=5,IsActive=true);
  String after;
  try { insert new PricebookEntry(Pricebook2Id=b.Id,Product2Id=p.Id,UnitPrice=4,IsActive=true); after='ok'; } catch(DmlException e) { after=String.valueOf(e.getDmlStatusCode(0)); }
  System.assert(false,'OBS=before='+before+'|after='+after);
 }
}
