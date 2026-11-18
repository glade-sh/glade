RcP__c p = new RcP__c(Name='rp'); insert p;
String res;
try { RcB__c x = new RcB__c(Name='n', P__c=p.Id); insert x; x.Name='n2'; update x; res = 'ok'; } catch (Exception e) { res = e.getTypeName() + '|' + e.getMessage().left(70); }
System.debug('OBS|g_noext_a|' + res);