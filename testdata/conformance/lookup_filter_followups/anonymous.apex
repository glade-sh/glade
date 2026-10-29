Map<String, Id> rts = new Map<String, Id>();
for (RecordType r : [SELECT Id, DeveloperName FROM RecordType WHERE SobjectType = 'Account']) rts.put(r.DeveloperName, r.Id);
List<String> rows = new List<String>();
String outcome(Database.SaveResult r) { return r.isSuccess() ? 'OK' : String.valueOf(r.getErrors()[0].getStatusCode()); }
Map<String, Account> accounts = new Map<String, Account>();
for (String name : new List<String>{'Birch_East','Cedar_West','Plain'}) {
 Account a = new Account(Name = name, RecordTypeId = rts.get(name)); insert a; accounts.put(name, a);
}
for (String field : new List<String>{'Label__c','QuotedLabel__c','Name__c','QuotedName__c','OtherLabel__c','ListLabel__c'}) {
 if (!Schema.SObjectType.CommaProbe__c.fields.getMap().containsKey(field)) continue;
 for (String name : new List<String>{'Birch_East','Cedar_West','Plain','null'}) {
  CommaProbe__c probe = new CommaProbe__c(Name = 'probe');
  if (name != 'null') probe.put(field,accounts.get(name).Id);
  rows.add('comma.'+field+'|'+name+'|'+outcome(Database.insert(probe,false)));
 }
}
for (Decimal amount : new List<Decimal>{1,234,1234,12.5,67.25,12.56725,999}) {
 Holder__c h = new Holder__c(Name='number',Amount__c=amount); insert h;
 for (String field : new List<String>{'Number__c','NumberPair__c','NumberQuoted__c'}) {
  if (!Schema.SObjectType.LiteralProbe__c.fields.getMap().containsKey(field)) continue;
  LiteralProbe__c probe = new LiteralProbe__c(Name='number');probe.put(field,h.Id);
  rows.add('number.'+field+'|'+String.valueOf(amount)+'|'+outcome(Database.insert(probe,false)));
 }
}
for (Integer day : new List<Integer>{25,26,27,28}) {
 Holder__c h = new Holder__c(Name='date',Day__c=Date.newInstance(2026,10,day)); insert h;
 for (String field : new List<String>{'Date__c','DateText__c','DateOne__c','DateLocale__c'}) {
  if (!Schema.SObjectType.LiteralProbe__c.fields.getMap().containsKey(field)) continue;
  LiteralProbe__c probe = new LiteralProbe__c(Name='date');probe.put(field,h.Id);
  rows.add('date.'+field+'|'+String.valueOf(day)+'|'+outcome(Database.insert(probe,false)));
 }
}
for (String name : new List<String>{'Named_Person','Named_Individual','Care'}) {
 Account a = new Account(LastName='native',RecordTypeId=rts.get(name));
 Database.SaveResult saved=Database.insert(a,false);
 if(saved.isSuccess()) {
  Account actual = [SELECT RecordTypeId,IsPersonAccount FROM Account WHERE Id=:a.Id];
  rows.add('explicit|'+name+'|'+String.valueOf(actual.RecordTypeId==rts.get(name))+'|'+String.valueOf(actual.IsPersonAccount));
 } else { rows.add('explicit|'+name+'|'+outcome(saved)); }
 rows.add('missingLastName|'+name+'|'+outcome(Database.insert(new Account(RecordTypeId=rts.get(name)),false)));
}
rows.sort();
for(String row : rows) System.debug(LoggingLevel.ERROR,'ROW|'+row);
