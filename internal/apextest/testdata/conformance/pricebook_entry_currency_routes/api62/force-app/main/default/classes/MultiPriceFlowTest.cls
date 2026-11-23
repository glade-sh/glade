@IsTest private class MultiPriceFlowTest {
 @IsTest static void probe() {
  insert new Pricebook2(Name='Standard Price Book',IsActive=true);
  Database.SaveResult r=Database.insert(new Product2(Name='CritPB',IsActive=true),false);
  System.assert(false,'OBS='+(r.isSuccess() ? 'ok' : String.valueOf(r.getErrors()[0].getStatusCode())));
 }
}
