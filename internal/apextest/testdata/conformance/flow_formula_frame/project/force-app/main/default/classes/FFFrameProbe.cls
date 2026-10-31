public class FFFrameProbe {
 public static String observe() {
  Map<String,Object> out = new Map<String,Object>();
  for (String kind : new List<String>{'ASSIGN','DECIDE','CREATE','ERROR','CONTROL'}) {
   Account a = new Account(Name='FF'+kind+'_ONE', Site='RECORD_SITE');
   Database.SaveResult result = Database.insert(a, false);
   Map<String,Object> row = new Map<String,Object>{'success'=>result.isSuccess()};
   if (!result.isSuccess()) row.put('message', result.getErrors()[0].getMessage());
   else {
    a = [SELECT Description, AccountNumber, TickerSymbol, BillingCity FROM Account WHERE Id = :a.Id];
    row.put('description', a.Description); row.put('accountNumber', a.AccountNumber); row.put('ticker', a.TickerSymbol); row.put('city', a.BillingCity);
    if (kind == 'CREATE') { Contact c = [SELECT LastName, Description, Title, Department FROM Contact WHERE Title = 'FFCREATE_ONE' LIMIT 1]; row.put('contact', new Map<String,Object>{'lastName'=>c.LastName,'description'=>c.Description,'title'=>c.Title,'department'=>c.Department}); }
   }
   out.put(kind,row);
  }
  return JSON.serialize(out);
 }
}
