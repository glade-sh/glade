HdA__c sa = new HdA__c(Name='a'); insert sa; insert new HdB__c(Name='b');
HdCtl.reset('catch');
HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name = 'qcatch';
String res;
try { update a; res = 'ok'; } catch (Exception e) { res = e.getTypeName() + '|max=' + e.getMessage().contains('maximum trigger depth exceeded'); }
System.debug('OBS|hd_catch|' + res + '|d=' + HdCtl.depth + '|log=' + HdCtl.log + '|' + HdCtl.lim + '|after=dml' + Limits.getDmlStatements() + ',q' + Limits.getQueries());