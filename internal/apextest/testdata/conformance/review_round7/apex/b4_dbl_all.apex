QmP__c p = new QmP__c(Name='p'); insert p; QmC__c c = new QmC__c(Name='n', P__c=p.Id, Amt__c=1); insert c;
String res;
try { c.Amt__c=2; List<Database.SaveResult> r = Database.update(new List<QmC__c>{c}); res = r[0].isSuccess() ? 'ok' : String.valueOf(r[0].getErrors()[0].getStatusCode()); } catch (Exception e) { res = e.getTypeName(); }
System.debug('OBS|b4_dbl_all|' + res);