QmP__c p = new QmP__c(Name='p'); insert p; QmC__c c = new QmC__c(Name='n', P__c=p.Id, Amt__c=1); insert c;
String res;
try { c.Amt__c=2; Database.SaveResult r = Database.update(c, false); res = r.isSuccess() ? 'ok' : String.valueOf(r.getErrors()[0].getStatusCode()); } catch (Exception e) { res = e.getTypeName(); }
System.debug('OBS|q_dbf|' + res);