RcP__c p = new RcP__c(Name='rp'); insert p;
String res;
try { RcA__c x = new RcA__c(Name='n', P__c=p.Id, Amt__c=1); insert x; x.Amt__c=2; update as system x; res = 'ok'; } catch (Exception e) { res = e.getTypeName() + '|' + e.getMessage().left(70); }
System.debug('OBS|g_sys_b|' + res);