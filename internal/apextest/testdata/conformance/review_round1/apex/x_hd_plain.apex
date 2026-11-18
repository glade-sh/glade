HdA__c sa = new HdA__c(Name='a'); insert sa; insert new HdB__c(Name='b');
HdCtl.reset('plain');
HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name = 'qplain';
String res;
try { update a; res = 'ok'; } catch (Exception e) { res = e.getTypeName() + '|max=' + e.getMessage().contains('maximum trigger depth exceeded'); }
System.debug('OBS|hd_plain|' + res + '|d=' + HdCtl.depth + '|log=' + HdCtl.log + '|' + HdCtl.lim + '|after=dml' + Limits.getDmlStatements() + ',q' + Limits.getQueries());