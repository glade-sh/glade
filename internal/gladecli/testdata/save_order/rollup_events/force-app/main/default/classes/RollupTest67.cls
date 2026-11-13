@IsTest private class RollupTest67 { @IsTest static void combined() {
Savepoint sp = Database.setSavepoint();
Probe_Parent__c ok = new Probe_Parent__c(Name='ok'); Probe_Parent__c bad = new Probe_Parent__c(Name='reject'); insert new List<Probe_Parent__c>{ok,bad};
RollupTrace.events = '';
Database.SaveResult[] rs = Database.insert(new List<Probe_Child__c>{new Probe_Child__c(Name='okchild',Parent__c=ok.Id),new Probe_Child__c(Name='badchild',Parent__c=bad.Id)},false);
List<String> parts = new List<String>(); for (Database.SaveResult r : rs) parts.add(r.isSuccess() ? 'ok' : String.valueOf(r.getErrors()[0].getStatusCode()));
Set<Id> ids = new Set<Id>{ok.Id,bad.Id};
List<Probe_Parent__c> parents = [SELECT Name, Child_Count__c FROM Probe_Parent__c WHERE Id IN :ids ORDER BY Name];
String observed = 'test:' + Test.isRunningTest() + '|' + String.join(parts, ',') + '|' + RollupTrace.events + '|counts:' + parents[0].Child_Count__c + ',' + parents[1].Child_Count__c + '|children:' + [SELECT COUNT() FROM Probe_Child__c WHERE Parent__c IN :ids]; Database.rollback(sp);
System.assertEquals('EXPECTED',observed); } }
