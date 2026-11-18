RcP__c p = new RcP__c(Name='rp'); insert p;
String res;
try { RcA__c x = new RcA__c(Name='n', P__c=p.Id, Amt__c=1); insert x; x.Name='n2'; update x; res = 'ok'; } catch (Exception e) { res = e.getTypeName() + '|' + e.getMessage().left(70); }
System.debug('OBS|g_name_a|' + res);