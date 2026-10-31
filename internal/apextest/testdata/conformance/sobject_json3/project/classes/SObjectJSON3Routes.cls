public class SObjectJSON3Routes {
  public static Map<String,Map<String,String>> observe() {
    Savepoint sp=Database.setSavepoint();
    Account a=new Account(Name='Synthetic',Industry='Technology',AnnualRevenue=1234.5); insert a;
    Account childAccount=new Account(Name='Synthetic child',ParentId=a.Id,Industry='Technology'); insert childAccount;
    Opportunity o=new Opportunity(Name='Synthetic',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.newInstance(2030,1,2),Amount=1234.5); insert o;
    Id aid=a.Id;
    Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
    try {
      Account s=[SELECT FORMAT(AnnualRevenue) formatted, toLabel(Industry) label FROM Account WHERE Id=:aid];
      rows.put('static',new Map<String,String>{'value'=>JSON.serialize(new List<Object>{s.get('formatted'),s.get('label')})});
    } catch(Exception e) {rows.put('static',new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()});}
    try {
      Database.QueryLocatorIterator it=Database.getQueryLocator('SELECT FORMAT(AnnualRevenue) formatted, toLabel(Industry) label FROM Account WHERE Id=:aid').iterator();
      SObject q=it.next();
      rows.put('locator',new Map<String,String>{'value'=>JSON.serialize(new List<Object>{q.get('formatted'),q.get('label')})});
    } catch(Exception e) {rows.put('locator',new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()});}
    try {
      Account p=[SELECT Id, (SELECT FORMAT(Amount) formatted, toLabel(StageName) label FROM Opportunities) FROM Account WHERE Id=:aid];
      SObject c=p.getSObjects('Opportunities')[0];
      rows.put('child',new Map<String,String>{'value'=>JSON.serialize(new List<Object>{c.get('formatted'),c.get('label')})});
    } catch(Exception e) {rows.put('child',new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()});}
    try {
      Account p=[SELECT Id, (SELECT toLabel(Industry) label FROM ChildAccounts) FROM Account WHERE Id=:aid];
      SObject c=p.getSObjects('ChildAccounts')[0];
      rows.put('childLabel',new Map<String,String>{'value'=>JSON.serialize(c.get('label'))});
    } catch(Exception e) {rows.put('childLabel',new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()});}
    try { Account s=[SELECT Name FROM Account WHERE Id=:aid]; rows.put('missing',new Map<String,String>{'value'=>String.valueOf(s.get('Industry'))}); }
    catch(Exception e) {rows.put('missing',new Map<String,String>{'value'=>e.getTypeName()});}
    Database.rollback(sp); return rows;
  }
  public static void emit() { Map<String,Map<String,String>> rows=observe(); for(String k:rows.keySet()) System.debug('SOBJECT_ROUTES|'+k+'|'+JSON.serialize(rows.get(k))); }
}
