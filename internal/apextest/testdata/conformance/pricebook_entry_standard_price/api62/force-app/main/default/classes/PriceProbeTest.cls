@IsTest private class PriceProbeTest {
 @IsTest static void probe() {
  Product2 p = new Product2(Name='Probe product', IsActive=true); insert p;
  Pricebook2 b = new Pricebook2(Name='Standard Price Book', IsActive=true); insert b;
  String observed;
  try { insert new PricebookEntry(Pricebook2Id=b.Id, Product2Id=p.Id, UnitPrice=5, IsActive=true); observed='ok'; }
  catch (DmlException e) { observed=String.valueOf(e.getDmlStatusCode(0))+':'+e.getDmlMessage(0); }
  System.assert(false, 'OBS='+observed);
 }
}
