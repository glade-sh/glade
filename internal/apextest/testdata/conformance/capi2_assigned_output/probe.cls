@IsTest private class Probe { @IsTest static void probe() {
  ConnectApi.CurrencyRecordField a=new ConnectApi.CurrencyRecordField(); a.type='wrong';
  ConnectApi.ContentHubRepository b=new ConnectApi.ContentHubRepository(); b.type='wrong';
  System.assert(false,'ROW|'+JSON.serialize(a)+'|field='+a.type+'|'+JSON.serialize(b)+'|field='+b.type);
 } }