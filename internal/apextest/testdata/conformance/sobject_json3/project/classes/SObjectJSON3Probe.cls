public class SObjectJSON3Probe {
    private static Map<String,String> ids;
    private static String normalize(String text) {
        for (String id : ids.keySet()) text=text.replace(id,ids.get(id));
        return text.replace(UserInfo.getName(),'<user-name>');
    }
    public static Map<String,Map<String,String>> observe() {
        Savepoint sp=Database.setSavepoint();
        Json_Number__c hi=new Json_Number__c(Name='High',Value__c=12345678901234.1234);
        Json_Number__c zero=new Json_Number__c(Name='Zero',Value__c=0);
        Json_Number__c neg=new Json_Number__c(Name='Negative',Value__c=-12.34);
        insert new List<Json_Number__c>{hi,zero,neg};
        Opportunity opp=new Opportunity(Name='Synthetic',StageName='Prospecting',CloseDate=Date.newInstance(2030,1,2),TotalOpportunityQuantity=12.5); insert opp;
        Opportunity oppZero=new Opportunity(Name='Zero',StageName='Prospecting',CloseDate=Date.newInstance(2001,12,31),TotalOpportunityQuantity=0,Amount=0);
        Opportunity oppNeg=new Opportunity(Name='Negative',StageName='Prospecting',CloseDate=Date.newInstance(2028,6,15),TotalOpportunityQuantity=-2.5,Amount=-12.34);
        insert new List<Opportunity>{oppZero,oppNeg};
        Account a=new Account(Name='Synthetic',Industry='Technology',AnnualRevenue=1234.5); insert a;
        Account empty=new Account(Name='Empty'); insert empty;
        Task task=new Task(Subject='Synthetic'); insert task;
        ids=new Map<String,String>{String.valueOf(hi.Id)=>'<high>',String.valueOf(zero.Id)=>'<zero>',String.valueOf(neg.Id)=>'<negative>',String.valueOf(opp.Id)=>'<opportunity>',String.valueOf(a.Id)=>'<account>',String.valueOf(empty.Id)=>'<empty>',String.valueOf(task.Id)=>'<task>',String.valueOf(UserInfo.getUserId())=>'<user>'};
        ids.put(String.valueOf(oppZero.Id),'<opp-zero>'); ids.put(String.valueOf(oppNeg.Id),'<opp-negative>');
        Id hiId=hi.Id,zeroId=zero.Id,negId=neg.Id,oppId=opp.Id,aId=a.Id,emptyId=empty.Id,taskId=task.Id;
        Id oppZeroId=oppZero.Id,oppNegId=oppNeg.Id;
        Map<String,String> queries=new Map<String,String>{
          'numberHigh'=>'SELECT Value__c FROM Json_Number__c WHERE Id=:hiId',
          'numberZero'=>'SELECT Value__c FROM Json_Number__c WHERE Id=:zeroId',
          'numberNegative'=>'SELECT Value__c FROM Json_Number__c WHERE Id=:negId',
          'standardDouble'=>'SELECT TotalOpportunityQuantity FROM Opportunity WHERE Id=:oppId',
          'standardZero'=>'SELECT TotalOpportunityQuantity FROM Opportunity WHERE Id=:oppZeroId',
          'standardNegative'=>'SELECT TotalOpportunityQuantity FROM Opportunity WHERE Id=:oppNegId',
          'formatNumber'=>'SELECT FORMAT(AnnualRevenue) FROM Account WHERE Id=:aId',
          'formatAlias'=>'SELECT FORMAT(AnnualRevenue) formatted FROM Account WHERE Id=:aId',
          'formatDate'=>'SELECT FORMAT(CloseDate) formatted FROM Opportunity WHERE Id=:oppId',
          'formatDateEndYear'=>'SELECT FORMAT(CloseDate) formatted FROM Opportunity WHERE Id=:oppZeroId',
          'formatDateMidYear'=>'SELECT FORMAT(CloseDate) formatted FROM Opportunity WHERE Id=:oppNegId',
          'formatZeroCurrency'=>'SELECT FORMAT(Amount) formatted FROM Opportunity WHERE Id=:oppZeroId',
          'formatNegativeCurrency'=>'SELECT FORMAT(Amount) formatted FROM Opportunity WHERE Id=:oppNegId',
          'formatNull'=>'SELECT FORMAT(AnnualRevenue) formatted FROM Account WHERE Id=:emptyId',
          'toLabel'=>'SELECT toLabel(Industry) FROM Account WHERE Id=:aId',
          'toLabelAlias'=>'SELECT toLabel(Industry) label FROM Account WHERE Id=:aId',
          'toLabelNull'=>'SELECT toLabel(Industry) label FROM Account WHERE Id=:emptyId',
          'taskOwner'=>'SELECT TYPEOF Owner WHEN User THEN Name ELSE Name END FROM Task WHERE Id=:taskId',
          'unknownField'=>'SELECT Unknown_Field__c FROM Account WHERE Id=:aId',
          'invalidOwnerType'=>'SELECT TYPEOF Owner WHEN Account THEN Name END FROM Task WHERE Id=:taskId',
          'emptyBoundary'=>'SELECT Value__c FROM Json_Number__c WHERE Name=\'Missing synthetic\''
        };
        Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
        for(String shape:queries.keySet()) {
          try {
            List<SObject> found=Database.query(queries.get(shape));
            Object value=found.isEmpty()?null:found[0];
            rows.put(shape,new Map<String,String>{'serialize'=>normalize(JSON.serialize(value)),'suppress'=>normalize(JSON.serialize(value,true))});
          } catch(Exception e) { rows.put(shape,new Map<String,String>{'query'=>e.getTypeName()+': '+e.getMessage()}); }
        }
        Database.rollback(sp); return rows;
    }
    public static String observeOwner() {
      Task t=new Task(Subject='Synthetic owner'); insert t;
      Task found=[SELECT TYPEOF Owner WHEN User THEN Name ELSE Name END FROM Task WHERE Id=:t.Id];
      ids=new Map<String,String>{String.valueOf(t.Id)=>'<task>',String.valueOf(UserInfo.getUserId())=>'<user>'};
      return normalize(JSON.serialize(found));
    }
    public static void emit() {
      Map<String,Map<String,String>> rows=observe();
      for(String shape:rows.keySet()) System.debug('SOBJECT_JSON3|'+shape+'|'+JSON.serialize(rows.get(shape)));
    }
}
