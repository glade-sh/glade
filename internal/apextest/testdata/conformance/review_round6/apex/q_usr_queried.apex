QmP__c p = new QmP__c(Name='p'); insert p; QmC__c c = new QmC__c(Name='n', P__c=p.Id, Amt__c=1); insert c;
String res;
try { QmC__c x = [SELECT Id, Name, Amt__c, P__c FROM QmC__c WHERE Id = :c.Id]; x.Amt__c=3; update as user x; res='ok'; } catch (Exception e) { res = e.getTypeName(); }
System.debug('OBS|q_usr_queried|' + res);